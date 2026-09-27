package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adapter "github.com/andresfwilt/banco-de-puntitos/internal/adapter/in/http"
)

// The platform needs the API reachable from two dev origins at once: the shell host
// (embedded) and this app's own standalone remote dev server. A single static
// CORS_ORIGIN can't express that, so cors() must accept a comma-separated allowlist.

func corsRequest(t *testing.T, corsOrigin, requestOrigin string) *httptest.ResponseRecorder {
	t.Helper()
	srv := adapter.NewRouter(adapter.UseCases{Points: fakeUC{}}, corsOrigin)
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	if requestOrigin != "" {
		r.Header.Set("Origin", requestOrigin)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

func TestCORS_AllowedOrigin_IsEchoedBack(t *testing.T) {
	for _, origin := range []string{"http://localhost:3000", "http://localhost:3007"} {
		w := corsRequest(t, "http://localhost:3000,http://localhost:3007", origin)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("origin %s: Access-Control-Allow-Origin = %q, want %q", origin, got, origin)
		}
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Fatalf("origin %s: Vary = %q, want %q", origin, got, "Origin")
		}
	}
}

func TestCORS_DisallowedOrigin_GetsNoAllowHeader(t *testing.T) {
	w := corsRequest(t, "http://localhost:3000,http://localhost:3007", "http://evil.example")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCORS_SingleOrigin_BackwardCompatible(t *testing.T) {
	w := corsRequest(t, "http://localhost:8081", "http://localhost:8081")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8081" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:8081")
	}
}

func TestCORS_EmptyConfig_MiddlewareDisabled(t *testing.T) {
	w := corsRequest(t, "", "http://localhost:3000")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty when CORS_ORIGIN unset", got)
	}
}
