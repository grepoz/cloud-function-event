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

// Router acts as the main entry point and dispatches to specific controllers
type Router struct {
	eventHandler    *EventHandler
	trackingHandler *TrackingHandler
}

func NewRouter(eventSvc service.EventService, trackingSvc service.TrackingService) *Router {
	return &Router{
		eventHandler:    &EventHandler{service: eventSvc},
		trackingHandler: &TrackingHandler{service: trackingSvc},
	}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route: /events
	if strings.HasPrefix(path, "/events") {
		// Strip the prefix so the EventHandler sees relative paths (like "/")
		http.StripPrefix("/events", rt.eventHandler).ServeHTTP(w, r)
		return
	}

	// Route: /tracking
	if strings.HasPrefix(path, "/tracking") {
		http.StripPrefix("/tracking", rt.trackingHandler).ServeHTTP(w, r)
		return
	}

	// Default/Fallback (Optional: maybe root / is also events?)
	http.NotFound(w, r)
}

type TrackingHandler struct {
	service service.TrackingService
}

func (h *TrackingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		h.handleCreate(w, r)

	case http.MethodGet:
		h.handleList(w, r)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCreate creates a new event
// @Summary Create Tracking Event
// @Description Create a new tracking event
// @Tags tracking
// @Accept json
// @Produce json
// @Param tracking body domain.TrackingEvent true "Tracking Event Data"
// @Success 201 {object} domain.APIResponse{data=string} "Returns Tracking Event ID"
// @Failure 400 {object} domain.APIResponse{error=string}
// @Router /tracking [post]
func (h *TrackingHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var track domain.TrackingEvent
	if err := json.NewDecoder(r.Body).Decode(&track); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Capture User-Agent automatically if not sent
	if track.UserAgent == "" {
		track.UserAgent = r.UserAgent()
	}

	if err := h.service.TrackEvent(r.Context(), &track); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(domain.APIResponse{Data: track.ID})
	if err != nil {
		return
	}
}

// handleList lists all tracking events
// @Summary List Tracking Events
// @Description Get a list of all tracking events
// @Tags tracking
// @Accept json
// @Produce json
// @Success 200 {object} domain.APIResponse{data=[]domain.TrackingEvent}
// @Router /tracking [get]
func (h *TrackingHandler) handleList(w http.ResponseWriter, r *http.Request) {
	tracks, err := h.service.GetAllTracking(r.Context())
	if err != nil {
		respondError(w, err)
		return
	}
	err = json.NewEncoder(w).Encode(domain.APIResponse{Data: tracks})
	if err != nil {
		return
	}
}

type EventHandler struct {
	service service.EventService
}

// EntryPoint handles the routing based on Method and Query Params
func (h *EventHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		h.handleCreate(w, r)
	case http.MethodPut:
		h.handleUpdate(w, r)
	case http.MethodGet:
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

// handleCreate creates a new event
// @Summary Create Event
// @Description Create a new event item
// @Tags events
// @Accept json
// @Produce json
// @Param event body domain.Event true "Event Data"
// @Success 201 {object} domain.APIResponse{data=string} "Returns Event ID"
// @Failure 400 {object} domain.APIResponse{error=string}
// @Router /events [post]
func (h *EventHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var event domain.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := h.service.CreateEvent(r.Context(), &event); err != nil {
		respondError(w, err) // Use helper function
		return
	}
	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(domain.APIResponse{Data: event.ID})
	if err != nil {
		return
	}
}

// handleUpdate updates an existing event
// @Summary Update Event
// @Description Update specific fields of an event
// @Tags events
// @Accept json
// @Produce json
// @Param id query string false "Event ID (can also be in body)"
// @Param event body map[string]interface{} true "Fields to update"
// @Success 200 {object} domain.APIResponse{data=string}
// @Failure 400 {object} domain.APIResponse{error=string}
// @Failure 500 {object} domain.APIResponse{error=string}
// @Router /events [put]
func (h *EventHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
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
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(domain.APIResponse{Data: "Updated successfully"})
	if err != nil {
		return
	}
}

// handleList lists events with filtering and sorting
// @Summary List Events
// @Description Get a list of events with optional filters
// @Tags events
// @Accept json
// @Produce json
// @Param city query string false "Filter by City"
// @Param type query string false "Filter by Type (e.g. concert)"
// @Param min_price query number false "Minimum Price"
// @Param max_price query number false "Maximum Price"
// @Param start_date query string false "Start Date (RFC3339)"
// @Param page_size query int false "Page Size"
// @Param page_token query string false "Pagination Token"
// @Success 200 {object} domain.APIResponse{data=[]domain.Event}
// @Router /events [get]
func (h *EventHandler) handleList(w http.ResponseWriter, r *http.Request) {
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
		respondError(w, err)
		return
	}

	resp := domain.APIResponse{Data: events}
	if nextToken != "" {
		resp.Meta = &domain.Meta{NextPageToken: nextToken}
	}

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		return
	}
}

// handleGet retrieves a single event
// @Summary Get Event
// @Description Get details of a specific event by ID
// @Tags events
// @Accept json
// @Produce json
// @Param id query string true "Event ID"
// @Success 200 {object} domain.APIResponse{data=domain.Event}
// @Failure 400 {object} domain.APIResponse{error=string}
// @Failure 404 {object} domain.APIResponse{error=string}
// @Router /events [get]
func (h *EventHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		respondError(w, err)
		return
	}

	err = json.NewEncoder(w).Encode(domain.APIResponse{Data: event})
	if err != nil {
		return
	}
}

// handleDelete deletes an event
// @Summary Delete Event
// @Description Remove an event by ID
// @Tags events
// @Produce json
// @Param id query string true "Event ID"
// @Success 200 {object} domain.APIResponse{data=string}
// @Failure 400 {object} domain.APIResponse{error=string}
// @Router /events [delete]
func (h *EventHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteEvent(r.Context(), id); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(domain.APIResponse{Data: "Deleted successfully"})
	if err != nil {
		return
	}
}

func respondError(w http.ResponseWriter, err error) {
	if _, ok := err.(*domain.ValidationError); ok {
		w.WriteHeader(http.StatusBadRequest)
	} else if err.Error() == "event not found" {
		w.WriteHeader(http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	err = json.NewEncoder(w).Encode(domain.APIResponse{Error: err.Error()})
	if err != nil {
		return
	}
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
			err := br.Close()
			if err != nil {
				http.Error(w, "Failed to close writer", http.StatusInternalServerError)
			}
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
