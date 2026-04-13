package serialization

import (
	"database/sql"
	"encoding/json"
	"time"
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

// Timestamp presentation (as ISO8601/RFC3339)
type Timestamp struct {
	int64
}

func (ts Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Unix(ts.int64, 0).Format(time.RFC3339))
}

// NullTS is a nullable timestamp presentation (as ISO8601/RFC3339)
type NullTS struct {
	sql.NullInt64
}

func (ns NullTS) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(Timestamp{ns.Int64})
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
	var result map[string]interface{}

	err := json.Unmarshal(d, &result)
	if err != nil {
		return JSONData{}, err
	}
	return JSONData{result}, nil
}
