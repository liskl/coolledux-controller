# MQTT and Home Assistant Integration Specification

## Configuration

| Setting | Env Var | Default |
|---------|---------|---------|
| Broker URL | `COOLLEDUX_MQTT_BROKER` | `tcp://localhost:1883` |
| Client ID | `COOLLEDUX_MQTT_CLIENT_ID` | `coolledux-controller` |
| Username | `COOLLEDUX_MQTT_USERNAME` | (empty) |
| Password | `COOLLEDUX_MQTT_PASSWORD` | (empty) |
| Topic prefix | `COOLLEDUX_MQTT_TOPIC_PREFIX` | `coolledux` |
| HA discovery prefix | `COOLLEDUX_MQTT_HA_PREFIX` | `homeassistant` |
| Keepalive | `COOLLEDUX_MQTT_KEEPALIVE` | `30s` |

## Device ID

Derived from MAC with colons stripped and lowercased: `010000fba416`.

## Topic Structure

```
State (published by service):
  coolledux/{device_id}/state              JSON: power, brightness, color, effect
  coolledux/{device_id}/availability       "online" or "offline"

Commands (subscribed by service):
  coolledux/{device_id}/set                JSON: power, brightness, color, effect
  coolledux/{device_id}/text/set           JSON: text, mode, speed, color, font_size, font
  coolledux/{device_id}/image/set          JSON: image_base64, mode, fit, x, y, width, height
  coolledux/{device_id}/gif/set            JSON: gif_base64, frame_duration, fit, x, y, width, height
```

## Home Assistant Auto-Discovery Payloads

Published with `retain: true` on MQTT connect/reconnect.

**Light entity** (`homeassistant/light/coolledux_{device_id}/config`):
- brightness 0-255, RGB color, effect modes
- Device: manufacturer "E-CrossStu / CoolLEDUX", model "JT_HW358.02 16x96"

**Binary sensor** (`homeassistant/binary_sensor/coolledux_{device_id}_connection/config`):
- Connectivity based on availability topic

## Last Will and Testament

- Topic: `coolledux/{device_id}/availability`
- Payload: `offline`
- QoS: 1
- Retain: true

On successful startup, publish `online` to the same topic with retain.

## Command Payloads

### Light Set (`coolledux/{device_id}/set`)

```json
{
  "state": "ON",
  "brightness": 200,
  "color": {"r": 255, "g": 0, "b": 0},
  "effect": "scroll_left"
}
```

Maps `state` ON/OFF to power, `brightness` to brightness command, `effect` to TextShowMode.

### Text (`coolledux/{device_id}/text/set`)

```json
{
  "text": "Hello World",
  "mode": "scroll_left",
  "speed": 5,
  "color": "#FF0000",
  "font_size": 16,
  "font": "8x16"
}
```

`font` is optional; empty selects the default. Valid names come from the REST endpoint `GET /fonts`.

### Image (`coolledux/{device_id}/image/set`)

```json
{
  "image_base64": "<base64-encoded PNG/JPEG>",
  "mode": "static",
  "fit": "letterbox"
}
```

### GIF (`coolledux/{device_id}/gif/set`)

```json
{
  "gif_base64": "<base64-encoded GIF>",
  "frame_duration": 100,
  "fit": "letterbox"
}
```

`fit` is optional. Values: `letterbox` (default), `stretch`, `cover`. `x`, `y`, `width`, `height` place the content as a sprite (defaults cover the full display). See the REST spec for descriptions.
