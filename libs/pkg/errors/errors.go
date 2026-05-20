package errors

import "fmt"

// Wrap annotates an existing error with contextual information.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", message, err)
}

// New creates a new error with the provided message.
func New(message string) error {
	return fmt.Errorf("%s", message)
}
