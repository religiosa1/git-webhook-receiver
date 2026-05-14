package serialization

import (
	"database/sql"
	"encoding/json"
)

// NullString is a nullable string presentation
type NullString struct {
	sql.NullString
}

func (ns NullString) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(ns.String)
	}
	return []byte("null"), nil
}

// NullTS is a nullable timestamp presentation (as ISO8601/RFC3339)
type NullTS struct {
	sql.NullTime
}

func (ns NullTS) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(ns.Time)
	}
	return []byte("null"), nil
}

// JSONData is JSON data dump presentation (as JSON)
type JSONData struct {
	data map[string]any
}

func (d JSONData) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.data)
}

func NewJSONData(d json.RawMessage) (JSONData, error) {
	var result map[string]any

	err := json.Unmarshal(d, &result)
	if err != nil {
		return JSONData{}, err
	}
	return JSONData{result}, nil
}
