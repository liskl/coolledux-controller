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
| POST | `/device/channel` | `{"channel":0}` | Switch program/channel slot (0-9 verified) |
| POST | `/device/time` | `{"hour":14,"minute":30,"second":0}` | Sync time |
| POST | `/device/timer` | `{"items":[...]}` | Set timers |
| POST | `/device/reset` | - | Factory reset |
| POST | `/display/text` | See below | Display text |
| POST | `/display/image` | See below | Display image |
| POST | `/display/gif` | See below | Display GIF |
| POST | `/display/color` | `{"color":"#FF8800"}` | Set global tint color (CMD_COLOR 0x13/0x01) |
| GET | `/fonts` | - | List available fonts for `/display/text` |
| POST | `/countdown` | See below | Countdown timer overlay (0x0a + 0x0F) |
| POST | `/stopwatch` | See below | Stopwatch overlay (0x10) |
| POST | `/scoreboard` | See below | Scoreboard overlay (0x11; no-op on 16x96) |
| POST | `/debug/timecount` | - | Experimental: raw 39-byte digit-bitmap probe |

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
  "stay_time": 0,
  "font": "8x16"
}
```

`mode` values: `static`, `scroll_left`, `scroll_right`, `scroll_up`, `scroll_down`, `wipe_down`, `expand_from_center`, `blink`, `zoom_in`, `zoom_out`, `wipe_left`, `wipe_right`, `collapse_to_center`

Several mode names diverge from the APK's strings because the APK's labels don't match the firmware's actual behavior on this hardware:

- `wipe_down` (mode 6) — reveals the image row-by-row from top to bottom. APK calls this `blink`.
- `expand_from_center` (mode 7) — horizontal iris-open wipe from the center column outward. APK calls this `fade_in`.
- `blink` (mode 8) — true on/off flash of the full image. APK calls this `fade_out`.
- `zoom_in` (mode 9) — behaves identically to `scroll_left` on this firmware.
- `zoom_out` (mode 10) — behaves identically to `scroll_right` on this firmware.
- `wipe_left` (mode 11) — column-by-column right-to-left reveal then dwell. APK calls this `rotate`.
- `wipe_right` (mode 12) — column-by-column left-to-right reveal then dwell. APK calls this `wave`.
- `collapse_to_center` (mode 13) — pieces of the image slide in from both edges and meet at the center. APK calls this `custom`. Visual mirror of `expand_from_center`.

Modes 14+ were probed during development and either silently render nothing or fall back to `scroll_left` behavior. Only modes 1..13 produce distinct animations on this firmware.

`font` is optional. Empty/missing selects the default (`7x13`). Unknown names return HTTP 500 with an explanatory error. `GET /fonts` lists what's registered. `font_size` is currently ignored; each registered font has a fixed cell size.

### POST /display/image

```json
{
  "image_base64": "<base64-encoded PNG/JPEG/BMP>",
  "mode": "static",
  "speed": 1,
  "stay_time": 0,
  "fit": "letterbox",
  "x": 0,
  "y": 0,
  "width": 96,
  "height": 16
}
```

Verified working end-to-end: base64 PNG -> decode -> resize -> RGB444 -> LZSS -> BLE upload -> device displays image.

`fit` is optional and selects how the source image maps onto the placement region:

- `letterbox` (default) -- preserve aspect ratio, center on a black canvas with bars on the empty axis.
- `stretch` -- ignore aspect ratio, scale to exactly the region dimensions (legacy behavior).
- `cover` -- preserve aspect ratio, scale to fill the region, crop overflow from the center.

`x`, `y`, `width`, `height` are optional and place the image as a sprite on the matrix:

- `x`, `y` default to `0` (top-left corner).
- `width` of `0` means "fill remaining display width from `x`"; same for `height`.
- The placement rectangle must fit within the 96x16 display, otherwise the request returns HTTP 500 with an error.

### POST /display/gif

```json
{
  "gif_base64": "<base64-encoded GIF>",
  "frame_duration": 100,
  "fit": "letterbox",
  "x": 0,
  "y": 0,
  "width": 96,
  "height": 16
}
```

`fit`, `x`, `y`, `width`, `height` accept the same values and defaults as `/display/image`.

### POST /display/color

Sets the global tint color applied to text/content. Maps to BLE command `0x13` subtype `0x01` (RGB444 packed).

```json
{"color": "#FF8800"}
```

### GET /fonts

```json
{
  "default": "7x13",
  "fonts": [
    {"name": "7x13",  "description": "Plan 9 bitmap monospace, 7x13 cell, pure 1-bit", "advance_px": 7, "line_px": 13, "monospace": true},
    {"name": "7x14b", "description": "X11 Misc Fixed Bold 7x14 (public domain)",      "advance_px": 7, "line_px": 14, "monospace": true},
    {"name": "8x16",  "description": "Spleen 8x16 by Frederic Cambus (BSD-2) — fills full matrix height", "advance_px": 8, "line_px": 16, "monospace": true}
  ]
}
```

The `name` field is what to pass in `POST /display/text`'s `font` field.

### POST /countdown, /stopwatch, /scoreboard

Action-based overlays. Action-specific fields are optional.

```json
// POST /countdown
{"action": "show", "hour": 0, "minute": 1, "second": 30, "color": "#00FF00"}
// actions: "show" (upload program + set + start), "set", "start", "stop", "status"
```

```json
// POST /stopwatch
{"action": "start"}
// actions: "reset", "start", "stop", "status"
```

```json
// POST /scoreboard     (ACKed on 16x96 hardware but not visible)
{"action": "set_scores", "score_a": 3, "score_b": 2}
// actions: "set_scores", "set_time", "start", "stop", "status"
```

For `/countdown`, `action: "show"` uploads a composite program (content type `0x03` animation + content type `0x0a` time-count) and starts the firmware timer via `0x0F`. The animation block is the APK's pre-baked 18-frame purple frame + hourglass (`ic_countdown_bg_animation_1696.gif`). The time-count block carries the APK's 140-byte digit bitmap (14 bytes/digit, 7 cols × 2 bytes, MSB=row 0) for clean 7×10 hollow digits matching the APK's visual output.

### Debug: POST /debug/timecount

Experimental. Uploads a hex-encoded bitmap in place of the APK's digit bitmap and starts the countdown, for probing the firmware's byte→pixel layout on other device sizes. Not part of the stable API; may be removed.

```json
{"probe_hex": "<hex bytes>", "color": "#00FF00"}
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
