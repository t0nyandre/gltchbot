package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorResponse represents a standardized API error response.
type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

// ErrorDetails contains detailed error information.
type ErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes a JSON response with the given status code and data.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// OK writes a 200 OK JSON response.
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created writes a 201 Created JSON response.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// statusToDefaultCode maps HTTP status codes to default error codes.
func statusToDefaultCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation_error"
	case http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case http.StatusInternalServerError:
		return "internal_server_error"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "unknown_error"
	}
}

// Error writes a JSON error response with a default error code derived from the status.
func Error(w http.ResponseWriter, message string, status int) {
	code := statusToDefaultCode(status)
	ErrorWithCode(w, code, message, status)
}

// ErrorWithCode writes a JSON error response with a custom error code.
func ErrorWithCode(w http.ResponseWriter, code, message string, status int) {
	resp := ErrorResponse{
		Error: ErrorDetails{
			Code:    code,
			Message: message,
		},
	}
	JSON(w, status, resp)
}

// BadRequest writes a 400 Bad Request error response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusBadRequest)
}

// Unauthorized writes a 401 Unauthorized error response.
func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusUnauthorized)
}

// Forbidden writes a 403 Forbidden error response.
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusForbidden)
}

// NotFound writes a 404 Not Found error response.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusNotFound)
}

// Conflict writes a 409 Conflict error response.
func Conflict(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusConflict)
}

// UnprocessableEntity writes a 422 Unprocessable Entity error response.
func UnprocessableEntity(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusUnprocessableEntity)
}

// TooManyRequests writes a 429 Too Many Requests error response.
func TooManyRequests(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusTooManyRequests)
}

// InternalServerError writes a 500 Internal Server Error response.
func InternalServerError(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusInternalServerError)
}

// ServiceUnavailable writes a 503 Service Unavailable error response.
func ServiceUnavailable(w http.ResponseWriter, message string) {
	Error(w, message, http.StatusServiceUnavailable)
}

// ValidationError writes a 400 Bad Request error response for validation failures.
// This is a convenience wrapper that uses "validation_error" code.
func ValidationError(w http.ResponseWriter, message string) {
	ErrorWithCode(w, "validation_error", message, http.StatusBadRequest)
}

// FieldValidationError represents a field-specific validation error.
type FieldValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// FieldValidationErrors writes a 422 Unprocessable Entity error response with field details.
func FieldValidationErrors(w http.ResponseWriter, fieldErrors []FieldValidationError) {
	resp := struct {
		Error struct {
			Code    string                 `json:"code"`
			Message string                 `json:"message"`
			Fields  []FieldValidationError `json:"fields,omitempty"`
		} `json:"error"`
	}{
		Error: struct {
			Code    string                 `json:"code"`
			Message string                 `json:"message"`
			Fields  []FieldValidationError `json:"fields,omitempty"`
		}{
			Code:    "validation_error",
			Message: "validation failed",
			Fields:  fieldErrors,
		},
	}
	JSON(w, http.StatusUnprocessableEntity, resp)
}

// WriteError writes an error response based on the error type.
// This is a convenience function that attempts to infer the appropriate status code.
func WriteError(w http.ResponseWriter, err error) {
	// This is a simple implementation; extend as needed
	InternalServerError(w, err.Error())
}