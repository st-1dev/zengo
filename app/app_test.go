package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestRunUsesBeforeShutdownHooksOnServeError(t *testing.T) {
	var (
		mu    sync.Mutex
		order []string
	)
	record := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	serveErr := errors.New("boom")
	a := New(Options{})
	a.AddBeforeShutdown(func(context.Context) error {
		record("before")
		return nil
	})
	a.AddComponent("test", &testComponent{
		serveErr: serveErr,
		onShutdown: func() {
			record("component")
		},
	})
	a.AddCleanup(func(context.Context) error {
		record("cleanup")
		return nil
	})

	err := a.Run(context.Background())
	if !errors.Is(err, serveErr) {
		t.Fatalf("run error = %v", err)
	}
	if !slices.Equal(order, []string{"before", "component", "cleanup"}) {
		t.Fatalf("shutdown order = %v", order)
	}
}

func TestRunUsesBeforeShutdownHooksOnCancellation(t *testing.T) {
	var (
		mu    sync.Mutex
		order []string
	)
	record := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	a := New(Options{})
	a.AddBeforeShutdown(func(context.Context) error {
		record("before")
		return nil
	})
	a.AddComponent("test", &testComponent{
		stopCh: make(chan struct{}),
		onShutdown: func() {
			record("component")
		},
	})
	a.AddCleanup(func(context.Context) error {
		record("cleanup")
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- a.Run(ctx)
	}()
	cancel()

	err := <-done
	if err != nil {
		t.Fatalf("run error = %v", err)
	}
	if !slices.Equal(order, []string{"before", "component", "cleanup"}) {
		t.Fatalf("shutdown order = %v", order)
	}
}

func TestRunJoinsShutdownErrors(t *testing.T) {
	hookErr := errors.New("hook failed")
	componentErr := errors.New("component failed")
	cleanupErr := errors.New("cleanup failed")
	a := New(Options{})
	a.AddBeforeShutdown(func(context.Context) error {
		return hookErr
	})
	a.AddComponent("test", &testComponent{
		stopCh:      make(chan struct{}),
		shutdownErr: componentErr,
	})
	a.AddCleanup(func(context.Context) error {
		return cleanupErr
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := a.Run(ctx)
	for _, expected := range []error{hookErr, componentErr, cleanupErr} {
		if !errors.Is(err, expected) {
			t.Fatalf("Run() error = %v, want joined %v", err, expected)
		}
	}
}

func TestRunReportsShutdownTimeout(t *testing.T) {
	a := New(Options{ShutdownTimeout: time.Millisecond})
	a.AddComponent("test", blockingComponent{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := a.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}
}

type testComponent struct {
	serveErr    error
	stopCh      chan struct{}
	onShutdown  func()
	shutdownErr error
	stopOnce    sync.Once
}

func (c *testComponent) Serve() error {
	if c.serveErr != nil {
		return c.serveErr
	}
	<-c.stopCh
	return http.ErrServerClosed
}

func (c *testComponent) Shutdown(context.Context) error {
	if c.onShutdown != nil {
		c.onShutdown()
	}
	if c.stopCh != nil {
		c.stopOnce.Do(func() {
			close(c.stopCh)
		})
	}
	return c.shutdownErr
}

type blockingComponent struct{}

func (blockingComponent) Serve() error {
	time.Sleep(10 * time.Millisecond)
	return http.ErrServerClosed
}

func (blockingComponent) Shutdown(context.Context) error {
	return nil
}
