package domain

import (
	"time"
)

// Event represents the database entity and the DTO
type Event struct {
	ID            string    `json:"id" firestore:"id"`
	OrganizerName string    `json:"organizer_name" firestore:"organizer_name"`
	EventName     string    `json:"eventname" firestore:"eventname"`
	HasTickets    bool      `json:"has_tickets" firestore:"has_tickets"`
	City          string    `json:"city" firestore:"city"`
	Country       string    `json:"country" firestore:"country"`
	FullAddress   string    `json:"full_address" firestore:"full_address"`
	Latitude      string    `json:"latitude" firestore:"latitude"`
	Longitude     string    `json:"longitude" firestore:"longitude"`
	State         string    `json:"state" firestore:"state"`
	Street        string    `json:"street" firestore:"street"`
	StartTime     time.Time `json:"start_time" firestore:"start_time"`
	EndTime       time.Time `json:"end_time" firestore:"end_time"`
	Timezone      string    `json:"timezone" firestore:"timezone"`
	EventURL      string    `json:"event_url" firestore:"event_url"`
	Provider      string    `json:"provider" firestore:"provider"`
	Price         float64   `json:"price" firestore:"price"`
	ImageURL      string    `json:"image_url" firestore:"image_url"`
	Type          string    `json:"type" firestore:"type"`
	CreatedAt     time.Time `json:"created_at" firestore:"created_at"`
}

// TrackingEvent represents an analytics or tracking action [NEW]
type TrackingEvent struct {
	ID        string    `json:"id" firestore:"id"`
	Action    string    `json:"action" firestore:"action"`
	Payload   string    `json:"payload" firestore:"payload"`
	UserAgent string    `json:"user_agent" firestore:"user_agent"`
	CreatedAt time.Time `json:"created_at" firestore:"created_at"`
}

// SearchRequest - helper structure for filters
type SearchRequest struct {
	Filters FilterRequest
	Sorting SortRequest
}

type FilterRequest struct {
	City      string
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

type Meta struct {
	NextPageToken string `json:"next_page_token,omitempty"`
}

type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
}
