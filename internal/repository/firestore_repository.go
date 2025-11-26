package repository

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// Shared cursorData and encode/decodeCursor utilities

type cursorData struct {
	SortValue interface{} `json:"v"`
	ID        string      `json:"id"`
}

func encodeCursor(sortVal interface{}, id string) string {
	data := cursorData{SortValue: sortVal, ID: id}
	b, _ := json.Marshal(data)
	return base64.StdEncoding.EncodeToString(b)
}

func decodeCursor(token string) ([]interface{}, error) {
	b, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var data cursorData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}

	// Handle time unmarshalling (JSON numbers/strings -> time.Time)
	if s, ok := data.SortValue.(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return []interface{}{t, data.ID}, nil
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return []interface{}{t, data.ID}, nil
		}
	}

	return []interface{}{data.SortValue, data.ID}, nil
}
