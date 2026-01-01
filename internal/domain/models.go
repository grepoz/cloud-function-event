package domain

import (
	"time"
)

// Address matches the nested address object in JSON
type Address struct {
	Type             string `firestore:"type" json:"type"`
	Name             string `firestore:"name" json:"name"`
	Street           string `firestore:"street" json:"street"`
	City             string `firestore:"city" json:"city"`
	Country          string `firestore:"country" json:"country"`
	PostalCode       string `firestore:"postal_code" json:"postal_code"`
	RawAddressString string `firestore:"raw_address_string" json:"raw_address_string"`
}

// Location matches the location object
type Location struct {
	Type    string  `firestore:"type" json:"type"`
	Name    string  `firestore:"name" json:"name"`
	Address Address `firestore:"address" json:"address"`
}

// Organization matches performer/organizer
type Organization struct {
	Type    string  `firestore:"type" json:"type"`
	Name    string  `firestore:"name" json:"name"`
	Url     string  `firestore:"url" json:"url"`
	Address Address `firestore:"address" json:"address"`
}

// Offer matches the offer item
type Offer struct {
	Type             string  `firestore:"type" json:"type"`
	Price            float64 `firestore:"price" json:"price"`
	Currency         string  `firestore:"currency" json:"currency"`
	Url              string  `firestore:"url" json:"url"`
	IsAvailable      bool    `firestore:"is_available" json:"is_available"`
	StatusText       string  `firestore:"status_text" json:"status_text"`
	AvailabilityType string  `firestore:"availability_type" json:"availability_type"`
}

// Event structure updated to match events-tricity.json
type Event struct {
	Id             string        `firestore:"id" json:"id"`
	Type           string        `firestore:"type" json:"type"` // "Event"
	Name           string        `firestore:"name" json:"name"`
	Description    string        `firestore:"description" json:"description"`
	ArticleBody    string        `firestore:"article_body" json:"article_body"`
	Keywords       []string      `firestore:"keywords" json:"keywords"`
	StartDate      time.Time     `firestore:"start_date" json:"start_date"`
	EndDate        *time.Time    `firestore:"end_date" json:"end_date"`
	DatePublished  time.Time     `firestore:"date_published" json:"date_published"`
	Url            string        `firestore:"url" json:"url"`
	ImageUrl       string        `firestore:"image_url" json:"image_url"`
	EventStatus    string        `firestore:"event_status" json:"event_status"`
	AttendanceMode string        `firestore:"attendance_mode" json:"attendance_mode"`
	Location       Location      `firestore:"location" json:"location"`
	Performer      Organization  `firestore:"performer" json:"performer"`
	Organizer      *Organization `firestore:"organizer" json:"organizer"`
	Offer          Offer         `firestore:"offer" json:"offer"`
	Provider       string        `firestore:"provider" json:"provider"`
	CreatedAt      time.Time     `firestore:"created_at" json:"created_at"`
}

// TrackingEvent remains unchanged
type TrackingEvent struct {
	Id        string    `firestore:"id" json:"id"`
	Action    string    `firestore:"action" json:"action"`
	UserName  string    `firestore:"user_name" json:"user_name"`
	Payload   string    `firestore:"payload" json:"payload"`
	UserAgent string    `firestore:"user_agent" json:"user_agent"`
	CreatedAt time.Time `firestore:"created_at" json:"created_at"`
}

// SearchRequest - helper structure for filters
type SearchRequest struct {
	Filters FilterRequest
	Sorting SortRequest
}

type FilterRequest struct {
	City      string
	Name      string
	StartDate *time.Time
	EndDate   *time.Time
	MinPrice  *float64
	MaxPrice  *float64
	Type      string
	Keywords  []string
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
