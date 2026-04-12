package mqtt

import (
	"encoding/json"
	"fmt"
)

const (
	deviceName         = "CoolLEDUX Sign"
	deviceManufacturer = "E-CrossStu / CoolLEDUX"
	deviceModel        = "JT_HW358.02 16x96"
	swVersion          = "1.0.0"
)

// effectList is the set of display effects exposed to Home Assistant.
var effectList = []string{
	"static", "scroll_left", "scroll_right", "scroll_up", "scroll_down",
	"blink", "fade_in", "fade_out", "zoom_in", "zoom_out", "rotate", "wave",
}

// fullDevice returns an HADevice with all fields populated (for the primary entity).
func fullDevice(deviceID string) HADevice {
	return HADevice{
		Identifiers:  []string{fmt.Sprintf("coolledux_%s", deviceID)},
		Name:         deviceName,
		Manufacturer: deviceManufacturer,
		Model:        deviceModel,
		SWVersion:    swVersion,
	}
}

// refDevice returns an HADevice with only identifiers (for secondary entities
// that share the same device).
func refDevice(deviceID string) HADevice {
	return HADevice{
		Identifiers: []string{fmt.Sprintf("coolledux_%s", deviceID)},
	}
}

// BuildLightConfig builds the HA auto-discovery payload for the light entity.
// It returns the discovery topic and the JSON payload bytes.
func BuildLightConfig(deviceID, topicPrefix, haPrefix string) (string, []byte) {
	topic := fmt.Sprintf("%s/light/coolledux_%s/config", haPrefix, deviceID)

	cfg := LightConfig{
		Name:                deviceName,
		UniqueID:            fmt.Sprintf("coolledux_%s_light", deviceID),
		ObjectID:            "coolledux_sign",
		CommandTopic:        fmt.Sprintf("%s/%s/set", topicPrefix, deviceID),
		StateTopic:          fmt.Sprintf("%s/%s/state", topicPrefix, deviceID),
		AvailabilityTopic:   fmt.Sprintf("%s/%s/availability", topicPrefix, deviceID),
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		Schema:              "json",
		Brightness:          true,
		BrightnessScale:     255,
		ColorMode:           true,
		SupportedColorModes: []string{"rgb"},
		Effect:              true,
		EffectList:          effectList,
		Device:              fullDevice(deviceID),
	}

	payload, _ := json.Marshal(cfg)
	return topic, payload
}

// BuildConnectionSensorConfig builds the HA auto-discovery payload for the
// connection binary_sensor entity.
func BuildConnectionSensorConfig(deviceID, topicPrefix, haPrefix string) (string, []byte) {
	topic := fmt.Sprintf("%s/binary_sensor/coolledux_%s_connection/config", haPrefix, deviceID)

	cfg := BinarySensorConfig{
		Name:        "CoolLEDUX Connection",
		UniqueID:    fmt.Sprintf("coolledux_%s_connection", deviceID),
		ObjectID:    "coolledux_sign_connection",
		StateTopic:  fmt.Sprintf("%s/%s/availability", topicPrefix, deviceID),
		PayloadOn:   "online",
		PayloadOff:  "offline",
		DeviceClass: "connectivity",
		Device:      refDevice(deviceID),
	}

	payload, _ := json.Marshal(cfg)
	return topic, payload
}

// BuildBrightnessSensorConfig builds the HA auto-discovery payload for the
// brightness sensor entity.
func BuildBrightnessSensorConfig(deviceID, topicPrefix, haPrefix string) (string, []byte) {
	topic := fmt.Sprintf("%s/sensor/coolledux_%s_brightness/config", haPrefix, deviceID)

	cfg := SensorConfig{
		Name:              "CoolLEDUX Brightness",
		UniqueID:          fmt.Sprintf("coolledux_%s_brightness", deviceID),
		ObjectID:          "coolledux_sign_brightness",
		StateTopic:        fmt.Sprintf("%s/%s/state", topicPrefix, deviceID),
		ValueTemplate:     "{{ value_json.brightness }}",
		UnitOfMeasurement: "",
		Device:            refDevice(deviceID),
	}

	payload, _ := json.Marshal(cfg)
	return topic, payload
}
