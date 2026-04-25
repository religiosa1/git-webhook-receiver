package utils

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error"`
}

func WriteErrorResponse(w http.ResponseWriter, statusCode int, err string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(errorResponse{
		StatusCode: statusCode,
		Error:      err,
	})
}
