package transport

import (
	"bibently.com/backend/internal/domain"
	"bibently.com/backend/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/andybalholm/brotli"
)

// ProjectID is needed for trace formatting. Fetch it once.
var googleProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT")

// Setup logger with a Handler that handles context for Tracing
var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		// Map standard keys to Google Cloud Logging keys
		if a.Key == slog.LevelKey {
			a.Key = "severity"
		}
		if a.Key == slog.MessageKey {
			a.Key = "message"
		}
		return a
	},
}))

// Log helper to extract trace from context and inject it into the log entry
func logError(ctx context.Context, msg string, err error) {
	args := []any{"error", err}

	// Add Trace ID to the log entry if present in context
	if traceID, ok := ctx.Value("trace_id").(string); ok && googleProjectID != "" {
		// GCP Format: projects/[PROJECT-ID]/traces/[TRACE-ID]
		traceVal := fmt.Sprintf("projects/%s/traces/%s", googleProjectID, traceID)
		args = append(args, "logging.googleapis.com/trace", traceVal)
	}

	logger.ErrorContext(ctx, msg, args...)
}

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

// WithTraceID extracts the Google Cloud Trace ID header.
// This allows logs to be correlated in the Logs Explorer.
func WithTraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceHeader := r.Header.Get("X-Cloud-Trace-Context")
		var traceID string
		if traceHeader != "" {
			// Header format: TRACE_ID/SPAN_ID;o=TRACE_TRUE
			parts := strings.Split(traceHeader, "/")
			if len(parts) > 0 {
				traceID = parts[0]
			}
		}

		if traceID != "" {
			ctx := context.WithValue(r.Context(), "trace_id", traceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// WithRecovery recovers from panics and logs them as critical errors
func WithRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				err, ok := rec.(error)
				if !ok {
					err = fmt.Errorf("%v", rec)
				}
				// Log with Trace ID context
				logError(r.Context(), "PANIC RECOVERED", err)

				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(domain.APIResponse{Error: "Internal Server Error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func respondError(w http.ResponseWriter, err error) {
	if _, ok := err.(*domain.ValidationError); ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(domain.APIResponse{Error: err.Error()})
		return
	}
	if err.Error() == "event not found" {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(domain.APIResponse{Error: err.Error()})
		return
	}

	// Use context-aware logger
	// We need request context here, but respondError signature doesn't have it.
	// In a real scenario, pass r.Context() to respondError.
	// For now, we log without context or you can refactor respondError to take ctx.
	logger.Error("SERVER ERROR", "error", err.Error(), "component", "api_handler")

	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Error: "Internal Server Error"})
}

func WithCORS(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
