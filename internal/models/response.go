package models

// ActionResponse represents a generic device response to a command.
type ActionResponse struct {
	Type         uint8  `json:"type"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	StatusCode   uint8  `json:"status_code"`
}

// InfoResponse is a device response that includes device identity and state
// information alongside the standard action response fields.
type InfoResponse struct {
	ActionResponse
	DeviceInfo
}
