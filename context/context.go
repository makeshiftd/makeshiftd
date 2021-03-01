package context

import (
	"context"
)

type C = context.Context
type Context = context.Context
type CancelFunc = context.CancelFunc

var TODO = context.TODO
var Background = context.Background

var WithValue = context.WithValue
var WithCancel = context.WithCancel
var WithTimeout = context.WithTimeout
var WithDeadline = context.WithDeadline
