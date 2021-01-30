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
		wg.CancelOnFirstDone(),

		func(ctx context.Context) error {

			shutdownSignal := make(chan os.Signal, 1)
			signal.Notify(shutdownSignal, os.Interrupt)

			select {
			case <-shutdownSignal:
				log.Debug().Msg("Interrupt signal recieved: start shutdown")
				serverCancel()
			case <-serverCtx.Done():
				log.Trace().Msg("HTTP server shutdown signal worker done")
				break
			}

			shutdownTimer := time.NewTimer(30 * time.Second)
			defer shutdownTimer.Stop()

			for {
				select {
				case <-shutdownSignal:
					log.Debug().Msg("Interrupt signal recieved: cancel shutdown")
					signal.Stop(shutdownSignal)
					log.Info().Msg("Http server shutdown interrupted")
					shutdownCancel()
				case <-shutdownTimer.C:
					log.Info().Msg("HTTP server shutdown timeout (30s)")
					shutdownCancel()
				case <-ctx.Done():
					log.Trace().Msg("HTTP server shutdown signal/timeout worker done")
					return ctx.Err()
				}
			}
		},
		func(ctx context.Context) error {
			select {
			case <-serverCtx.Done():
				log.Info().Msg("HTTP server shutdown started")
				err := server.Shutdown(shutdownCtx)
				log.Info().Err(err).Msg("HTTP server shutdown complete")
				return err
			case <-ctx.Done():
				log.Trace().Msg("HTTP server shutdown worker done")
				return ctx.Err()
			}
		},
		func(ctx context.Context) error {
			err := server.ListenAndServe()
			log.Info().Err(err).Msg("HTTP server listener stopped")
			// If a server shutdown has not been initiated,
			// then return the error from ListenAndServe().
			select {
			case <-serverCtx.Done():
				break
			default:
				log.Trace().Msg("HTTP listen and serve worker failed")
				return err
			}

			select {
			case <-ctx.Done():
				log.Trace().Msg("HTTP listen and serve worker done")
				return ctx.Err()
			}
		},
	)
}
