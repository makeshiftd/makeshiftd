package loggers

import (
	"context"
	"io"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LazyLogger wraps a zerolog Logger and initializes it to the root logger on first use.
type LazyLogger struct {
	logger zerolog.Logger

	configFunc func()
	configOnce sync.Once
}

// NewLazyLogger builds a new LazyLogger an applies the provided configuration on initialization
func NewLazyLogger(config func(ctx zerolog.Context) zerolog.Context) *LazyLogger {
	l := &LazyLogger{}
	l.configFunc = func() {
		c := log.Logger.With()
		if config != nil {
			c = config(c)
		}
		l.logger = c.Logger()
	}
	return l
}

// NewLazyLoggerPkg builds a new LazyLogger and configures it with the 'pkg' provided
func NewLazyLoggerPkg(pkg string) *LazyLogger {
	return NewLazyLogger(func(ctx zerolog.Context) zerolog.Context {
		return ctx.Str("pkg", pkg)
	})
}

func (l *LazyLogger) log() *zerolog.Logger {
	l.configOnce.Do(l.configFunc)
	return &l.logger
}

// Functions and docs below mostly copied from Zerolog source code

// Output duplicates the global logger and sets w as its output.
func (l *LazyLogger) Output(w io.Writer) zerolog.Logger {
	return l.log().Output(w)
}

// With creates a child logger with the field added to its context.
func (l *LazyLogger) With() zerolog.Context {
	return l.log().With()
}

// Level creates a child logger with the minimum accepted level set to level.
func (l *LazyLogger) Level(level zerolog.Level) zerolog.Logger {
	return l.log().Level(level)
}

// Sample returns a logger with the s sampler.
func (l *LazyLogger) Sample(s zerolog.Sampler) zerolog.Logger {
	return l.log().Sample(s)
}

// Hook returns a logger with the h Hook.
func (l *LazyLogger) Hook(h zerolog.Hook) zerolog.Logger {
	return l.log().Hook(h)
}

// Err starts a new message with error level with err as a field if not nil or
// with info level if err is nil.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Err(err error) *zerolog.Event {
	return l.log().Err(err)
}

// Trace starts a new message with trace level.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Trace() *zerolog.Event {
	return l.log().Trace()
}

// Debug starts a new message with debug level.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Debug() *zerolog.Event {
	return l.log().Debug()
}

// Info starts a new message with info level.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Info() *zerolog.Event {
	return l.log().Info()
}

// Warn starts a new message with warn level.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Warn() *zerolog.Event {
	return l.log().Warn()
}

// Error starts a new message with error level.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Error() *zerolog.Event {
	return l.log().Error()
}

// Fatal starts a new message with fatal level. The os.Exit(1) function
// is called by the Msg method.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Fatal() *zerolog.Event {
	return l.log().Fatal()
}

// Panic starts a new message with panic level. The message is also sent
// to the panic function.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Panic() *zerolog.Event {
	return l.log().Panic()
}

// WithLevel starts a new message with level.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) WithLevel(level zerolog.Level) *zerolog.Event {
	return l.log().WithLevel(level)
}

// Log starts a new message with no level. Setting zerolog.GlobalLevel to
// zerolog.Disabled will still disable events produced by this method.
//
// You must call Msg on the returned event in order to send the event.
func (l *LazyLogger) Log() *zerolog.Event {
	return l.log().Log()
}

// Print sends a log event using debug level and no extra field.
// Arguments are handled in the manner of fmt.Print.
func (l *LazyLogger) Print(v ...interface{}) {
	l.log().Print(v...)
}

// Printf sends a log event using debug level and no extra field.
// Arguments are handled in the manner of fmt.Printf.
func (l *LazyLogger) Printf(format string, v ...interface{}) {
	l.log().Printf(format, v...)
}

// Ctx returns the Logger associated with the ctx. If no logger
// is associated, a disabled logger is returned.
func Ctx(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}
