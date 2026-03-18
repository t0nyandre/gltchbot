package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/t0nyandre/gltchbot/internal/logging"
)

// Auditor provides audit logging functionality.
type Auditor interface {
	// Log logs an audit event.
	Log(ctx context.Context, event Event)
	// LogEvent logs an audit event with the given type and details.
	LogEvent(ctx context.Context, eventType EventType, details map[string]any)
}

// auditorImpl implements Auditor using slog.
type auditorImpl struct {
	logger *slog.Logger
}

// New creates a new Auditor with the given logger.
// If logger is nil, a default audit logger will be created.
func New(logger *slog.Logger) Auditor {
	if logger == nil {
		logger = logging.ModuleLogger("audit")
	}
	return &auditorImpl{logger: logger}
}

// Log logs an audit event.
func (a *auditorImpl) Log(ctx context.Context, event Event) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	
	// Convert event to slog attributes
	attrs := []any{
		slog.String("type", string(event.Type)),
		slog.String("timestamp", event.Timestamp),
		slog.String("request_id", event.RequestID),
		slog.String("api_key", event.APIKey),
		slog.String("ip_address", event.IPAddress),
		slog.String("user_agent", event.UserAgent),
		slog.String("method", event.Method),
		slog.String("path", event.Path),
		slog.Int("status", event.Status),
	}
	if event.UserID != "" {
		attrs = append(attrs, slog.String("user_id", event.UserID))
	}
	if len(event.Details) > 0 {
		attrs = append(attrs, slog.Any("details", event.Details))
	}
	
	// Log at Info level (audit events are always considered info level)
	a.logger.InfoContext(ctx, "audit", attrs...)
}

// LogEvent logs an audit event with the given type and details.
// It extracts common fields from the request context if available.
func (a *auditorImpl) LogEvent(ctx context.Context, eventType EventType, details map[string]any) {
	event := a.eventFromContext(ctx, eventType, 0)
	if details != nil {
		event.Details = details
	}
	a.Log(ctx, event)
}

// eventFromContext creates an audit event from the request context.
// It extracts request ID, API key, IP address, user agent, method, and path.
func (a *auditorImpl) eventFromContext(ctx context.Context, eventType EventType, status int) Event {
	// Extract request-scoped logger from context to get request ID
	logger := logging.FromContext(ctx)
	
	// Default values
	requestID := ""
	apiKey := ""
	ipAddress := ""
	userAgent := ""
	method := ""
	path := ""
	
	// The request-scoped logger may have attributes attached.
	// We could also store request info in context via middleware.
	// For now, we rely on the auditor middleware to store request info in context.
	// This method will be called after middleware sets the audit context.
	
	// Try to get request info from context
	if ri, ok := RequestInfoFromContext(ctx); ok {
		requestID = ri.RequestID
		apiKey = ri.APIKey
		ipAddress = ri.IPAddress
		userAgent = ri.UserAgent
		method = ri.Method
		path = ri.Path
	}
	
	event := NewEvent(eventType, requestID, apiKey, ipAddress, userAgent, method, path, status)
	
	// If user ID is in context, add it
	if userID, ok := UserIDFromContext(ctx); ok {
		event.UserID = userID
	}
	
	return event
}

// RequestInfo holds request-specific information for audit logging.
type RequestInfo struct {
	RequestID string
	APIKey    string
	IPAddress string
	UserAgent string
	Method    string
	Path      string
}

type requestInfoKey struct{}

// WithRequestInfo adds request information to the context.
func WithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey{}, info)
}

// RequestInfoFromContext returns request information from the context.
func RequestInfoFromContext(ctx context.Context) (RequestInfo, bool) {
	info, ok := ctx.Value(requestInfoKey{}).(RequestInfo)
	return info, ok
}

// UserIDKey is the key used to store the user ID in context.
type userIDKey struct{}

// WithUserID adds the user ID to the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserIDFromContext returns the user ID from the context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey{}).(string)
	return userID, ok
}