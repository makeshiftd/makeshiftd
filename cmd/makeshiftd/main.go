package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	wg "github.com/dxmaxwell/workgroup"
)

func listenAndServe(ctx context.Context, h http.Handler) {
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

	wg.Work(
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
