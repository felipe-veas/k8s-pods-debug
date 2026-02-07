package plugin

import (
	"fmt"
	"os"
	"strings"
)

// ErrorType represents different types of errors
type ErrorType string

const (
	ErrorTypePodNotFound   ErrorType = "POD_NOT_FOUND"
	ErrorTypePermission    ErrorType = "PERMISSION_DENIED"
	ErrorTypeNetwork       ErrorType = "NETWORK_ERROR"
	ErrorTypeKubectl       ErrorType = "KUBECTL_ERROR"
	ErrorTypeValidation    ErrorType = "VALIDATION_ERROR"
	ErrorTypeTimeout       ErrorType = "TIMEOUT_ERROR"
	ErrorTypeClusterAccess ErrorType = "CLUSTER_ACCESS_ERROR"
	ErrorTypeResourceLimit ErrorType = "RESOURCE_LIMIT_ERROR"
)

// DetailedError provides structured error information
type DetailedError struct {
	Type        ErrorType
	Message     string
	Suggestion  string
	Command     string
	OriginalErr error
}

func (e *DetailedError) Error() string {
	var sb strings.Builder

	// Error icon and main message
	sb.WriteString("❌ ")
	sb.WriteString(e.Message)
	sb.WriteString("\n")

	// Suggestion if available
	if e.Suggestion != "" {
		sb.WriteString("\n💡 Suggestion: ")
		sb.WriteString(e.Suggestion)
		sb.WriteString("\n")
	}

	// Command if available
	if e.Command != "" {
		sb.WriteString("\n🔧 Try: ")
		sb.WriteString(e.Command)
		sb.WriteString("\n")
	}

	// Original error for debugging
	if e.OriginalErr != nil {
		sb.WriteString("\n🔍 Details: ")
		sb.WriteString(e.OriginalErr.Error())
		sb.WriteString("\n")
	}

	return sb.String()
}

// NewDetailedError creates a new detailed error
func NewDetailedError(errorType ErrorType, message string) *DetailedError {
	return &DetailedError{
		Type:    errorType,
		Message: message,
	}
}

// WithSuggestion adds a suggestion to the error
func (e *DetailedError) WithSuggestion(suggestion string) *DetailedError {
	e.Suggestion = suggestion
	return e
}

// WithCommand adds a command suggestion to the error
func (e *DetailedError) WithCommand(command string) *DetailedError {
	e.Command = command
	return e
}

// WithOriginalError adds the original error for debugging
func (e *DetailedError) WithOriginalError(err error) *DetailedError {
	e.OriginalErr = err
	return e
}

// Common error constructors
func NewPodNotFoundError(podName, namespace string) *DetailedError {
	return NewDetailedError(
		ErrorTypePodNotFound,
		fmt.Sprintf("Pod '%s' not found in namespace '%s'", podName, namespace),
	).WithSuggestion(
		"Check if the pod name is correct and the pod exists",
	).WithCommand(
		fmt.Sprintf("kubectl get pods -n %s", namespace),
	)
}

func NewPermissionError(operation string) *DetailedError {
	return NewDetailedError(
		ErrorTypePermission,
		fmt.Sprintf("Permission denied for operation: %s", operation),
	).WithSuggestion(
		"Check your RBAC permissions or contact your cluster administrator",
	).WithCommand(
		"kubectl auth can-i create pods",
	)
}

func NewClusterAccessError() *DetailedError {
	return NewDetailedError(
		ErrorTypeClusterAccess,
		"Cannot connect to Kubernetes cluster",
	).WithSuggestion(
		"Check your kubeconfig and cluster connectivity",
	).WithCommand(
		"kubectl cluster-info",
	)
}

func NewValidationError(field, value, reason string) *DetailedError {
	return NewDetailedError(
		ErrorTypeValidation,
		fmt.Sprintf("Invalid %s: '%s' - %s", field, value, reason),
	).WithSuggestion(
		"Check the parameter value and try again",
	)
}

func NewTimeoutError(operation string, timeout string) *DetailedError {
	return NewDetailedError(
		ErrorTypeTimeout,
		fmt.Sprintf("Operation '%s' timed out after %s", operation, timeout),
	).WithSuggestion(
		"The operation may take longer than expected. Try increasing the timeout or check cluster resources",
	)
}

// HandleError provides centralized error handling with improved UX
func HandleError(err error) {
	if err == nil {
		return
	}

	// If it's already a DetailedError, print it nicely
	if detailedErr, ok := err.(*DetailedError); ok {
		fmt.Fprint(os.Stderr, detailedErr.Error())
		os.Exit(1)
		return
	}

	// Try to categorize common kubectl errors
	errStr := err.Error()
	var detailedErr *DetailedError

	switch {
	case strings.Contains(errStr, "not found"):
		detailedErr = NewDetailedError(
			ErrorTypePodNotFound,
			"Resource not found",
		).WithOriginalError(err)

	case strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "unauthorized"):
		detailedErr = NewPermissionError("resource access").WithOriginalError(err)

	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host"):
		detailedErr = NewClusterAccessError().WithOriginalError(err)

	case strings.Contains(errStr, "timeout"):
		detailedErr = NewTimeoutError("kubectl operation", "default").WithOriginalError(err)

	default:
		detailedErr = NewDetailedError(
			ErrorTypeKubectl,
			"An unexpected error occurred",
		).WithOriginalError(err).WithSuggestion(
			"Check the error details below and verify your cluster connection",
		)
	}

	fmt.Fprint(os.Stderr, detailedErr.Error())
	os.Exit(1)
}

// WrapKubectlError wraps kubectl command errors with better context
func WrapKubectlError(err error, operation string) *DetailedError {
	if err == nil {
		return nil
	}

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "not found"):
		return NewDetailedError(
			ErrorTypePodNotFound,
			fmt.Sprintf("Resource not found during %s", operation),
		).WithOriginalError(err)

	case strings.Contains(errStr, "forbidden"):
		return NewPermissionError(operation).WithOriginalError(err)

	case strings.Contains(errStr, "connection refused"):
		return NewClusterAccessError().WithOriginalError(err)

	default:
		return NewDetailedError(
			ErrorTypeKubectl,
			fmt.Sprintf("Failed to %s", operation),
		).WithOriginalError(err)
	}
}
