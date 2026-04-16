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
  coolledux/{device_id}/color/mode/set     plain text: "off" or a mode ID (e.g. "10")
  coolledux/{device_id}/color/speed/set    plain text: integer 1-10
  coolledux/{device_id}/show_id/set        plain text: "ON" or "OFF"
  coolledux/{device_id}/remote/set         plain text: "ON" or "OFF"
```

## Home Assistant Auto-Discovery Payloads

Published with `retain: true` on MQTT connect/reconnect.

**Light entity** (`homeassistant/light/coolledux_{device_id}/config`):
- brightness 0-255, RGB color, effect modes
- Device: manufacturer "E-CrossStu / CoolLEDUX", model "JT_HW358.02 16x96"

**Binary sensor** (`homeassistant/binary_sensor/coolledux_{device_id}_connection/config`):
- Connectivity based on availability topic

**Select entity** (`homeassistant/select/coolledux_{device_id}_color_mode/config`):
- Options: `"off"` + the valid color mode IDs ("1", "2", "5", ..., "31")
- `"off"` re-sends the last known static RGB color (reverts 0x13/0x03 → 0x13/0x01)

**Number entity** (`homeassistant/number/coolledux_{device_id}_color_speed/config`):
- Range 1-10, step 1, slider UI

**Switch entities**:
- `homeassistant/switch/coolledux_{device_id}_show_id/config` — toggles the 0x1E/0x01 "show device ID" flag.
- `homeassistant/switch/coolledux_{device_id}_remote/config` — toggles the 0x1E/0x02 "remote enable" flag.

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

### Device Info Toggles (`coolledux/{device_id}/show_id/set`, `coolledux/{device_id}/remote/set`)

Plain-text payload (not JSON), HA switch convention:

```
ON
```

or

```
OFF
```

`show_id` maps to BLE command `0x1E` subtype `0x01`; `remote` maps to `0x1E` subtype `0x02`. Both affect the panel's idle/default scroll only — no visible change while an active program is running.

### Color Mode (`coolledux/{device_id}/color/mode/set`)

Plain-text payload (not JSON):

```
10
```

Valid values are `off` or any ID returned by the protocol color-mode table (1, 2, 5..31). `off` reverts to the last static RGB color. See `docs/specs/protocol-ble.md` "Color Mode and Speed" for what each mode does visually.

### Color Speed (`coolledux/{device_id}/color/speed/set`)

Plain-text integer 1-10:

```
7
```

Out-of-range values are clamped rather than rejected so HA slider edges don't produce errors. No-op unless a color mode is currently active.

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
