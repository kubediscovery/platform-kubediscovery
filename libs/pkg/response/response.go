// Package response defines common API response payload types.
package response

// ErrorResponse is a minimal API error payload.
type ErrorResponse struct {
	Message string `json:"message"`
}

// SuccessResponse is a minimal API success payload.
type SuccessResponse struct {
	Message string `json:"message"`
}
