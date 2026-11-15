package utils

import (
	"encoding/json"
	"net/http"
)

func JSONResponse(
	w http.ResponseWriter,
	status int,
	data interface{},
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		return
	}
}

func ErrorResponse(
	w http.ResponseWriter,
	status int,
	message string,
) {
	JSONResponse(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
