package transport

import (
	"cloud-function-event/internal/domain"
	"cloud-function-event/internal/service"
	"encoding/json"
	"net/http"
)

type TrackingHandler struct {
	service service.TrackingService
	mux     *http.ServeMux
}

func NewTrackingHandler(svc service.TrackingService) *TrackingHandler {
	h := &TrackingHandler{
		service: svc,
		mux:     http.NewServeMux(),
	}
	h.routes()
	return h
}

func (h *TrackingHandler) routes() {
	// GET /tracking/ (List)
	h.mux.HandleFunc("GET /{$}", h.handleList)
	// POST /tracking/ (Create)
	h.mux.HandleFunc("POST /{$}", h.handleCreate)
}

func (h *TrackingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	h.mux.ServeHTTP(w, r)
}

// handleCreate creates a new tracking event
// @Summary Create Tracking Event
// @Description Create a new tracking event
// @Tags tracking
// @Accept json
// @Produce json
// @Param tracking body domain.TrackingEvent true "Tracking Event Data"
// @Success 201 {object} domain.APIResponse{data=string} "Returns Tracking Event Id"
// @Failure 400 {object} domain.APIResponse{error=string}
// @Router /tracking [post]
func (h *TrackingHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var dto domain.TrackingEventDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, domain.ErrValidation("Invalid JSON"))
		return
	}
	if err := domain.Validate.Struct(dto); err != nil {
		respondError(w, domain.ErrValidation(err.Error()))
		return
	}
	trackingEvent := domain.TrackingEvent{
		Action:    dto.Action,
		Payload:   dto.Payload,
		UserAgent: dto.UserAgent,
		UserName:  dto.UserName,
	}
	if trackingEvent.UserAgent == "" {
		trackingEvent.UserAgent = r.UserAgent()
	}
	if err := h.service.TrackEvent(r.Context(), &trackingEvent); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Data: trackingEvent.Id})
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
	_ = json.NewEncoder(w).Encode(domain.APIResponse{Data: tracks})
}
