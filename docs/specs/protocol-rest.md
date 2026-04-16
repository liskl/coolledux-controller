# REST API Specification

Fiber v2, default port `:8080`.

Every device-addressed endpoint is keyed by the panel's registry ID (the normalized MAC — lowercase, no colons; `01:00:00:FB:A4:16` becomes `010000fba416`). Use `GET /devices` to list the IDs currently registered; `POST /scan` discovers nearby panels without modifying the registry. Startup auto-populates the registry when `ble.devices` is empty and `ble.scan_on_startup` isn't explicitly disabled.

## Endpoints

### Service-level

| Method | Path | Body | Description |
|--------|------|------|-------------|
| GET | `/health` | - | Health check (primary BLE/MQTT status, uptime) |
| GET | `/fonts` | - | List available fonts for `/device/:id/text` |
| GET | `/devices` | - | Registered devices with per-device connection state |
| POST | `/scan` | - | BLE scan; returns nearby CoolLEDUX advertisers and their `registered` flag |

### Per-device

`:id` is the normalized MAC returned from `/devices`.

| Method | Path | Body | Description |
|--------|------|------|-------------|
| GET | `/device/:id/info` | - | Device info from BLE |
| POST | `/device/:id/power` | `{"state":"on"}` | Power on/off |
| POST | `/device/:id/brightness` | `{"brightness":128}` | Brightness 0-255 |
| POST | `/device/:id/flip` | `{"mode":"horizontal"}` | none/horizontal/vertical/both |
| POST | `/device/:id/channel` | `{"channel":0}` | Switch program/channel slot (0-9 verified) |
| POST | `/device/:id/time` | `{"hour":14,"minute":30,"second":0}` | Sync time |
| POST | `/device/:id/timer` | `{"items":[...]}` | Set timers |
| GET | `/device/:id/timer` | - | Read raw timer bytes from device |
| POST | `/device/:id/reset` | - | Factory reset |
| POST | `/device/:id/show-id` | `{"on":true}` | 0x1E/0x01 toggle |
| POST | `/device/:id/remote` | `{"on":false}` | 0x1E/0x02 toggle |
| POST | `/device/:id/password/check` | `{"password":"1234"}` | 0x0D verify; 200 ok, 401 rejected, 400 bad format |
| POST | `/device/:id/password/set` | `{"password":"abcd"}` | 0x0E persistent set — risk: lockout if forgotten |
| POST | `/device/:id/text` | See below | Display text |
| POST | `/device/:id/image` | See below | Display image |
| POST | `/device/:id/gif` | See below | Display GIF |
| POST | `/device/:id/color` | `{"color":"#FF8800"}` | Set global tint color (CMD_COLOR 0x13/0x01) |
| POST | `/device/:id/color/mode` | `{"mode":10}` | Preset color animation (0x13/0x03) |
| POST | `/device/:id/color/speed` | `{"speed":5}` | Animation cycle speed (0x13/0x02) |
| POST | `/device/:id/countdown` | See below | Countdown timer overlay (0x0a + 0x0F) |
| POST | `/device/:id/stopwatch` | See below | Stopwatch overlay (0x10) |
| POST | `/device/:id/scoreboard` | See below | Scoreboard overlay (0x11; composite program with scores + clock) |
| POST | `/device/:id/debug/timecount` | - | Experimental: raw 39-byte digit-bitmap probe |

Unknown `:id` returns 404; an unconfigured registry returns 503.

## Request/Response Details

### GET /health

```json
{"status": "ok", "ble_connected": true, "mqtt_connected": false, "uptime_seconds": 3600}
```

### POST /device/:id/text

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

### POST /device/:id/image

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

### POST /device/:id/gif

```json
{
  "gif_base64": "<base64-encoded GIF>",
  "frame_duration": 100,
  "fit": "letterbox",
  "x": 0,
  "y": 0,
  "width": 96,
  "height": 16,
  "raw": false
}
```

`fit`, `x`, `y`, `width`, `height` accept the same values and defaults as `/device/:id/image`.

`raw: true` opts into content type `0x0C` (firmware v30+): the GIF is uploaded verbatim and decoded on-device. `frame_duration` and `fit` are ignored in this mode because the firmware handles timing and scaling. Older firmware ACKs the upload but renders nothing, so only set this when you know the device supports it. Default `false` keeps the frame-by-frame `0x03` path that works on all firmware revisions.

### POST /device/:id/show-id

Toggles whether the panel displays its device identifier. Maps to BLE command `0x1E` subtype `0x01`. Hardware-validated on 16x96; affects the default/idle scroll text, not any actively displayed program.

