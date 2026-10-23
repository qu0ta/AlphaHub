package dsclient

import (
	"encoding/json"
	"errors"
	"net/http"
)

// WriteHTTPError translates dependency failures without reporting DS outages as input errors.
func WriteHTTPError(w http.ResponseWriter, err error) bool {
	var remote *DSError
	if errors.As(err, &remote) {
		status := http.StatusBadGateway
		if remote.StatusCode >= 400 && remote.StatusCode < 500 {
			status = remote.StatusCode
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": remote.Code, "message": remote.Message}})
		return true
	}
	if errors.Is(err, ErrDSUnavailable) {
		http.Error(w, "DS service unavailable; retry the request", http.StatusServiceUnavailable)
		return true
	}
	return false
}
