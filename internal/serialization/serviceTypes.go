package serialization

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Nullable string presentation
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

// Nullable timestamp presentation (as ISO8601/RFC3339)
type NullTs struct {
	sql.NullInt64
}

func (ns NullTs) MarshalJSON() ([]byte, error) {
	if ns.Valid {
		return json.Marshal(Timestamp{ns.Int64})
	}
	return []byte("null"), nil
}

// JSON data dump presentaion (as JSON)
type JsonData struct {
	data map[string]interface{}
}

func (d JsonData) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.data)
}

func NewJsonData(d json.RawMessage) (JsonData, error) {
	var result map[string]interface{}

	err := json.Unmarshal(d, &result)
	if err != nil {
		return JsonData{}, err
	}
	return JsonData{result}, nil
}
