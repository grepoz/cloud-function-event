package domain

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

var Validate = validator.New()

func init() {
	// Register a custom validation tag named "event_type"
	err := Validate.RegisterValidation("event_type", func(fl validator.FieldLevel) bool {
		// Convert the field value to your custom type and check validity
		return EventType(fl.Field().String()).IsValid()
	})
	if err != nil {
		return
	}
}

// TrackingEventDTO is used for API input/output
// Add validation tags for required fields
// You can extend this DTO as needed
// Example: Action is required, Payload is optional

type TrackingEventDTO struct {
	Action    string `json:"action" validate:"required"`
	Payload   string `json:"payload"`
	UserAgent string `json:"user_agent"`
	UserName  string `json:"user_name"`
}

// EventDTO is used for API input/output for events
// Add validation tags for required fields
// Example: EventName and City are required

type EventDTO struct {
	EventName string    `json:"event_name" validate:"required"`
	City      string    `json:"city" validate:"required"`
	Type      EventType `json:"type" validate:"required,event_type" example:"concert"`
	Price     float64   `json:"price" validate:"gte=0"`
	StartTime string    `json:"start_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00" example:"2024-07-20T22:00:00Z"`
	EndTime   string    `json:"end_time" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00" example:"2024-07-20T22:00:00Z"`
	// Add other fields as needed, with appropriate validation tags
	// OrganizerName, Country, etc.
}

type EventListDTO struct {
	// Pagination & Sorting
	PageSize  int    `validate:"gte=1,lte=100"`                                               // Hard limit: 1-100
	PageToken string `validate:"omitempty,base64"`                                            // Must be valid base64
	SortDir   string `validate:"omitempty,oneof=asc desc"`                                    // Only "asc" or "desc"
	SortKey   string `validate:"omitempty,oneof=event_name city price start_time created_at"` // Whitelist allowed columns

	// Filters - Numeric
	MinPrice *float64 `validate:"omitempty,gte=0"` // Pointer allows distinguishing "0" from "not present"
	MaxPrice *float64 `validate:"omitempty,gte=0"`

	// Filters - Date (ISO8601/RFC3339)
	StartDate string `validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	EndDate   string `validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`

	// Filters - Text
	City      string `validate:"omitempty,max=50,printascii"` // Prevent huge strings or weird chars
	EventName string `validate:"omitempty,max=100"`
	Type      string `validate:"omitempty,oneof=concert festival theater standup conference meetup other"`
}

type BatchEventRequest struct {
	Events []EventDTO `json:"events" validate:"required,min=1,dive"`
}

func EventDTOToModel(dto *EventDTO) (*Event, error) {
	startTime, err := time.Parse(time.RFC3339, dto.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start_time format: %w", err)
	}

	var endTime time.Time
	if dto.EndTime != "" {
		endTime, err = time.Parse(time.RFC3339, dto.EndTime)
		if err != nil {
			return nil, fmt.Errorf("invalid end_time format: %w", err)
		}
		// Logical check: End before Start
		if endTime.Before(startTime) {
			return nil, fmt.Errorf("end_time cannot be before start_time")
		}
	}

	return &Event{
		EventName: dto.EventName,
		City:      dto.City,
		Type:      dto.Type,
		Price:     dto.Price,
		StartTime: startTime,
		EndTime:   endTime,
		// Map other fields if necessary
	}, nil
}
