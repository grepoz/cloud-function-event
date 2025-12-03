package domain

import (
	"time"

	"github.com/go-playground/validator/v10"
)

var Validate = validator.New()

// TrackingEventDTO is used for API input/output
// Add validation tags for required fields
// You can extend this DTO as needed
// Example: Action is required, Payload is optional

type TrackingEventDTO struct {
	Action    string `json:"action" validate:"required"`
	Payload   string `json:"payload"`
	UserAgent string `json:"user_agent"`
}

// EventDTO is used for API input/output for events
// Add validation tags for required fields
// Example: EventName and City are required

type EventDTO struct {
	EventName string  `json:"eventname" validate:"required"`
	City      string  `json:"city" validate:"required"`
	Type      string  `json:"type" validate:"required"`
	Price     float64 `json:"price" validate:"gte=0"`
	StartTime string  `json:"start_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00" example:"2024-07-20T22:00:00Z"`
	EndTime   string  `json:"end_time" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00" example:"2024-07-20T22:00:00Z"`
	// Add other fields as needed, with appropriate validation tags
	// OrganizerName, Country, etc.
}

func EventDTOToModel(dto *EventDTO) *Event {
	// Parse time strings to time.Time
	// Note: We ignore errors here because the validator should have ensured format is correct
	startTime, _ := time.Parse(time.RFC3339, dto.StartTime)
	var endTime time.Time
	if dto.EndTime != "" {
		endTime, _ = time.Parse(time.RFC3339, dto.EndTime)
	}

	return &Event{
		EventName: dto.EventName,
		City:      dto.City,
		Type:      dto.Type,
		Price:     dto.Price,
		StartTime: startTime,
		EndTime:   endTime,
		// Map other fields if necessary
	}
}
