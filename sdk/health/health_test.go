package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivenessHandlerHealthyByDefault(t *testing.T) {
	h := New(Options{})
	resp := runProbe(t, h.LivenessHandler(), "/livez")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	if resp.Body.Probe != probeLiveness || resp.Body.Status != statusOK {
		t.Fatalf("unexpected response: %+v", resp.Body)
	}
	if resp.Body.State.ShuttingDown {
		t.Fatal("unexpected shuttingDown state")
	}
	if resp.Body.State.StartupComplete {
		t.Fatal("unexpected startupComplete state")
	}
}

func TestReadinessHandlerFailsWhenCheckFails(t *testing.T) {
	h := New(Options{
		Readiness: map[string]Check{
			"db": func(context.Context) error { return errors.New("down") },
		},
	})
	resp := runProbe(t, h.ReadinessHandler(), "/readyz")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", resp.Code)
	}
	got := resp.Body.Checks["db"]
	if got != "down" {
		t.Fatalf("db check = %q", got)
	}
}

func TestStartupHandlerFallsBackToReadinessAndLatches(t *testing.T) {
	ready := false
	h := New(Options{
		Readiness: map[string]Check{
			"db": func(context.Context) error {
				if !ready {
					return errors.New("warming up")
				}
				return nil
			},
		},
	})

	first := runProbe(t, h.StartupHandler(), "/startupz")
	if first.Code != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d", first.Code)
	}
	if first.Body.State.StartupComplete {
		t.Fatal("startup should not be complete before first success")
	}

	ready = true
	second := runProbe(t, h.StartupHandler(), "/startupz")
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d", second.Code)
	}
	if !second.Body.State.StartupComplete {
		t.Fatal("startup should latch after first success")
	}
	got := second.Body.Checks["db"]
	if got != statusOK {
		t.Fatalf("db check = %q", got)
	}

	ready = false
	third := runProbe(t, h.StartupHandler(), "/startupz")
	if third.Code != http.StatusOK {
		t.Fatalf("third status = %d", third.Code)
	}
	if !third.Body.State.StartupComplete {
		t.Fatal("startup latch unexpectedly reset")
	}
	if len(third.Body.Checks) != 0 {
		t.Fatalf("latched startup checks = %v", third.Body.Checks)
	}
}

func TestMarkShuttingDownAffectsReadinessOnly(t *testing.T) {
	h := New(Options{
		Readiness: map[string]Check{
			"db": func(context.Context) error { return nil },
		},
	})

	resp := runProbe(t, h.StartupHandler(), "/startupz")
	if resp.Code != http.StatusOK {
		t.Fatalf("startup status = %d", resp.Code)
	}
	h.MarkShuttingDown()

	live := runProbe(t, h.LivenessHandler(), "/livez")
	if live.Code != http.StatusOK {
		t.Fatalf("livez status = %d", live.Code)
	}
	if !live.Body.State.ShuttingDown {
		t.Fatal("livez should reflect shutdown state")
	}

	ready := runProbe(t, h.ReadinessHandler(), "/readyz")
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d", ready.Code)
	}
	if ready.Body.Status != statusFailed {
		t.Fatalf("readyz status text = %q", ready.Body.Status)
	}
	got := ready.Body.Checks["db"]
	if got != statusOK {
		t.Fatalf("db check = %q", got)
	}

	startup := runProbe(t, h.StartupHandler(), "/startupz")
	if startup.Code != http.StatusOK {
		t.Fatalf("startupz status = %d", startup.Code)
	}
	if !startup.Body.State.StartupComplete {
		t.Fatal("startup should remain latched during shutdown")
	}
}

func TestRegisterMountsCanonicalEndpoints(t *testing.T) {
	h := New(Options{})
	mux := http.NewServeMux()
	h.Register(mux)

	for _, path := range []string{"/livez", "/readyz", "/startupz"} {
		resp := runProbe(t, mux, path)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, resp.Code)
		}
	}

	legacy := httptest.NewRecorder()
	mux.ServeHTTP(legacy, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil))
	if legacy.Code != http.StatusNotFound {
		t.Fatalf("legacy /healthz status = %d", legacy.Code)
	}
}

type probeResult struct {
	Code int
	Body probeResponse
}

func runProbe(t *testing.T, handler http.Handler, path string) probeResult {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var body probeResponse
	err := decodeJSON(w, &body)
	if err != nil {
		t.Fatal(err)
	}
	return probeResult{Code: w.Code, Body: body}
}

func decodeJSON(w *httptest.ResponseRecorder, out *probeResponse) error {
	if w.Code == http.StatusNotFound {
		return nil
	}
	return json.NewDecoder(w.Result().Body).Decode(out)
}
