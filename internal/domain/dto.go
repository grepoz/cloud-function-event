package domain

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

var Validate = validator.New()

func init() {
	// Custom validations can be re-added here if needed
}

// AddressDTO for validation
type AddressDTO struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	Street           string `json:"street"`
	City             string `json:"city" validate:"required"` // City is usually required
	Country          string `json:"country"`
	PostalCode       string `json:"postal_code"`
	RawAddressString string `json:"raw_address_string"`
}

type LocationDTO struct {
	Type    string     `json:"type"`
	Name    string     `json:"name"`
	Address AddressDTO `json:"address" validate:"required"`
}

type OrganizationDTO struct {
	Type    string     `json:"type"`
	Name    string     `json:"name"`
	Url     string     `json:"url"`
	Address AddressDTO `json:"address"`
}

type OfferDTO struct {
	Type        string  `json:"type"`
	Price       float64 `json:"price" validate:"gte=0"`
	Currency    string  `json:"currency"`
	Url         string  `json:"url"`
	IsAvailable bool    `json:"is_available"`
}

// EventDTO matches the input JSON structure
type EventDTO struct {
	Type           string           `json:"type" validate:"required"` // "Event"
	Name           string           `json:"name" validate:"required"`
	Description    string           `json:"description"`
	ArticleBody    string           `json:"article_body"`
	Keywords       []string         `json:"keywords"`
	StartDate      string           `json:"start_date" validate:"required,datetime=2006-01-02T15:04:05Z07:00" example:"2024-07-20T22:00:00Z"`
	EndDate        *string          `json:"end_date" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00" example:"2024-07-20T22:00:00Z"`
	DatePublished  string           `json:"date_published"`
	Url            string           `json:"url"`
	ImageUrl       string           `json:"image_url"`
	EventStatus    string           `json:"event_status"`
	AttendanceMode string           `json:"attendance_mode"`
	Location       LocationDTO      `json:"location" validate:"required"`
	Performer      OrganizationDTO  `json:"performer"`
	Organizer      *OrganizationDTO `json:"organizer"`
	Offer          OfferDTO         `json:"offer"`
	Provider       string           `json:"provider"`
}

// EventListDTO for query parameters
type EventListDTO struct {
	PageSize  int    `validate:"gte=1,lte=100"`
	PageToken string `validate:"omitempty,base64"`
	SortDir   string `validate:"omitempty,oneof=asc desc"`
	SortKey   string `validate:"omitempty,oneof=name location.address.city start_date created_at"`

	MinPrice *float64 `validate:"omitempty,gte=0"`
	MaxPrice *float64 `validate:"omitempty,gte=0"`

	StartDate string `validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	EndDate   string `validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`

	City     string   `validate:"omitempty,max=50"`
	Name     string   `validate:"omitempty,max=100"`
	Type     string   `validate:"omitempty"`
	Keywords []string `validate:"omitempty,max=10,dive,max=30"`
}

type UpdateEventDTO struct {
	Type           string           `json:"type" validate:"required"` // "Event"
	Name           string           `json:"name" validate:"required"`
	Description    string           `json:"description"`
	ArticleBody    string           `json:"article_body"`
	Keywords       []string         `json:"keywords"`
	StartDate      string           `json:"start_date" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	EndDate        *string          `json:"end_date" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	DatePublished  string           `json:"date_published"`
	Url            string           `json:"url"`
	ImageUrl       string           `json:"image_url"`
	EventStatus    string           `json:"event_status"`
	AttendanceMode string           `json:"attendance_mode"`
	Location       LocationDTO      `json:"location" validate:"required"`
	Performer      OrganizationDTO  `json:"performer"`
	Organizer      *OrganizationDTO `json:"organizer"`
	Offer          OfferDTO         `json:"offer"`
	Provider       string           `json:"provider"`
}

type BatchEventRequest struct {
	Events []EventDTO `json:"events" validate:"required,min=1,max=500,dive"`
}

type TrackingEventDTO struct {
	Action    string `json:"action" validate:"required"`
	Payload   string `json:"payload"`
	UserAgent string `json:"user_agent"`
	UserName  string `json:"user_name"`
}

func EventDTOToModel(dto *EventDTO) (*Event, error) {
	startDate, err := time.Parse(time.RFC3339, dto.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date format: %w", err)
	}

	var endDate *time.Time
	if dto.EndDate != nil && *dto.EndDate != "" {
		t, err := time.Parse(time.RFC3339, *dto.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid end_date format: %w", err)
		}
		if t.Before(startDate) {
			return nil, fmt.Errorf("end_date cannot be before start_date")
		}
		endDate = &t
	}

	var pubTime time.Time
	if dto.DatePublished != "" {
		pubTime, _ = time.Parse(time.RFC3339, dto.DatePublished)
	}

	// Map nested structures
	loc := Location{
		Type: dto.Location.Type,
		Name: dto.Location.Name,
		Address: Address{
			Type:             dto.Location.Address.Type,
			Name:             dto.Location.Address.Name,
			Street:           dto.Location.Address.Street,
			City:             dto.Location.Address.City,
			Country:          dto.Location.Address.Country,
			PostalCode:       dto.Location.Address.PostalCode,
			RawAddressString: dto.Location.Address.RawAddressString,
		},
	}

	// Map Offer
	offer := Offer{
		Type:        dto.Offer.Type,
		Price:       dto.Offer.Price,
		Currency:    dto.Offer.Currency,
		Url:         dto.Offer.Url,
		IsAvailable: dto.Offer.IsAvailable,
	}

	return &Event{
		Type:           dto.Type,
		Name:           dto.Name,
		Description:    dto.Description,
		ArticleBody:    dto.ArticleBody,
		Keywords:       dto.Keywords,
		StartDate:      startDate,
		EndDate:        endDate,
		DatePublished:  pubTime,
		Url:            dto.Url,
		ImageUrl:       dto.ImageUrl,
		EventStatus:    dto.EventStatus,
		AttendanceMode: dto.AttendanceMode,
		Location:       loc,
		Offer:          offer,
		Provider:       dto.Provider,
	}, nil
}
