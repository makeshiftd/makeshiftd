package context

import (
	"context"

	"github.com/rs/zerolog"
)

type C = context.Context
type Context = context.Context

var TODO = context.TODO
var Background = context.Background

var WithValue = context.WithValue
var WithCancel = context.WithCancel
var WithTimeout = context.WithTimeout
var WithDeadline = context.WithDeadline

func WithLog(c C, log zerolog.Context) C {
	logger := log.Logger()
	return logger.WithContext(c)
}
