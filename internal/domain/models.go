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
	Type          string    `json:"type" firestore:"type"` // Added 'Type' as requested in filter props
	CreatedAt     time.Time `json:"created_at" firestore:"created_at"`
}

// FilterRequest encapsulates filtering parameters
type FilterRequest struct {
	City      string     `json:"city"`
	StartDate *time.Time `json:"start_date"` // Pointer to allow nil check
	EndDate   *time.Time `json:"end_date"`
	MinPrice  *float64   `json:"min_price"`
	MaxPrice  *float64   `json:"max_price"`
	Type      string     `json:"type"`
}

// SortRequest encapsulates sorting and pagination
type SortRequest struct {
	SortKey       string `json:"sort_key"`       // e.g., "start_time", "price"
	SortDirection string `json:"sort_direction"` // "asc" or "desc"
	PageSize      int    `json:"page_size"`
	PageNumber    int    `json:"page_number"`
}

// SearchRequest is the composite request object for listing events
type SearchRequest struct {
	Filters FilterRequest `json:"filters"`
	Sorting SortRequest   `json:"sorting"`
}

// APIResponse is a standard wrapper for responses
type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Meta  interface{} `json:"meta,omitempty"`
}
