package middleware

import "fmt"

var (
	ErrInvalidDomain = fmt.Errorf("invalid domain")
	ErrNoTenant      = fmt.Errorf("no tenant found")
	ErrFetchTenant   = fmt.Errorf("failed to fetch tenant")
	ErrInvalidInput  = fmt.Errorf("invalid input")
)

// Wrap for context
func WrapErr(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
