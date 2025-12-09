package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithSecurityHeaders(t *testing.T) {
	// 1. Create a dummy handler that does nothing
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 2. Wrap it with the security middleware
	secureHandler := WithSecurityHeaders(dummyHandler)

	// 3. Create a request and recorder
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// 4. Serve the request
	secureHandler.ServeHTTP(w, req)

	// 5. Verify Headers
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}

	for key, expectedValue := range expectedHeaders {
		if got := w.Header().Get(key); got != expectedValue {
			t.Errorf("Header %s: expected '%s', got '%s'", key, expectedValue, got)
		}
	}

	// Ensure the request actually passed through to the handler
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
