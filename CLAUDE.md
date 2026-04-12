# CoolLEDUX Controller -- Go Service

A standalone Go service that controls a CoolLEDUX 16x96 full-color RGB LED matrix sign over Bluetooth Low Energy (BLE). Exposes a REST API for direct control and integrates with Home Assistant via MQTT Auto-Discovery.

**Target hardware:** JT_HW358.02 16x96 LED matrix (E-CrossStu / CoolLEDUX)
**Reference implementation:** Python SDK at `NunoMiguelVeloso/coolledux-controller` on GitHub
**Go module:** `github.com/liskl/coolledux-controller`
**Minimum Go version:** 1.23

---

## BLE Protocol Specification (Verified Against Real Hardware)

The protocol details below were verified by live testing against a CoolLEDUX device (MAC `01:00:00:FB:A4:16`) on 2026-04-11. Several command codes differ from the Python SDK documentation. **Trust the "Verified" values below, not the SDK source.**

### Connection Parameters

| Parameter | Value |
|-----------|-------|
| BLE Service UUID | `0000fff0-0000-1000-8000-00805f9b34fb` |
| BLE Characteristic UUID | `0000fff1-0000-1000-8000-00805f9b34fb` |
| Characteristic properties | read, write-without-response, notify |
| Default device name | `CoolLEDUX` |
| Default MAC address | `01:00:00:FB:A4:16` |
| MTU | 23 (observed), write payload = 20 bytes |
| Reconnect delay | 1 second minimum after disconnect |
| Notification start delay | 400ms before first attempt |
| Max notification retries | 3 (then disconnect) |

**BLE Connection Quirks (tinygo bluetooth on Linux/WSL2):**
- Must reset adapter (`bluetoothctl power off/on`) between connection sessions for reliability
- Need 2-second pause after scan completes before calling Connect
- Connection retry (up to 3 attempts, 2s between) helps with flaky "le-connection-abort-by-local"
- Uses write-without-response (only mode tinygo supports on Linux)

### Packet Framing: Stream Frame Only

**All packets** (control commands, program start, program data chunks) use the same framing:

```
[0x01][length:2 BE][escaped_payload...][0x03]
```

- `0x01` = start byte
- Length = big-endian uint16, counts payload bytes BEFORE escaping
- Escaped payload = length bytes + payload with escape sequences applied
- `0x03` = end byte

**Escape rules:** Bytes `0x01-0x03` within the length+payload region are escaped:
- Replace byte `B` with `[0x02][B XOR 0x04]`
- On decode: when `0x02` is seen, consume next byte and XOR with `0x04`

**The `[0x52,0x52]` BLE packet header described in the Python SDK is NOT used.** The device ignores packets with that header. All packets are stream-framed only.

### Command Codes (Verified)

| Command | Code | Payload | Verified |
|---------|------|---------|----------|
| BRIGHTNESS | `0x04` | `[value:1]` (0-255) | Yes, device echoes value in ACK |
| POWER | `0x05` | `[0x01]`=on, `[0x00]`=off | Yes, device sends ACK |
| CMD 0x06 | `0x06` | - | Device ignores ALL packets with this code |
| CHANNEL | `0x07` | `[slot:1]` (program/channel index) | Yes, switches displayed program |
| PROGRAM | `0x08` | See "Program Upload" below | Yes, device ACKs start + chunks |
| PASSWORD | `0x09` | `[op:1][password_bytes...]` | ACK observed |
| TIME | `0x0A` | `[hour:1][minute:1][second:1]` | ACK observed |
| TIMER | `0x0B` | `[count:1][items...]` | ACK observed |
| FLIP | `0x0C` | `[mode:1]` 0=none, 1=H, 2=V, 3=both | Yes, all 4 modes visually verified |
| INFO | `0x0D` | Empty (request) | ACK observed |
| RESET | `0x0E` | Empty | ACK observed |

