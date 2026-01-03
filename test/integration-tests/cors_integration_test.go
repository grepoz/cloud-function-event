package integration_tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestIntegration_CORS_Preflight(t *testing.T) {
	// Reuse your existing setup logic that builds the router and middleware stack
	router, _ := setupIntegration(t)

	t.Run("Verify_Preflight_Response", func(t *testing.T) {
		// 1. Create an OPTIONS request with the Origin header
		req := httptest.NewRequest(http.MethodOptions, "/events/", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")

		w := httptest.NewRecorder()

		// 2. Serve the request
		router.ServeHTTP(w, req)

		// 3. Assertions based on industry best practices
		// If using rs/cors, this will be 204. If using your current manual code, it's 200.
		if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
			t.Errorf("Expected 204 No Content or 200 OK, got %d", w.Code)
		}

		// Verify CORS headers exist
		if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
			t.Error("Missing Access-Control-Allow-Origin header")
		}

		if got := w.Header().Get("Access-Control-Max-Age"); got == "" {
			t.Error("Missing Access-Control-Max-Age header (required for performance)")
		}
	})
}

func TestIntegration_CORS_UnauthorizedOrigin(t *testing.T) {
	// 1. Force a specific allowed origin for the test
	os.Setenv("CORS_ALLOWED_ORIGIN", "https://trusted-app.com")
	router, _ := setupIntegration(t) // Assuming this builds your middleware stack

	t.Run("Reject_Unauthorized_Origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/events/", nil)

		// Simulate a request from an evil domain
		req.Header.Set("Origin", "https://evil-attacker.com")
		req.Header.Set("Access-Control-Request-Method", "POST")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// ASSERTION: The Access-Control-Allow-Origin header MUST be empty
		// rs/cors will not return the header if the origin is not allowed.
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Security Risk: Expected no Access-Control-Allow-Origin header for unauthorized origin, but got %s", got)
		}
	})
}
