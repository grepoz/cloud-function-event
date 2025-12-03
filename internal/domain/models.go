package domain

import (
	"time"
)

// Event represents the database entity and the DTO
type Event struct {
	ID            string    `firestore:"id"`
	OrganizerName string    `firestore:"organizer_name"`
	EventName     string    `firestore:"eventname"`
	HasTickets    bool      `firestore:"has_tickets"`
	City          string    `firestore:"city"`
	Country       string    `firestore:"country"`
	FullAddress   string    `firestore:"full_address"`
	Latitude      string    `firestore:"latitude"`
	Longitude     string    `firestore:"longitude"`
	State         string    `firestore:"state"`
	Street        string    `firestore:"street"`
	StartTime     time.Time `firestore:"start_time"`
	EndTime       time.Time `firestore:"end_time"`
	Timezone      string    `firestore:"timezone"`
	EventURL      string    `firestore:"event_url"`
	Provider      string    `firestore:"provider"`
	Price         float64   `firestore:"price"`
	ImageURL      string    `firestore:"image_url"`
	Type          string    `firestore:"type"`
	CreatedAt     time.Time `firestore:"created_at"`
}

// TrackingEvent represents an analytics or tracking action
type TrackingEvent struct {
	ID        string    `firestore:"id"`
	Action    string    `firestore:"action"`
	Payload   string    `firestore:"payload"`
	UserAgent string    `firestore:"user_agent"`
	CreatedAt time.Time `firestore:"created_at"`
}

// SearchRequest - helper structure for filters
type SearchRequest struct {
	Filters FilterRequest
	Sorting SortRequest
}

type FilterRequest struct {
	City      string
	EventName string
	StartDate *time.Time
	EndDate   *time.Time
	MinPrice  *float64
	MaxPrice  *float64
	Type      string
}

type SortRequest struct {
	SortKey       string
	SortDirection string
	PageSize      int
	PageToken     string
}

type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

type Meta struct {
	NextPageToken string `json:"nextPageToken,omitempty"`
}

type APIPaginationResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
}
