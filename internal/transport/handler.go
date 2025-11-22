package transport

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		h.handleCreate(w, r)

	case http.MethodPut:
		h.handleUpdate(w, r)

	case http.MethodGet:
		// If "id" is present, get single event; otherwise list/search
		if r.URL.Query().Has("id") {
			h.handleGet(w, r)
		} else {
			h.handleList(w, r)
		}

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
	// Decode into a map to support partial updates
	var updates map[string]interface{}

	// Use UseNumber to prevent automatic float64 conversion for integers if mixed
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Convert json.Number back to float64 or int64 appropriately for Firestore
	for k, v := range updates {
		if num, ok := v.(json.Number); ok {
			if f, err := num.Float64(); err == nil {
				updates[k] = f
			}
		}
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
	q := r.URL.Query()

	// Build Filters from Query Params
	filters := domain.FilterRequest{
		City: q.Get("city"),
		Type: q.Get("type"),
	}

	// Parse Dates (RFC3339)
	if start := q.Get("start_date"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			filters.StartDate = &t
		}
	}
	if end := q.Get("end_date"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			filters.EndDate = &t
		}
	}

	// Parse Prices (float64)
	if minP := q.Get("min_price"); minP != "" {
		if v, err := strconv.ParseFloat(minP, 64); err == nil {
			filters.MinPrice = &v
		}
	}
	if maxP := q.Get("max_price"); maxP != "" {
		if v, err := strconv.ParseFloat(maxP, 64); err == nil {
			filters.MaxPrice = &v
		}
	}

	// Build Sorting
	sorting := domain.SortRequest{
		SortKey:       q.Get("sort_key"),
		SortDirection: q.Get("sort_dir"),
		PageToken:     q.Get("page_token"),
	}

	if size := q.Get("page_size"); size != "" {
		if s, err := strconv.Atoi(size); err == nil {
			sorting.PageSize = s
		}
	}

	searchReq := domain.SearchRequest{
		Filters: filters,
		Sorting: sorting,
	}

	events, nextToken, err := h.service.ListEvents(r.Context(), searchReq)
	if err != nil {
		h.respondError(w, err)
		return
	}

	resp := domain.APIResponse{Data: events}
	if nextToken != "" {
		resp.Meta = &domain.Meta{NextPageToken: nextToken}
	}

	json.NewEncoder(w).Encode(resp)
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
