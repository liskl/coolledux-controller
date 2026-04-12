# REST API Specification

Fiber v2, default port `:8080`.

## Endpoints

| Method | Path | Body | Description |
|--------|------|------|-------------|
| GET | `/health` | - | Health check (BLE/MQTT status, uptime) |
| GET | `/device/info` | - | Device info from BLE |
| POST | `/device/power` | `{"state":"on"}` | Power on/off |
| POST | `/device/brightness` | `{"brightness":128}` | Brightness 0-255 |
| POST | `/device/flip` | `{"mode":"horizontal"}` | none/horizontal/vertical/both |
| POST | `/device/time` | `{"hour":14,"minute":30,"second":0}` | Sync time |
| POST | `/device/timer` | `{"items":[...]}` | Set timers |
| POST | `/device/reset` | - | Factory reset |
| POST | `/display/text` | See below | Display text |
| POST | `/display/image` | See below | Display image |
| POST | `/display/gif` | See below | Display GIF |

## Request/Response Details

### GET /health

```json
{"status": "ok", "ble_connected": true, "mqtt_connected": false, "uptime_seconds": 3600}
```

### POST /display/text

```json
{
  "text": "Hello",
  "mode": "scroll_left",
  "speed": 5,
  "color": "#FF0000",
  "font_size": 16,
  "stay_time": 0
}
```

`mode` values: `static`, `scroll_left`, `scroll_right`, `scroll_up`, `scroll_down`, `blink`, `fade_in`, `fade_out`, `zoom_in`, `zoom_out`, `rotate`, `wave`

### POST /display/image

```json
{
  "image_base64": "<base64-encoded PNG/JPEG/BMP>",
  "mode": "static",
  "speed": 1,
  "stay_time": 0
}
```

Verified working end-to-end: base64 PNG -> decode -> resize to 96x16 -> RGB444 -> LZSS -> BLE upload -> device displays image.

### POST /display/gif

```json
{
  "gif_base64": "<base64-encoded GIF>",
  "frame_duration": 100
}
```

### Success/Error Responses

```json
{"success": true}
```

```json
{"success": false, "error": "description of what went wrong"}
```

## Timer Day Bitmask

| Day | Bit |
|-----|-----|
| Monday | 0x01 |
| Tuesday | 0x02 |
| Wednesday | 0x04 |
| Thursday | 0x08 |
| Friday | 0x10 |
| Saturday | 0x20 |
| Sunday | 0x40 |
| Daily | 0x7F |
| Weekdays | 0x1F |
| Weekends | 0x60 |
