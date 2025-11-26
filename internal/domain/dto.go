package domain

import "github.com/go-playground/validator/v10"

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
	StartTime string  `json:"start_time" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	EndTime   string  `json:"end_time" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	// Add other fields as needed, with appropriate validation tags
	// OrganizerName, Country, etc.
}
