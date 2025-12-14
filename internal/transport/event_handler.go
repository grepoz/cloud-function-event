package transport

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type EventHandler struct {
	service service.EventService
	mux     *http.ServeMux
}

func NewEventHandler(svc service.EventService) *EventHandler {
	h := &EventHandler{
		service: svc,
		mux:     http.NewServeMux(),
	}
	h.routes()
	return h
}

func (h *EventHandler) routes() {
	// Collection routes (matched at root of stripped prefix)
	h.mux.HandleFunc("GET /{$}", h.handleList)
	h.mux.HandleFunc("POST /{$}", h.handleCreate)
	h.mux.HandleFunc("POST /batch", h.handleBatchCreate)

	// Item routes (matched with path value)
	h.mux.HandleFunc("GET /{id}", h.handleGet)
	h.mux.HandleFunc("PUT /{id}", h.handleUpdate)
	h.mux.HandleFunc("DELETE /{id}", h.handleDelete)
}

func (h *EventHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	h.mux.ServeHTTP(w, r)
}

// handleCreate creates a new event
// @Summary Create Event
// @Description Create a new event item
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param event body domain.EventDTO true "Event Data"
// @Success 201 {object} domain.APIResponse{data=string} "Returns Event Id"
// @Failure 400 {object} domain.APIResponse{error=string}
// @Router /events [post]
func (h *EventHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var eventDTO domain.EventDTO
	if err := json.NewDecoder(r.Body).Decode(&eventDTO); err != nil {
		respondError(w, domain.ErrValidation("Invalid JSON body"))
		return
	}
	if err := domain.Validate.Struct(eventDTO); err != nil {
		respondError(w, domain.ErrValidation(err.Error()))
		return
	}
	event, err := domain.EventDTOToModel(&eventDTO)

	if err != nil {
		respondError(w, domain.ErrValidation(err.Error()))
		return
	}
	if err := h.service.CreateEvent(r.Context(), event); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Data: event.Id})
}

// handleBatchCreate creates multiple events
// @Summary Batch Create Events
// @Description Create multiple events in one go
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batch body domain.BatchEventRequest true "Batch Data"
// @Success 201 {object} domain.APIResponse{data=string}
// @Failure 400 {object} domain.APIResponse{error=string}
// @Router /events/batch [post]
func (h *EventHandler) handleBatchCreate(w http.ResponseWriter, r *http.Request) {
	var req domain.BatchEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, domain.ErrValidation("Invalid JSON body"))
		return
	}

	if err := domain.Validate.Struct(req); err != nil {
		respondError(w, domain.ErrValidation(err.Error()))
		return
	}

	var events []*domain.Event
	for i, dto := range req.Events {
		model, err := domain.EventDTOToModel(&dto)
		if err != nil {
			respondError(w, domain.ErrValidation(fmt.Sprintf("Item %d: %v", i, err)))
			return
		}
		events = append(events, model)
	}

	if err := h.service.BatchCreateEvents(r.Context(), events); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Data: fmt.Sprintf("Successfully created %d events", len(events))})
}

// handleUpdate updates an existing event
// @Summary Update Event
// @Description Update specific fields of an event
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Event Id"
// @Param event body map[string]interface{} true "Fields to update"
// @Success 200 {object} domain.APIResponse{data=string}
// @Failure 400 {object} domain.APIResponse{error=string}
// @Failure 500 {object} domain.APIResponse{error=string}
// @Router /events/{id} [put]
func (h *EventHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, domain.ErrValidation("Missing id path parameter"))
		return
	}
	var updates map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&updates); err != nil {
		respondError(w, domain.ErrValidation("Invalid JSON body"))
		return
	}
	for k, v := range updates {
		if num, ok := v.(json.Number); ok {
			if f, err := num.Float64(); err == nil {
				updates[k] = f
			}
		}
	}
	if err := h.service.UpdateEvent(r.Context(), id, updates); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Data: "Updated successfully"})
}

