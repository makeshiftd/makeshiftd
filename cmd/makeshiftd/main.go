package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	wg "github.com/dxmaxwell/workgroup"
	"github.com/makeshiftd/makeshiftd/context"
)

func main() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		w := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = os.Stdout
		})
		log.Logger = zerolog.New(w).With().Timestamp().Logger()
	}

	ctx := context.Background()
	mainCtx, mainCancel := context.WithCancel(ctx)
	shutdownCtx, shutdownCancel := context.WithCancel(ctx)

	wg.Work(context.WithLog(ctx, log.With().Str("wg", "main")), nil, wg.CancelOnFirstDone(),
		func(ctx context.C) error {
			log := log.Ctx(ctx).With().Str("wk", "shutdown signal").Logger()

			const StateStarted = "STARTED"
			const StateStopping = "STOPPING"
			const StateExiting = "EXITING"
			var state = StateStarted

			shutdownTimeout := make(<-chan time.Time)
			shutdownSignal := make(chan os.Signal, 1)
			signal.Notify(shutdownSignal, os.Interrupt)

			for {
				select {
				case s := <-shutdownSignal:
					log.Info().Msgf("Shutdown signal recieved: %s", s)
					if state == StateStarted {
						shutdownTimeout = time.After(30 * time.Second)
						state = StateStopping
						mainCancel()
					} else if state == StateStopping {
						shutdownTimeout = nil
						state = StateExiting
						shutdownCancel()
					} else {
						os.Exit(2)
					}

				case <-shutdownTimeout:
					log.Debug().Msg("Shutdown timeout: cancel context")
					shutdownCancel()

				case <-mainCtx.Done():
					log.Trace().Msg("Worker context cancelled")
					return mainCtx.Err()
				}
			}
		},
		func(ctx context.C) error {
			log := log.Ctx(ctx).With().Str("wk", "main with contexts").Logger()
			err := mainWithContexts(log.WithContext(mainCtx), shutdownCtx)
			if err != nil {
				log.Err(err).Send()
				if err, ok := err.(interface{ ExitCode() int }); ok {
					os.Exit(err.ExitCode())
				}
				os.Exit(1)
			}
			return nil
		},
	)
}

func mainWithContexts(mainCtx, shutdownCtx context.C) error {
	log := log.Ctx(mainCtx)

	pflag.StringP("config", "f", "", "Location of configuration file")
	pflag.Parse()

	viper.BindPFlag("configFile", pflag.Lookup("config"))

	viper.SetConfigName("makeshiftd")
	viper.AddConfigPath("/etc/makeshiftd")
	viper.AddConfigPath("$HOME/.makeshifted")

	if viper.GetString("configFile") != "" {
		viper.SetConfigFile(viper.GetString("configFile"))
	}

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {
		return err
	}

	handler := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Write([]byte("<html><body>Hello World</body></html>"))
	})

	log.Info().Msg("Makeshiftd starting")
	err = listenAndServe(mainCtx, shutdownCtx, handler)
	log.Info().Msg("Makshiftd stopped")
	return err
}

func listenAndServe(serverCtx, shutdownCtx context.C, handler http.Handler) error {
	// if ctx == nil {
	// 	ctx = context.TODO()
	// }

	// serverCtx, serverCancel := context.WithCancel(ctx)
	// defer serverCancel()

	// shutdownCtx, shutdownCancel := context.WithCancel(ctx)
	// defer shutdownCancel()

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
		BaseContext: func(l net.Listener) context.Context {
			return serverCtx
		},
	}

	return wg.Work(
		context.Background(),
		wg.NewUnlimited(),
		wg.CancelOnFirstDone(),

		// func(ctx context.Context) error {

		// 	shutdownSignal := make(chan os.Signal, 1)
		// 	signal.Notify(shutdownSignal, os.Interrupt)

		// 	select {
		// 	case <-shutdownSignal:
		// 		log.Debug().Msg("Interrupt signal recieved: start shutdown")
		// 		serverCancel()
		// 	case <-serverCtx.Done():
		// 		log.Trace().Msg("HTTP server shutdown signal worker done")
		// 		break
		// 	}

		// 	shutdownTimer := time.NewTimer(30 * time.Second)
		// 	defer shutdownTimer.Stop()

		// 	for {
		// 		select {
		// 		case <-shutdownSignal:
		// 			log.Debug().Msg("Interrupt signal recieved: cancel shutdown")
		// 			signal.Stop(shutdownSignal)
		// 			log.Info().Msg("Http server shutdown interrupted")
		// 			shutdownCancel()
		// 		case <-shutdownTimer.C:
		// 			log.Info().Msg("HTTP server shutdown timeout (30s)")
		// 			shutdownCancel()
		// 		case <-ctx.Done():
		// 			log.Trace().Msg("HTTP server shutdown signal/timeout worker done")
		// 			return ctx.Err()
		// 		}
		// 	}
		// },
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
