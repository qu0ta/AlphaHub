package dsclient

import "fmt"

type DSError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *DSError) Error() string {
	return fmt.Sprintf("DS HTTP %d: %s: %s",
		e.StatusCode, e.Code, e.Message)
}
