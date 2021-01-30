package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	wg "github.com/dxmaxwell/workgroup"
)

func main() {

	if isatty.IsTerminal(os.Stdout.Fd()) {
		w := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = os.Stdout
		})
		log.Logger = zerolog.New(w).With().Timestamp().Logger()
	}

	err := mainWithContext(context.Background())
	if err != nil {
		log.Err(err)
		if err, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(err.ExitCode())
		}
		os.Exit(1)
	}
}

func mainWithContext(ctx context.Context) error {

	log.Info().Msg("Makeshiftd starting")

	handler := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		fmt.Printf("Hello World!")
		res.Write([]byte("<html><body>Hello World</body></html>"))
	})

	err := listenAndServe(ctx, handler)

	log.Info().Msg("Makshiftd stopped")
	return err
}

func listenAndServe(ctx context.Context, h http.Handler) error {
	if ctx == nil {
		ctx = context.TODO()
	}

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	shutdownCtx, shutdownCancel := context.WithCancel(ctx)
	defer shutdownCancel()

	server := &http.Server{
		BaseContext: func(l net.Listener) context.Context {
			return serverCtx
		},
	}

	return wg.Work(
		context.Background(),
		wg.NewUnlimited(),
		wg.CancelOnFirstError(),

		func(ctx context.Context) error {

			shutdownSignal := make(chan os.Signal, 1)
			signal.Notify(shutdownSignal, os.Interrupt)

			select {
			case <-shutdownSignal:
				serverCancel()
			case <-serverCtx.Done():
				break
			}

			shutdownTimer := time.NewTimer(30 * time.Second)
			defer shutdownTimer.Stop()

			for {
				select {
				case <-shutdownSignal:
					signal.Stop(shutdownSignal)
					shutdownCancel()
				case <-shutdownTimer.C:
					shutdownCancel()
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		},
		func(ctx context.Context) error {
			select {
			case <-serverCtx.Done():
				return server.Shutdown(shutdownCtx)
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		func(ctx context.Context) error {
			err := server.ListenAndServe()
			// If a server shutdown has not been initiated,
			// then return the error from ListenAndServe().
			select {
			case <-serverCtx.Done():
				break
			default:
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)
}
