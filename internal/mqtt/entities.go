package mqtt

// HADevice represents the "device" block in Home Assistant MQTT discovery payloads.
type HADevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name,omitempty"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Model        string   `json:"model,omitempty"`
	SWVersion    string   `json:"sw_version,omitempty"`
}

// LightConfig is the HA auto-discovery payload for a light entity.
type LightConfig struct {
	Name                string   `json:"name"`
	UniqueID            string   `json:"unique_id"`
	ObjectID            string   `json:"object_id"`
	CommandTopic        string   `json:"command_topic"`
	StateTopic          string   `json:"state_topic"`
	AvailabilityTopic   string   `json:"availability_topic"`
	PayloadAvailable    string   `json:"payload_available"`
	PayloadNotAvailable string   `json:"payload_not_available"`
	Schema              string   `json:"schema"`
	Brightness          bool     `json:"brightness"`
	BrightnessScale     int      `json:"brightness_scale"`
	ColorMode           bool     `json:"color_mode"`
	SupportedColorModes []string `json:"supported_color_modes"`
	Effect              bool     `json:"effect"`
	EffectList          []string `json:"effect_list"`
	Device              HADevice `json:"device"`
}

// BinarySensorConfig is the HA auto-discovery payload for a binary_sensor entity.
type BinarySensorConfig struct {
	Name        string   `json:"name"`
	UniqueID    string   `json:"unique_id"`
	ObjectID    string   `json:"object_id"`
	StateTopic  string   `json:"state_topic"`
	PayloadOn   string   `json:"payload_on"`
	PayloadOff  string   `json:"payload_off"`
	DeviceClass string   `json:"device_class"`
	Device      HADevice `json:"device"`
}

// SensorConfig is the HA auto-discovery payload for a sensor entity.
type SensorConfig struct {
	Name              string   `json:"name"`
	UniqueID          string   `json:"unique_id"`
	ObjectID          string   `json:"object_id"`
	StateTopic        string   `json:"state_topic"`
	ValueTemplate     string   `json:"value_template"`
	UnitOfMeasurement string   `json:"unit_of_measurement"`
	Device            HADevice `json:"device"`
}

// LightState is the JSON published to the state topic.
type LightState struct {
	State      string    `json:"state"`
	Brightness uint8     `json:"brightness"`
	Color      *RGBColor `json:"color,omitempty"`
	Effect     string    `json:"effect,omitempty"`
}

// RGBColor represents an RGB color value for HA JSON schema lights.
type RGBColor struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

// LightCommand is the JSON received on the light set topic.
type LightCommand struct {
	State      string    `json:"state,omitempty"`
	Brightness *uint8    `json:"brightness,omitempty"`
	Color      *RGBColor `json:"color,omitempty"`
	Effect     string    `json:"effect,omitempty"`
}

// TextCommand is the JSON received on the text set topic.
type TextCommand struct {
	Text     string `json:"text"`
	Mode     string `json:"mode"`
	Speed    uint8  `json:"speed"`
	Color    string `json:"color"`
	FontSize int    `json:"font_size"`
	Font     string `json:"font"`
}

// ImageCommand is the JSON received on the image set topic.
type ImageCommand struct {
	ImageBase64 string `json:"image_base64"`
	Fit         string `json:"fit"`
	Mode        string `json:"mode"`
}

// GIFCommand is the JSON received on the GIF set topic.
type GIFCommand struct {
	GIFBase64     string `json:"gif_base64"`
	FrameDuration uint16 `json:"frame_duration"`
	Fit           string `json:"fit"`
}
