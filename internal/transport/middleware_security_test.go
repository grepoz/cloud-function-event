package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithSecurityHeaders(t *testing.T) {
	// Define the base handler once
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Define headers that should ALWAYS be present regardless of environment
	commonHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
	}

	tests := []struct {
		name     string
		isProd   bool
		wantHSTS bool
	}{
		{
			name:     "Production_HasHSTS",
			isProd:   true,
			wantHSTS: true,
		},
		{
			name:     "Dev_NoHSTS",
			isProd:   false,
			wantHSTS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize middleware with the test case's config
			secureHandler := WithSecurityHeaders(dummyHandler, tt.isProd)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			secureHandler.ServeHTTP(w, req)

			// 1. Verify Status Code (Ensure request passed through)
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// 2. Verify Common Security Headers (Must exist in ALL modes)
			for key, expectedValue := range commonHeaders {
				if got := w.Header().Get(key); got != expectedValue {
					t.Errorf("Header %s: expected %q, got %q", key, expectedValue, got)
				}
			}

			// 3. Verify HSTS Header (Conditional)
			hsts := w.Header().Get("Strict-Transport-Security")
			if tt.wantHSTS && hsts == "" {
				t.Error("Expected HSTS header in production mode, but it was missing")
			} else if !tt.wantHSTS && hsts != "" {
				t.Errorf("Expected NO HSTS header in dev mode, but got: %q", hsts)
			}
		})
	}
}
