// internal/service/errors/handler.go
package errors

import (
	"go.uber.org/zap"
)

// ErrorHandler handles service errors
type ErrorHandler interface {
	HandleError(err error) *ErrorResponse
	LogError(err error, context map[string]interface{})
	RecoverPanic()
}

// DefaultErrorHandler implements ErrorHandler
type DefaultErrorHandler struct {
	logger *zap.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *zap.Logger) ErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// HandleError converts an error to ErrorResponse
func (h *DefaultErrorHandler) HandleError(err error) *ErrorResponse {
	return NewErrorResponse(err, nil)
}

// LogError logs an error with context
func (h *DefaultErrorHandler) LogError(err error, context map[string]interface{}) {
	fields := make([]zap.Field, 0, len(context)+1)
	fields = append(fields, zap.Error(err))

	for key, value := range context {
		fields = append(fields, zap.Any(key, value))
	}

	h.logger.Error("Service error", fields...)
}

// RecoverPanic recovers from panics and logs them
func (h *DefaultErrorHandler) RecoverPanic() {
	if r := recover(); r != nil {
		h.logger.Error("Panic recovered",
			zap.Any("panic", r),
			zap.Stack("stack"))
	}
}