**SDK vs Reality:**
- SDK says BRIGHTNESS=0x06, device uses **0x04**
- SDK says FLIP=0x07, device uses **0x0C** (SDK's 0x07 is actually channel/program switch)
- SDK says OTA=0x0C, but 0x0C is actually FLIP on this device

**Command packet format (all commands):**
```
stream_frame([CMD_CODE:1][data_bytes...][CRC32:4 LE])
```

### Response Format

Device responses arrive via BLE notifications, also stream-framed:
```
stream_frame([response_type:1][status_or_data...])
```

Observed response types:
- `0x02` = Program start ACK
- `0x03` = Program data chunk ACK (includes chunk index info)
- `0x04` = Brightness ACK (echoes brightness value)
- `0x05` = Power ACK (echoes power state)
- `0x07` = Channel switch ACK
- `0x0C` = Flip ACK (echoes flip mode)
- `0x0D` = Device info response

### CRC32 Algorithm

Polynomial: `0x4C11DB7`. Initial: `0xFFFFFFFF`. Final XOR: **NONE**.

```
func CRC32(data []byte) uint32 {
    crc := uint32(0xFFFFFFFF)
    for _, b := range data {
        xbit := uint32(0x80000000)
        tmp := uint32(b) & 0xFF
        for i := 0; i < 32; i++ {
            if crc & 0x80000000 != 0 {
                crc = (crc << 1) ^ 0x4C11DB7
            } else {
                crc = crc << 1
            }
            if tmp & xbit != 0 {
                crc ^= 0x4C11DB7
            }
            xbit >>= 1
        }
    }
    return crc
}
```

32 iterations per byte. Output as 4 bytes **little-endian**. Verified: Go and Python produce identical CRC values.

### LZSS Compression

| Parameter | Value |
|-----------|-------|
| Window size | 512 bytes |
| Lookahead size | 18 bytes |
| Match threshold | 2 |
| Initial buffer position | 494 (`WINDOW_SIZE - LOOKAHEAD_SIZE`) |

Flag byte (8 ops): bit=1 literal (1 byte), bit=0 match ref (2 bytes). LSB-first.
Match: `byte0 = pos & 0xFF`, `byte1 = ((pos >> 4) & 0xF0) | (len - 3)`.

**LZSS is required for program uploads.** The device accepts uncompressed data (ACKs everything) but displays default text instead of the image. Compression must be applied.

### Program Upload Protocol (Verified)

Verified by uploading a 96x16 full-white image and a rainbow gradient. Both displayed correctly.

#### Step 1: Build Program Content

**Graffiti/Image content (type 0x02):**
```
Offset  Size  Field
------  ----  -----
0       4     Total length (includes these 4 bytes, big-endian)
4       1     Content type: 0x02
5       7     Reserved: 0x00 x 7
12      1     Layer type: 0x01 (MUST be 1, not 0)
13      2     Start column (big-endian, usually 0)
15      2     Start row (big-endian, usually 0)
17      2     Show width (big-endian, e.g. 96)
19      2     Show height (big-endian, e.g. 16)
21      1     Display mode (1=static)
22      1     Speed (1-10)
23      1     Stay time (0=infinite)
24      4     Image data length (big-endian)
28      N     Image data (column-major RGB444)
```

**layer_type MUST be 1.** Setting it to 0 causes the device to silently show default text instead of the uploaded image.

#### Step 2: Wrap in Program Envelope

```
[0x00 x 8][content_count:1 = 0x01][separator:1 = 0x00][content_data...]
```

#### Step 3: LZSS Compress

Compress the entire program envelope. For a 96x16 white image: 3110 bytes -> 392 bytes.

#### Step 4: Send Program Start

```
stream_frame([0x02][CRC32_of_raw:4 BE][raw_length:4 BE][index:1][count:1][show_count:1])
```

- CRC32 is computed over the raw (uncompressed) program envelope
- raw_length = byte count of uncompressed program envelope
- CRC32 is big-endian here (unlike control command CRC which is LE)

#### Step 5: Send Data Chunks

Split compressed data into chunks of up to 1024 bytes. For each chunk:

```
stream_frame([0x03][0x00][total_compressed_len:4 BE][chunk_idx:2 BE][chunk_len:2 BE][chunk_data...][XOR:1])
```

XOR checksum = XOR of all bytes from offset 1 through end of chunk data.

Send each stream-framed chunk in 20-byte MTU pieces with ~50ms delay between pieces. Wait for ACK after each complete chunk.

### Image Data Encoding

**RGB444 per pixel (2 bytes):**
```
func rgb444Transfer(value uint8) uint8 {
    if value >= 238 { return 15 }
    if value <= 30  { return 0 }
    return uint8((int(value) - 30) / 15 + 1)
}
```
Output: `[0x0R]` `[0xGB]` where R, G, B are 4-bit values.

**Column-major ordering:** Outer loop columns, inner loop rows. Source index = `row * width + col`.

Verified with rainbow gradient: red(0) -> yellow(16) -> green(32) -> cyan(48) -> blue(64) -> magenta(80) across 96 columns, all colors rendered correctly.

---

## Go Module Structure

```
coolledux-controller/
  cmd/
    coolledux-controller/
      main.go                       # Entrypoint: config load, DI wiring, graceful shutdown
  internal/
    ble/
      client.go                     # BLE connection manager (scan, connect, MTU, reconnect)
      transport.go                  # Chunked writes, notification handler, send lock
    protocol/
      constants.go                  # All command codes, response types, limits, defaults
      crc.go                        # CRC32 (polynomial 0x4C11DB7, no final XOR)
      lzss.go                       # LZSS compressor/decompressor
      packet.go                     # Stream framing [0x01..0x03] (BLE header NOT used)
      commands.go                   # Build command payloads (power, brightness, flip, etc.)
      response.go                   # Parse device responses by type code
      program.go                    # Program start/data packet builders, chunk management
    controller/
      controller.go                 # Orchestrator: state machine, program upload session
      state.go                      # DeviceState, ProgramSendingState enums
    models/
      device.go                     # DeviceInfo struct
      program.go                    # GraffitiProgram, AnimationProgram, TextProgram
      enums.go                      # TextShowMode, BorderMode, BorderType, FullColorType, FlipMode
      display.go                    # DrawItem
      response.go                   # ActionResponse, InfoResponse
    image/
      processor.go                  # Resize, rotate, flip, RGBA-to-DrawItem extraction
      gif.go                        # GIF frame extraction, duration parsing
      color.go                      # RGB888->RGB444, column-major encoding
      font.go                       # Embedded bitmap font assets, glyph lookup, //go:embed
    api/
      server.go                     # Fiber HTTP server setup, route registration
      handlers.go                   # Route handlers for all endpoints
      middleware.go                 # Structured logging, error recovery, CORS
      schemas.go                    # Request/response JSON structs
    mqtt/
      client.go                     # MQTT connection, publish/subscribe, LWT, reconnect
      discovery.go                  # Home Assistant MQTT auto-discovery payload builders
      handlers.go                   # Command topic handlers (power, brightness, text, image)
      entities.go                   # Entity definitions (light, binary_sensor)
    config/
      config.go                     # Viper-based config: YAML file + env vars + defaults
  config.example.yaml               # Example configuration file
  Dockerfile                        # Multi-stage build
  docker-compose.yml                # Service + Mosquitto broker
```

### Key Dependencies

| Package | Purpose |
|---------|---------|
| `tinygo.org/x/bluetooth` | BLE client (Linux BlueZ) |
| `github.com/eclipse/paho.mqtt.golang` | MQTT v3.1.1 client |
| `github.com/gofiber/fiber/v2` | HTTP framework |
| `github.com/spf13/viper` | Configuration (YAML + env) |
| `github.com/disintegration/imaging` | Image resize/rotate/flip |
| `log/slog` (stdlib) | Structured logging |

---

## MQTT and Home Assistant Integration

### MQTT Configuration

| Setting | Env Var | Default |
|---------|---------|---------|
| Broker URL | `COOLLEDUX_MQTT_BROKER` | `tcp://localhost:1883` |
| Client ID | `COOLLEDUX_MQTT_CLIENT_ID` | `coolledux-controller` |
| Username | `COOLLEDUX_MQTT_USERNAME` | (empty) |
| Password | `COOLLEDUX_MQTT_PASSWORD` | (empty) |
| Topic prefix | `COOLLEDUX_MQTT_TOPIC_PREFIX` | `coolledux` |
| HA discovery prefix | `COOLLEDUX_MQTT_HA_PREFIX` | `homeassistant` |
| Keepalive | `COOLLEDUX_MQTT_KEEPALIVE` | `30s` |

### Device ID

Derived from MAC with colons stripped and lowercased: `010000fba416`.

### Topic Structure

```
State (published by service):
  coolledux/{device_id}/state              JSON: power, brightness, color, effect
  coolledux/{device_id}/availability       "online" or "offline"

Commands (subscribed by service):
  coolledux/{device_id}/set                JSON: power, brightness, color, effect
  coolledux/{device_id}/text/set           JSON: text, mode, speed, color, font_size
  coolledux/{device_id}/image/set          JSON: image_base64, mode
  coolledux/{device_id}/gif/set            JSON: gif_base64, frame_duration
```

### Home Assistant Auto-Discovery Payloads

Published with `retain: true` on MQTT connect/reconnect.

**Light entity** (`homeassistant/light/coolledux_{device_id}/config`):
- brightness 0-255, RGB color, effect modes
- Device: manufacturer "E-CrossStu / CoolLEDUX", model "JT_HW358.02 16x96"

**Binary sensor** (`homeassistant/binary_sensor/coolledux_{device_id}_connection/config`):
- Connectivity based on availability topic

**LWT:** Topic `coolledux/{device_id}/availability`, payload `offline`, QoS 1, retain true.

---

## REST API Specification

Fiber v2, default port `:8080`.

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
| POST | `/display/text` | `{"text":"Hello","mode":"scroll_left","speed":5,"color":"#FF0000","font_size":16}` | Display text |
| POST | `/display/image` | `{"image_base64":"...","mode":"static"}` | Display image |
| POST | `/display/gif` | `{"gif_base64":"...","frame_duration":100}` | Display GIF |

---

## Configuration

### config.yaml

```yaml
ble:
  device_name: "CoolLEDUX"
  device_mac: "01:00:00:FB:A4:16"
  service_uuid: "0000fff0-0000-1000-8000-00805f9b34fb"
  char_uuid: "0000fff1-0000-1000-8000-00805f9b34fb"
  device_service_uuid: "9056aa8d-24a1-427e-ae91-b70e0bf992cd"
  scan_timeout: "10s"
  reconnect_interval: "5s"

display:
  columns: 96
  rows: 16
  default_brightness: 128
  default_font_size: 16
  default_color: "#FFFFFF"
  default_speed: 5

mqtt:
  broker: "tcp://localhost:1883"
  client_id: "coolledux-controller"
  username: ""
  password: ""
  topic_prefix: "coolledux"
  ha_discovery_prefix: "homeassistant"
  keepalive: "30s"

api:
  listen: ":8080"
  cors_origins:
    - "*"

log:
  level: "info"
  format: "json"
```

Env vars override config: `COOLLEDUX_BLE_DEVICE_MAC`, `COOLLEDUX_MQTT_BROKER`, etc. (prefix `COOLLEDUX_`, underscores for nesting).

---

## Build and Deployment

```bash
go build -o coolledux-controller ./cmd/coolledux-controller
go test ./...
```

Docker: multi-stage build (golang:1.23-alpine -> alpine:3.20 with bluez+dbus). Requires `network_mode: host` and `privileged: true` for BLE.

---

## Code Conventions

- `context.Context` first param, `fmt.Errorf("op: %w", err)` wrapping, `log/slog` logging
- No mocks, no global state, table-driven tests
- Acronyms capitalized: BLE, CRC, LZSS, MQTT, MTU, UUID

---

## Critical Implementation Notes

1. **Stream framing for everything.** All packets use `[0x01][len_BE][escaped][0x03]`. The `[0x52,0x52]` BLE header is NOT used. Verified on real hardware.

2. **Command codes differ from SDK docs.** BRIGHTNESS=0x04 (not 0x06), FLIP=0x0C (not 0x07), CHANNEL=0x07 (SDK calls this "flip").

3. **layer_type MUST be 1** in all program content structures. Using 0 causes silent failure (device ACKs but shows default text).

4. **LZSS compression is required** for program uploads. Uncompressed data is accepted (ACKed) but not displayed correctly.

5. **CRC32 uses 32 iterations per byte**, polynomial 0x4C11DB7, no final XOR. Output little-endian for control commands, big-endian in program start metadata.

6. **RGB444 transfer is piecewise**: `>=238->15, <=30->0, else (value-30)/15+1`. Not a bit shift.

7. **Column-major pixel ordering.** Outer loop columns, inner loop rows. Index = `row * width + col`.

8. **Program wrapper:** 8 zero bytes + content count (1) + separator (0x00) + content data.

9. **XOR checksum** on data chunks (not CRC32). XOR of bytes from offset 1 through end of chunk data.

10. **BLE connection is flaky on Linux.** Adapter reset between sessions, 2s post-scan delay, and 3-attempt retry are all necessary.

11. **Send chunks in 20-byte MTU pieces** with ~50ms inter-piece delay. Wait for notification ACK after each complete chunk.

12. **Escape applies to length bytes too.** The 2-byte big-endian length is part of the escaped region.
