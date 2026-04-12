package models

// DeviceInfo holds the current state and identity of the LED matrix device.
// Field mapping verified from the CoolLED 1248 Android app and real hardware.
type DeviceInfo struct {
	// Current state (from 0x1F response)
	Power            bool     `json:"power"`
	Brightness       uint8    `json:"brightness"`
	FlipMode         FlipMode `json:"flip_mode"`
	MicSupported     bool     `json:"mic_supported"`
	MicEnabled       bool     `json:"mic_enabled"`
	MicMode          uint8    `json:"mic_mode"`
	ShowDeviceID     bool     `json:"show_device_id"`
	MaxProgramNumber uint8    `json:"max_program_number"`
	RemoteEnabled    bool     `json:"remote_enabled"`

	// Extended fields (indices 10-18, not fully mapped)
	ExtendedData []byte `json:"extended_data,omitempty"`

	// BLE connection info (populated by the service, not from device response)
	BLEAddress string `json:"ble_address,omitempty"`
	BLEName    string `json:"ble_name,omitempty"`
	Connected  bool   `json:"connected"`
}
