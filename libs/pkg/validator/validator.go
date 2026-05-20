// Package validator provides input validation helpers.
package validator

import "strings"

// Required checks if a string value is present.
func Required(value string) bool {
	return strings.TrimSpace(value) != ""
}
