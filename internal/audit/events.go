package audit

// EventType represents the type of audit event.
type EventType string

const (
	// Authentication events
	EventAuthSuccess    EventType = "auth_success"
	EventAuthFailure    EventType = "auth_failure"
	EventAuthInvalidKey EventType = "auth_invalid_key"
	EventAuthMissingKey EventType = "auth_missing_key"

	// Admin actions
	EventModuleEnabled  EventType = "module_enabled"
	EventModuleDisabled EventType = "module_disabled"
	EventModuleConfig   EventType = "module_config_changed"

	// Permission changes (future)
	EventPermissionGranted EventType = "permission_granted"
	EventPermissionRevoked EventType = "permission_revoked"

	// Sensitive data access
	EventSensitiveDataRead  EventType = "sensitive_data_read"
	EventSensitiveDataWrite EventType = "sensitive_data_write"

	// Security configuration changes
	EventSecurityConfigChanged EventType = "security_config_changed"
)

// Event represents an audit event.
type Event struct {
	Type      EventType      `json:"type"`
	Timestamp string         `json:"timestamp"`  // ISO8601
	RequestID string         `json:"request_id"` // Correlation ID
	UserID    string         `json:"user_id"`    // Discord user ID (if available)
	APIKey    string         `json:"api_key"`    // Masked API key (first 4 chars)
	IPAddress string         `json:"ip_address"`
	UserAgent string         `json:"user_agent"`
	Method    string         `json:"method"`
	Path      string         `json:"path"`
	Status    int            `json:"status"`
	Details   map[string]any `json:"details"` // Event-specific details
}

// NewEvent creates a new audit event with common fields.
func NewEvent(eventType EventType, requestID, apiKey, ipAddress, userAgent, method, path string, status int) Event {
	return Event{
		Type:      eventType,
		Timestamp: "", // Set by auditor
		RequestID: requestID,
		APIKey:    maskAPIKey(apiKey),
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Method:    method,
		Path:      path,
		Status:    status,
		Details:   make(map[string]any),
	}
}

// WithUserID sets the user ID and returns the event.
func (e Event) WithUserID(userID string) Event {
	e.UserID = userID
	return e
}

// WithDetails adds event-specific details and returns the event.
func (e Event) WithDetails(details map[string]any) Event {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// MaskAPIKey masks the API key for logging (shows first 4 characters).
func MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 4 {
		return "***"
	}
	return apiKey[:4] + "***"
}

// maskAPIKey is the private version for internal use.
func maskAPIKey(apiKey string) string {
	return MaskAPIKey(apiKey)
}
