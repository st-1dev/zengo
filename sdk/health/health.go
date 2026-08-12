package health

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"sync/atomic"
)

const (
	probeLiveness  = "liveness"
	probeReadiness = "readiness"
	probeStartup   = "startup"
	statusOK       = "ok"
	statusFailed   = "failed"
)

// Check reports probe state for the current request context.
type Check func(context.Context) error

// Options groups checks by probe semantics.
type Options struct {
	// Liveness holds checks that determine whether the process is alive.
	Liveness map[string]Check
	// Readiness holds checks that determine whether the process can receive traffic.
	Readiness map[string]Check
	// Startup holds checks that gate startup success before the handler latches healthy.
	Startup map[string]Check
}

// Handler serves Kubernetes-style liveness, readiness, and startup probes.
type Handler struct {
	liveness        map[string]Check
	readiness       map[string]Check
	startup         map[string]Check
	shuttingDown    atomic.Bool
	startupComplete atomic.Bool
}

// New constructs a probe-aware health handler.
func New(opts Options) *Handler {
	return &Handler{
		liveness:  cloneChecks(opts.Liveness),
		readiness: cloneChecks(opts.Readiness),
		startup:   cloneChecks(opts.Startup),
	}
}

// LivenessHandler serves /livez.
func (h *Handler) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, probeLiveness)
	})
}

// ReadinessHandler serves /readyz.
func (h *Handler) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, probeReadiness)
	})
}

// StartupHandler serves /startupz.
func (h *Handler) StartupHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, probeStartup)
	})
}

// Register mounts canonical Kubernetes probe endpoints on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("/livez", h.LivenessHandler())
	mux.Handle("/readyz", h.ReadinessHandler())
	mux.Handle("/startupz", h.StartupHandler())
}

// MarkShuttingDown flips readiness into failed state while keeping liveness intact.
func (h *Handler) MarkShuttingDown() {
	h.shuttingDown.Store(true)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, probe string) {
	resp := probeResponse{
		Probe: probe,
		State: probeState{
			ShuttingDown:    h.shuttingDown.Load(),
			StartupComplete: h.startupComplete.Load(),
		},
		Checks: map[string]string{},
	}

	var status int
	switch probe {
	case probeLiveness:
		resp.Checks, status = runChecks(r.Context(), h.liveness)
	case probeReadiness:
		resp.Checks, status = runChecks(r.Context(), h.readiness)
		if resp.State.ShuttingDown {
			status = http.StatusServiceUnavailable
		}
	case probeStartup:
		if resp.State.StartupComplete {
			resp.Status = statusOK
			h.write(w, http.StatusOK, resp)
			return
		}
		checks := h.startup
		if len(checks) == 0 {
			checks = h.readiness
		}
		resp.Checks, status = runChecks(r.Context(), checks)
		if status == http.StatusOK {
			h.startupComplete.Store(true)
			resp.State.StartupComplete = true
		}
	default:
		http.NotFound(w, r)
		return
	}

	if status == http.StatusOK {
		resp.Status = statusOK
	} else {
		resp.Status = statusFailed
	}
	h.write(w, status, resp)
}

func (h *Handler) write(w http.ResponseWriter, status int, resp probeResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func runChecks(ctx context.Context, checks map[string]Check) (map[string]string, int) {
	results := make(map[string]string, len(checks))
	status := http.StatusOK
	for name, check := range checks {
		err := check(ctx)
		if err != nil {
			status = http.StatusServiceUnavailable
			results[name] = err.Error()
			continue
		}
		results[name] = statusOK
	}
	return results, status
}

func cloneChecks(src map[string]Check) map[string]Check {
	if len(src) == 0 {
		return map[string]Check{}
	}
	dst := make(map[string]Check, len(src))
	maps.Copy(dst, src)
	return dst
}

// probeResponse is the stable JSON payload returned by probe handlers.
type probeResponse struct {
	// Probe is the canonical probe name.
	Probe string `json:"probe"`
	// Status reports whether the probe succeeded.
	Status string `json:"status"`
	// State exposes runtime lifecycle state alongside the check results.
	State probeState `json:"state"`
	// Checks contains per-check results keyed by logical check name.
	Checks map[string]string `json:"checks"`
}

// probeState captures lifecycle flags that influence probe behavior.
type probeState struct {
	// ShuttingDown reports whether the service has entered graceful shutdown.
	ShuttingDown bool `json:"shuttingDown"`
	// StartupComplete reports whether the startup probe has latched healthy.
	StartupComplete bool `json:"startupComplete"`
}
