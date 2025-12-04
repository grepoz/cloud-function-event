package transport

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

// NewRouter initializes the main HTTP handler using Go 1.22+ ServeMux
func NewRouter(eventSvc service.EventService, trackingSvc service.TrackingService) http.Handler {
	mux := http.NewServeMux()

	// --- Events ---
	eventHandler := NewEventHandler(eventSvc)
	// 1. Main registration with trailing slash (canonical)
	mux.Handle("/events/", http.StripPrefix("/events", eventHandler))

	// 2. Fix: Explicitly handle missing slash.
	// Redirect using 307 (Temporary Redirect) to preserve POST method and body.
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		target := "/events/"
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})

	// --- Tracking ---
	trackingHandler := NewTrackingHandler(trackingSvc)
	mux.Handle("/tracking/", http.StripPrefix("/tracking", trackingHandler))

	// Apply the same fix for tracking
	mux.HandleFunc("/tracking", func(w http.ResponseWriter, r *http.Request) {
		target := "/tracking/"
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})

	return mux
}

func WithCORS(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default to "*" if no origin is specified in env
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Handle Preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// respondError is a shared helper for JSON error responses
func respondError(w http.ResponseWriter, err error) {
	if _, ok := err.(*domain.ValidationError); ok {
		w.WriteHeader(http.StatusBadRequest)
	} else if err.Error() == "event not found" {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Error: err.Error()})
}

// WithCompression is a middleware for Brotli compression
func WithCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "br")
		br := brotli.NewWriter(w)
		defer func(br *brotli.Writer) {
			_ = br.Close()
		}(br)
		cw := &compressedWriter{w: w, cw: br}
		next.ServeHTTP(cw, r)
	})
}

type compressedWriter struct {
	w  http.ResponseWriter
	cw *brotli.Writer
}

func (cw *compressedWriter) Header() http.Header         { return cw.w.Header() }
func (cw *compressedWriter) Write(b []byte) (int, error) { return cw.cw.Write(b) }
func (cw *compressedWriter) WriteHeader(statusCode int)  { cw.w.WriteHeader(statusCode) }
