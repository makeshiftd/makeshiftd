package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	wg "github.com/dxmaxwell/workgroup"
	"github.com/makeshiftd/makeshiftd/context"
	"github.com/makeshiftd/makeshiftd/loggers"
)

var log = loggers.NewLazyLoggerPkg("main")

func main() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		w := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = os.Stdout
		})
		zlog.Logger = zerolog.New(w).With().Timestamp().Logger()
	} else {
		// Copied from the zerolog/log source for initialization of global logger
		zlog.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	ctx := context.Background()
	mainCtx, mainCancel := context.WithCancel(ctx)
	shutdownCtx, shutdownCancel := context.WithCancel(ctx)

	log := log.With().Str("wg", "main").Logger()
	wg.Work(ctx, nil, wg.CancelOnFirstDone(),
		func(ctx context.C) error {
			log := log.With().Str("wk", "shutdown").Logger()

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
					log.Trace().Err(mainCtx.Err()).Msg("Worker context done")
					return mainCtx.Err()
				}
			}
		},
		func(ctx context.C) error {
			log := log.With().Str("wk", "main").Logger()
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
	log := loggers.Ctx(mainCtx)

	pflag.StringP("config", "f", "", "Location of configuration file")
	pflag.Parse()

	viper.BindPFlag("configFile", pflag.Lookup("config"))

	viper.SetConfigName("makeshiftd")
	viper.AddConfigPath("$HOME/.makeshifted")
	viper.AddConfigPath("/etc/makeshiftd")

	configFile := viper.GetString("configFile")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	}

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {
		var perr *os.PathError
		if errors.As(err, &perr) {
			log.Err(err).Msgf("Configuration file not read: %s", configFile)
			return err
		}
		var verr viper.ConfigFileNotFoundError
		if !errors.As(err, &verr) {
			return err
		}
	}
	configFile = viper.ConfigFileUsed()
	if configFile != "" {
		log.Info().Msgf("Configuration file read: %s", configFile)
	}

	handler := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		log := zerolog.Ctx(req.Context())
		log.Info().Msg("Hello World")
		res.Write([]byte("<html><body>Hello World</body></html>"))
	})

	log.Info().Msg("Makeshiftd starting")
	err = listenAndServe(mainCtx, shutdownCtx, handler, viper.Sub("server"))
	log.Info().Msg("Makshiftd stopped")
	return err
}

func listenAndServe(serverCtx, shutdownCtx context.C, handler http.Handler, c *viper.Viper) error {

	address := fmt.Sprintf("%s:%s", c.GetString("host"), c.GetString("port"))

	serveWorkers := make(chan wg.Worker)

	server := &http.Server{
		Addr: address,
		Handler: http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			w := sync.WaitGroup{}
			w.Add(1)
			serveWorkers <- func(ctx context.C) error {
				defer w.Done()
				loggers.Ctx(ctx).Trace().Msg("HTTP server serve request")
				handler.ServeHTTP(res, req)
				return nil
			}
			w.Wait()
		}),
	}

	log := log.With().Str("wg", "serve").Logger()
	return wg.Work(context.Background(), nil, wg.CancelOnFirstDone(),
		func(ctx context.C) error {
			log := log.With().Str("wk", "shutdown").Logger()

			defer func() {
				log.Trace().Msg("HTTP server shtudown close worker channel")
				close(serveWorkers)
			}()

			select {
			case <-serverCtx.Done():
				log.Info().Msg("HTTP server shutdown started")
				err := server.Shutdown(shutdownCtx)
				if !errors.Is(err, http.ErrServerClosed) {
					log.Err(err).Msg("HTTP server shutdown complete")
					return err
				}
				log.Info().Msg("HTTP server shutdown complete")
				return nil

			case <-ctx.Done():
				log.Trace().Err(ctx.Err()).Msg("Worker context done")
				return ctx.Err()
			}
		},
		func(ctx context.C) error {
			log := log.With().Str("wk", "listen").Logger()

			log.Info().Msgf("HTTP server listening: %s", address)
			err := server.ListenAndServe()
			if !errors.Is(err, http.ErrServerClosed) {
				log.Err(err).Msg("HTTP server listening stopped")
				return err
			}

			log.Info().Msg("HTTP server listening stopped")
			return nil
		},
		func(ctx context.C) error {
			log := log.With().Str("wk", "serve").Logger()
			return wg.WorkChan(log.WithContext(ctx), nil, wg.CancelNeverFirstError(), serveWorkers)
		},
	)
}
