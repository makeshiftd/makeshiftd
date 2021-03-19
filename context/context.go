package context

import (
	"context"
	"sync"
	"time"
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

type merged struct {
	primary   context.Context
	secondary context.Context

	err    error
	errMut sync.Mutex

	done     chan struct{}
	doneOnce sync.Once
}

// Merge merges two contexts, when obtaining values from the
// resulting context the primary context is checked first.
func Merge(primary, secondary C) (C, CancelFunc) {

	m := &merged{
		primary:   primary,
		secondary: secondary,

		done: make(chan struct{}),
	}

	cancel := func(err error) {
		m.doneOnce.Do(func() {
			m.errMut.Lock()
			defer m.errMut.Unlock()
			m.err = err
			close(m.done)
		})
	}

	go func() {
		select {
		case <-m.primary.Done():
			cancel(m.primary.Err())
			return
		case <-m.secondary.Done():
			cancel(m.secondary.Err())
			return
		case <-m.done:
			return
		}
	}()

	return m, func() { cancel(context.Canceled) }
}

func (m *merged) Value(key interface{}) interface{} {
	v := m.primary.Value(key)
	if v == nil {
		v = m.secondary.Value(key)
	}
	return v
}

func (m *merged) Done() <-chan struct{} {
	return m.done
}

func (m *merged) Err() error {
	m.errMut.Lock()
	defer m.errMut.Unlock()
	return m.err
}

func (m *merged) Deadline() (deadline time.Time, ok bool) {
	priDeadline, priOK := m.primary.Deadline()
	secDeadline, secOK := m.secondary.Deadline()
	if priOK && secOK {
		if priDeadline.Before(secDeadline) {
			return priDeadline, true
		}
		return secDeadline, true
	}

	if priOK {
		return priDeadline, true
	}

	if secOK {
		return secDeadline, true
	}

	return time.Time{}, false
}