```json
{"on": true}
```

### POST /device/:id/remote

Toggles the panel's remote-control mode. Maps to BLE command `0x1E` subtype `0x02`. Same idle-only visibility as `/device/:id/show-id`.

```json
{"on": false}
```

### POST /device/:id/color

Sets the global tint color applied to text/content. Maps to BLE command `0x13` subtype `0x01` (RGB444 packed).

```json
{"color": "#FF8800"}
```

### POST /device/:id/color/mode

Activates one of the built-in color animation presets. Maps to BLE command `0x13` subtype `0x03`. See `docs/specs/protocol-ble.md` "Color Mode and Speed" for the full mode table and the semantic meaning of the per-mode `i3`/`i4`/`i2` parameters the firmware exposes.

```json
{"mode": 10}
```

Valid IDs are 1, 2, 5..31. Modes 3 and 4 are rejected with HTTP 400 because the APK resolves them to an empty no-op.

### POST /device/:id/color/speed

Adjusts how fast the active color animation cycles. Maps to BLE command `0x13` subtype `0x02`.

```json
{"speed": 7}
```

Integer 1-10. No-op unless a color mode is currently active.

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

The `name` field is what to pass in `POST /device/:id/text`'s `font` field.

### POST /device/:id/{countdown,stopwatch,scoreboard}

Action-based overlays. Action-specific fields are optional.

```json
// POST /device/:id/countdown
{"action": "show", "hour": 0, "minute": 1, "second": 30, "color": "#00FF00"}
// actions: "show" (upload program + set + start), "set", "start", "stop", "status"
```

```json
// POST /device/:id/stopwatch
{"action": "show", "color": "#00FF00"}
// actions: "show" (upload program + reset + start), "reset", "start", "stop", "status"
```

```json
// POST /device/:id/scoreboard
{"action": "show", "color": "#FFFFFF"}
// actions: "show" (upload program + prime scores/clock + start), "set_scores", "set_time", "start", "stop", "status"
// set_scores payload: {"action":"set_scores","score_a":12,"score_b":7,"total_a":1,"total_b":0}
//   score_a/score_b -- host/visit main scores, uint16 (2 BE bytes on the wire)
//   total_a/total_b -- host/visit period/set counter, uint8 (the small digits above each main score)
```

For `/countdown`, `action: "show"` uploads a composite program (content type `0x03` animation + content type `0x0a` time-count) and starts the firmware timer via `0x0F`. The animation block is the APK's pre-baked 18-frame purple frame + hourglass (`ic_countdown_bg_animation_1696.gif`). The time-count block carries the APK's 140-byte digit bitmap (14 bytes/digit, 7 cols × 2 bytes, MSB=row 0) for clean 7×10 hollow digits matching the APK's visual output.

For `/stopwatch`, `action: "show"` is the sibling path: same composite program shape and identical time-count layout on 16x96, but the animation block uses the APK's stopwatch background (`ic_stopwatch_bg_animation_1696.gif`) and the firmware is driven by the `0x10` command family (reset `0x10 02`, start/stop `0x10 03 01/00`, status `0x10 01`). The display always counts upward from `00:00:00`.

For `/scoreboard`, `action: "show"` uploads a different composite: content type `0x0b` with distinct regions for host/visit team scores (3 digits each, 7×10 glyphs), a 1-digit period/set counter per team (4×5 glyphs), and an MM:SS game clock (4×5 glyphs with a 1-column colon). The animation block uses `ic_scoreboard_bg_1696.gif`. After upload the handler primes with `set_scores(0, 0, 0, 0)` and `set_time(0, 0, timer=true)` then starts the firmware via `0x11 04 01`. Subsequent `/scoreboard` calls with `set_scores` / `set_time` drive updates without re-uploading the program.

Clock sequencing: `set_time` while the clock is running silently fails to latch the new value on 16x96 — the firmware only picks up a new time when the clock is stopped. To reset the clock after `show`, send `{"action":"stop"}`, then `{"action":"set_time",...}`, then `{"action":"start"}`. Main scores and period counters via `set_scores` update immediately without this dance.

### GET /device/:id/timer

Reads the current timer table from the device. Returns the raw device payload (base64-encoded bytes) under `data` without further parsing; callers are responsible for interpreting the byte layout (see `protocol-ble.md` for the timer format).

```json
{"success": true, "data": "<base64 bytes>"}
```

### Debug: POST /device/:id/debug/timecount

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
