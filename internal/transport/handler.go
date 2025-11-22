package transport

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

type Handler struct {
	service service.EventService
}

func NewHandler(s service.EventService) *Handler {
	return &Handler{service: s}
}

// EntryPoint handles the routing based on Method and Query Params
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		if r.URL.Query().Get("action") == "search" {
			h.handleList(w, r)
		} else {
			h.handleCreate(w, r)
		}

	case http.MethodPut:
		h.handleUpdate(w, r)

	case http.MethodGet:
		h.handleGet(w, r)

	case http.MethodDelete:
		h.handleDelete(w, r)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var event domain.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := h.service.CreateEvent(r.Context(), &event); err != nil {
		h.respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(domain.APIResponse{Data: event.ID})
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	// Decode into a map to support partial updates (Merge)
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Extract ID: try Query param first, then Body
	id := r.URL.Query().Get("id")
	if id == "" {
		if bodyID, ok := updates["id"].(string); ok {
			id = bodyID
		}
	}

	if err := h.service.UpdateEvent(r.Context(), id, updates); err != nil {
		h.respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(domain.APIResponse{Data: "Updated successfully"})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	var searchReq domain.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&searchReq); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	events, err := h.service.ListEvents(r.Context(), searchReq)
	if err != nil {
		h.respondError(w, err)
		return
	}

	json.NewEncoder(w).Encode(domain.APIResponse{Data: events})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		h.respondError(w, err)
		return
	}

	json.NewEncoder(w).Encode(domain.APIResponse{Data: event})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteEvent(r.Context(), id); err != nil {
		h.respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(domain.APIResponse{Data: "Deleted successfully"})
}

func (h *Handler) respondError(w http.ResponseWriter, err error) {
	if _, ok := err.(*domain.ValidationError); ok {
		w.WriteHeader(http.StatusBadRequest)
	} else if err.Error() == "event not found" {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(domain.APIResponse{Error: err.Error()})
}

func WithCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "br")
		br := brotli.NewWriter(w)
		defer br.Close()

		cw := &compressedWriter{w: w, cw: br}
		next.ServeHTTP(cw, r)
	})
}

type compressedWriter struct {
	w  http.ResponseWriter
	cw *brotli.Writer
}

func (cw *compressedWriter) Header() http.Header {
	return cw.w.Header()
}

func (cw *compressedWriter) Write(b []byte) (int, error) {
	return cw.cw.Write(b)
}

func (cw *compressedWriter) WriteHeader(statusCode int) {
	cw.w.WriteHeader(statusCode)
}
