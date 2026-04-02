package provider

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIErrorCategory classifies Anthropic API errors for retry/fallback logic.
type APIErrorCategory string

const (
	ErrCatRateLimit      APIErrorCategory = "rate_limit"       // 429
	ErrCatOverloaded     APIErrorCategory = "overloaded"       // 529
	ErrCatPromptTooLong  APIErrorCategory = "prompt_too_long"
	ErrCatAuthError      APIErrorCategory = "auth_error"       // 401
	ErrCatBillingError   APIErrorCategory = "billing_error"    // 402
	ErrCatNotFound       APIErrorCategory = "not_found"        // 404
	ErrCatServerError    APIErrorCategory = "server_error"     // 5xx
	ErrCatUnknown        APIErrorCategory = "unknown"
)

// APIError wraps an Anthropic API failure with its category.
type APIError struct {
	StatusCode int
	Category   APIErrorCategory
	Message    string
	// RetryAfterSecs is set when the server provides a Retry-After header.
	RetryAfterSecs int
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("api error %d (%s): %s", e.StatusCode, e.Category, e.Message)
	}
	return fmt.Sprintf("api error (%s): %s", e.Category, e.Message)
}

// IsRetryable reports whether this error category should be retried.
func (e *APIError) IsRetryable() bool {
	switch e.Category {
	case ErrCatRateLimit, ErrCatOverloaded, ErrCatServerError:
		return true
	}
	return false
}

// IsFallbackable reports whether we should try a fallback model.
func (e *APIError) IsFallbackable() bool {
	return e.Category == ErrCatOverloaded || e.Category == ErrCatServerError
}

// CategorizeHTTPError maps an HTTP status code and body to an APIError.
func CategorizeHTTPError(statusCode int, body string, retryAfterSecs int) *APIError {
	cat := categorizeByStatus(statusCode, body)
	return &APIError{
		StatusCode:     statusCode,
		Category:       cat,
		Message:        body,
		RetryAfterSecs: retryAfterSecs,
	}
}

// CategorizeRetryableAPIError classifies an arbitrary error, returning nil
// if the error is not an *APIError or is not retryable.
func CategorizeRetryableAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.IsRetryable() {
		return apiErr
	}
	return nil
}

// IsPromptTooLong reports whether the error indicates the prompt exceeded the
// model's context window.
func IsPromptTooLong(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Category == ErrCatPromptTooLong
	}
	return strings.Contains(strings.ToLower(err.Error()), "prompt is too long")
}

func categorizeByStatus(code int, body string) APIErrorCategory {
	lbody := strings.ToLower(body)
	switch code {
	case http.StatusUnauthorized:
		return ErrCatAuthError
	case http.StatusPaymentRequired:
		return ErrCatBillingError
	case http.StatusNotFound:
		return ErrCatNotFound
	case http.StatusTooManyRequests:
		return ErrCatRateLimit
	case 529:
		return ErrCatOverloaded
	default:
		if code >= 500 {
			if strings.Contains(lbody, "overloaded") {
				return ErrCatOverloaded
			}
			return ErrCatServerError
		}
	}
	if strings.Contains(lbody, "prompt is too long") ||
		strings.Contains(lbody, "context_length_exceeded") ||
		strings.Contains(lbody, "prompt_too_long") {
		return ErrCatPromptTooLong
	}
	return ErrCatUnknown
}
