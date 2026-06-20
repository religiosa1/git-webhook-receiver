package serialization

import (
	"encoding/json"
)

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
