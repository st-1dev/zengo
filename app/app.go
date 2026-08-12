package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Component is a long-running runtime unit managed by App.
type Component interface {
	// Serve starts the component and blocks until it stops.
	Serve() error
	// Shutdown asks the component to stop within the provided deadline.
	Shutdown(ctx context.Context) error
}

// NamedComponent pairs a runtime component with a stable log/shutdown name.
type NamedComponent struct {
	// Name is the stable identifier used in logs and shutdown reporting.
	Name string
	// Component is the runtime unit to start and stop.
	Component
}

// Options controls App lifecycle behavior.
type Options struct {
	// ShutdownTimeout bounds the total shutdown window for hooks and components.
	ShutdownTimeout time.Duration
}

// App runs components until one fails or its context is canceled.
type App struct {
	components     []NamedComponent
	beforeShutdown []func(context.Context) error
	cleanups       []func(context.Context) error
	opts           Options
}

// New creates an App with sensible shutdown defaults.
func New(opts Options) *App {
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 15 * time.Second
	}
	return &App{opts: opts}
}

// AddCleanup registers a cleanup hook that runs after components shut down.
func (a *App) AddCleanup(fn func(context.Context) error) {
	a.cleanups = append(a.cleanups, fn)
}

// AddBeforeShutdown registers a hook that runs before components begin shutting down.
func (a *App) AddBeforeShutdown(fn func(context.Context) error) {
	a.beforeShutdown = append(a.beforeShutdown, fn)
}

// AddComponent registers a managed runtime component in startup order.
func (a *App) AddComponent(name string, c Component) {
	a.components = append(a.components, NamedComponent{Name: name, Component: c})
}

// Run starts all components and blocks until cancellation or the first component exit.
//
// Components are shut down in registration order. Cleanup hooks run in reverse
// registration order after component shutdown completes.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, len(a.components))
	var wg sync.WaitGroup
	for _, c := range a.components {
		component := c
		wg.Go(func() {
			errCh <- component.Serve()
		})
	}

	var runErr error
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = err
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.opts.ShutdownTimeout)
	defer cancel()

	shutdownErr := a.shutdown(shutdownCtx)
	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()
	allStopped := false
	select {
	case <-stopped:
		allStopped = true
	case <-shutdownCtx.Done():
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("wait for components: %w", shutdownCtx.Err()))
	}

	if allStopped {
		close(errCh)
		for err := range errCh {
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				runErr = errors.Join(runErr, err)
			}
		}
	}
	return errors.Join(runErr, shutdownErr)
}

func (a *App) shutdown(ctx context.Context) error {
	var shutdownErr error
	for i, hook := range a.beforeShutdown {
		err := hook(ctx)
		if err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("before shutdown hook %d: %w", i, err))
		}
	}
	for _, c := range a.components {
		err := c.Shutdown(ctx)
		if err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("shutdown component %q: %w", c.Name, err))
		}
	}
	for i := len(a.cleanups) - 1; i >= 0; i-- {
		err := a.cleanups[i](ctx)
		if err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("cleanup hook %d: %w", i, err))
		}
	}
	return shutdownErr
}
