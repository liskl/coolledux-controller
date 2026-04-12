package models

// DeviceInfo holds the current state and identity of the LED matrix device.
type DeviceInfo struct {
	// Identity
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	HardwareVersion string `json:"hardware_version"`
	SerialNumber    string `json:"serial_number"`

	// Display dimensions
	Columns int `json:"columns"`
	Rows    int `json:"rows"`

	// Current state
	Power      bool     `json:"power"`
	Brightness uint8    `json:"brightness"`
	FlipMode   FlipMode `json:"flip_mode"`

	// BLE connection info
	BLEAddress string `json:"ble_address"`
	BLEName    string `json:"ble_name"`
	Connected  bool   `json:"connected"`
}
