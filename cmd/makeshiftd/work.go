package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	wg "github.com/dxmaxwell/workgroup"
)

type SelectCtxFunc func(worker, request context.Context) (context.Context, context.CancelFunc)

func WorkerContext() SelectCtxFunc {
	return func(worker, request context.Context) (context.Context, context.CancelFunc) {
		return worker, func() {}
	}
}

func RequestContext() SelectCtxFunc {
	return func(worker, request context.Context) (context.Context, context.CancelFunc) {
		return request, func() {}
	}
}

type mergedContext struct {
	ctx1 context.Context
	ctx2 context.Context

	err error
	mtx sync.Mutex

	done chan struct{}
	once sync.Once
}

func MergedContext() SelectCtxFunc {
	return func(worker, request context.Context) (context.Context, context.CancelFunc) {
		m := &mergedContext{
			ctx1: worker,
			ctx2: request,

			err: nil,
			mtx: sync.Mutex{},

			done: make(chan struct{}),
			once: sync.Once{},
		}

		cancel := func(err error) {
			m.once.Do(func() {
				m.mtx.Lock()
				defer m.mtx.Unlock()
				m.err = err
				close(m.done)
			})
		}

		go func() {
			select {
			case <-m.ctx1.Done():
				cancel(m.ctx1.Err())
				return
			case <-m.ctx2.Done():
				cancel(m.ctx2.Err())
				return
			case <-m.done:
				return
			}
		}()

		return m, func() { cancel(context.Canceled) }
	}
}

func (m *mergedContext) Value(key interface{}) interface{} {
	v := m.ctx1.Value(key)
	if v == nil {
		v = m.ctx2.Value(key)
	}
	return v
}

func (m *mergedContext) Done() <-chan struct{} {
	return m.done
}

func (m *mergedContext) Err() error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.err
}

func (m *mergedContext) Deadline() (deadline time.Time, ok bool) {
	deadline1, ok1 := m.ctx1.Deadline()
	deadline2, ok2 := m.ctx2.Deadline()
	if ok1 && ok2 {
		if deadline1.Before(deadline2) {
			return deadline1, true
		}
		return deadline2, true
	}

	if ok1 {
		return deadline1, true
	}

	if ok2 {
		return deadline2, true
	}

	return time.Time{}, false
}

type WorkHandler struct {
	selctx  SelectCtxFunc
	handler http.Handler

	workers chan wg.Worker
	once    sync.Once
}

func NewWorkHandler(handler http.Handler, selctx SelectCtxFunc) *WorkHandler {
	return &WorkHandler{
		selctx:  selctx,
		handler: handler,

		workers: make(chan wg.Worker),
		once:    sync.Once{},
	}
}

func (h *WorkHandler) Chan() <-chan wg.Worker {
	return h.workers
}

func (h *WorkHandler) Close() {
	h.once.Do(func() { close(h.workers) })
}

func (h *WorkHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	w := &sync.WaitGroup{}
	w.Add(1)
	h.workers <- func(ctx context.Context) error {
		defer w.Done()

		ctx, cancel := h.selctx(ctx, req.Context())
		defer cancel()

		h.handler.ServeHTTP(res, req.WithContext(ctx))
		return nil
	}
	w.Wait()
}