// handleList lists events with strict validation and filtering
// @Summary List Events
// @Description Get a list of events with optional filters
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param event_name query string false "Filter by Event Name"
// @Param city query string false "Filter by City"
// @Param type query domain.EventType false "Filter by Type"
// @Param min_price query number false "Minimum Price"
// @Param max_price query number false "Maximum Price"
// @Param start_date query string false "Start Date (RFC3339)"
// @Param end_date query string false "End Date (RFC3339)"
// @Param page_size query int false "Page Size (1-100)"
// @Param page_token query string false "Pagination Token"
// @Param sort_key query string false "Sort Key (e.g. price, start_time)"
// @Param sort_dir query string false "Sort Direction (asc, desc)"
// @Success 200 {object} domain.APIResponse{data=[]domain.Event}
// @Router /events [get]
func (h *EventHandler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// 1. Bind Query Params to DTO
	// We map strings directly and parse numbers manually to catch type errors early.
	dto := domain.EventListDTO{
		PageToken: q.Get("page_token"),
		SortDir:   q.Get("sort_dir"),
		SortKey:   q.Get("sort_key"),
		StartDate: q.Get("start_date"),
		EndDate:   q.Get("end_date"),
		City:      q.Get("city"),
		EventName: q.Get("event_name"),
		Type:      q.Get("type"),
	}

	// Safe Parsing: PageSize
	if val := q.Get("page_size"); val != "" {
		i, err := strconv.Atoi(val)
		if err != nil {
			respondError(w, domain.ErrValidation("page_size must be a valid integer"))
			return
		}
		dto.PageSize = i
	} else {
		dto.PageSize = 20 // Default value
	}

	// Safe Parsing: MinPrice
	if val := q.Get("min_price"); val != "" {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			respondError(w, domain.ErrValidation("min_price must be a valid number"))
			return
		}
		dto.MinPrice = &f
	}

	// Safe Parsing: MaxPrice
	if val := q.Get("max_price"); val != "" {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			respondError(w, domain.ErrValidation("max_price must be a valid number"))
			return
		}
		dto.MaxPrice = &f
	}

	// 2. Struct Validation (Check constraints like gte=0, oneof, etc.)
	if err := domain.Validate.Struct(dto); err != nil {
		respondError(w, domain.ErrValidation(err.Error()))
		return
	}

	// 3. Logical Cross-Field Validation
	if dto.MinPrice != nil && dto.MaxPrice != nil && *dto.MinPrice > *dto.MaxPrice {
		respondError(w, domain.ErrValidation("min_price cannot be greater than max_price"))
		return
	}

	// 4. Convert DTO to Domain Request
	// Time parsing is safe here because validation ensured the format is correct.
	var startTime, endTime *time.Time

	if dto.StartDate != "" {
		t, _ := time.Parse(time.RFC3339, dto.StartDate)
		startTime = &t
	}
	if dto.EndDate != "" {
		t, _ := time.Parse(time.RFC3339, dto.EndDate)
		endTime = &t
	}

	if startTime != nil && endTime != nil && endTime.Before(*startTime) {
		respondError(w, domain.ErrValidation("end_date cannot be before start_date"))
		return
	}

	// Set defaults for Sorting if empty (though logic is also in Repo, it's good to be explicit)
	if dto.SortKey == "" {
		dto.SortKey = "created_at"
	}
	if dto.SortDir == "" {
		dto.SortDir = "asc"
	}

	searchReq := domain.SearchRequest{
		Filters: domain.FilterRequest{
			City:      dto.City,
			EventName: dto.EventName,
			Type:      domain.EventType(dto.Type), // Safe cast due to validation
			MinPrice:  dto.MinPrice,
			MaxPrice:  dto.MaxPrice,
			StartDate: startTime,
			EndDate:   endTime,
		},
		Sorting: domain.SortRequest{
			PageSize:      dto.PageSize,
			PageToken:     dto.PageToken,
			SortKey:       dto.SortKey,
			SortDirection: dto.SortDir,
		},
	}

	// 5. Call Service
	events, nextToken, err := h.service.ListEvents(r.Context(), searchReq)
	if err != nil {
		respondError(w, err)
		return
	}

	// 6. Response
	resp := domain.APIPaginationResponse{
		Data: events,
		Meta: &domain.Meta{
			NextPageToken: nextToken,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGet retrieves a single event
// @Summary Get Event
// @Description Get details of a specific event by Id
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Event Id"
// @Success 200 {object} domain.APIResponse{data=domain.Event}
// @Failure 400 {object} domain.APIResponse{error=string}
// @Failure 404 {object} domain.APIResponse{error=string}
// @Router /events/{id} [get]
func (h *EventHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, domain.ErrValidation("Missing id path parameter"))
		return
	}

	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		respondError(w, err)
		return
	}

	_ = json.NewEncoder(w).Encode(domain.APIResponse{Data: event})
}

// handleDelete deletes an event
// @Summary Delete Event
// @Description Remove an event by Id
// @Tags events
// @Produce json
// @Security BearerAuth
// @Param id path string true "Event Id"
// @Success 200 {object} domain.APIResponse{data=string}
// @Failure 400 {object} domain.APIResponse{error=string}
// @Router /events/{id} [delete]
func (h *EventHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, domain.ErrValidation("Missing id path parameter"))
		return
	}

	if err := h.service.DeleteEvent(r.Context(), id); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Data: "Deleted successfully"})
}
