# CoolLEDUX Controller -- Go Service

A standalone Go service that controls a CoolLEDUX 16x96 full-color RGB LED matrix sign over Bluetooth Low Energy (BLE). Exposes a REST API for direct control and integrates with Home Assistant via MQTT Auto-Discovery.

**Target hardware:** JT_HW358.02 16x96 LED matrix (E-CrossStu / CoolLEDUX)
**Reference implementation:** Python SDK at `NunoMiguelVeloso/coolledux-controller` on GitHub
**Go module:** `github.com/liskl/coolledux-controller`
**Minimum Go version:** 1.23

## Protocol Specs

Detailed protocol specifications live in `docs/specs/`:

- [BLE Protocol](docs/specs/protocol-ble.md) -- command codes, packet framing, CRC32, LZSS, program upload, image encoding (verified against real hardware)
- [REST API](docs/specs/protocol-rest.md) -- all HTTP endpoints, request/response schemas
- [MQTT / Home Assistant](docs/specs/protocol-mqtt.md) -- topic structure, HA auto-discovery payloads, command handling

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
  docs/specs/                       # Protocol specifications
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

## Configuration

```yaml
ble:
  device_name: "CoolLEDUX"
  device_mac: "01:00:00:FB:A4:16"
  service_uuid: "0000fff0-0000-1000-8000-00805f9b34fb"
  char_uuid: "0000fff1-0000-1000-8000-00805f9b34fb"
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
  topic_prefix: "coolledux"
  ha_discovery_prefix: "homeassistant"
  keepalive: "30s"

api:
  listen: ":8080"
  cors_origins: ["*"]

log:
  level: "info"
  format: "json"
```

Env vars override config: prefix `COOLLEDUX_`, underscores for nesting (e.g. `COOLLEDUX_BLE_DEVICE_MAC`).

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

2. **Command codes differ from SDK docs.** BRIGHTNESS=0x04 (not 0x06), MIRROR/ROTATE=0x0C (the APK builders `getSetMirror` and `setRotate` both write `0x0C`). Program start is `0x02` (3-arg) or `0x1A` (simple); there is no `0x07` or `0x08` command builder in the APK.

3. **layer_type MUST be 1** in all program content structures. Using 0 causes silent failure (device ACKs but shows default text).

4. **LZSS compression is required** for program uploads. Uncompressed data is accepted (ACKed) but not displayed correctly.

5. **CRC32 uses 32 iterations per byte**, polynomial 0x4C11DB7, no final XOR. Output little-endian for control commands, big-endian in program start metadata.

6. **RGB444 transfer is piecewise**: `>=238->15, <=47->0, else (value-47)/14+1`. Not a bit shift. (Source: `TextEmojiManagerCoolLEDUX.java:396`.)

7. **Column-major pixel ordering.** Outer loop columns, inner loop rows. Index = `row * width + col`.

8. **Program wrapper:** 8 zero bytes + content count (1) + separator (0x00) + content data.

9. **XOR checksum** on data chunks (not CRC32). XOR of bytes from offset 1 through end of chunk data.

10. **BLE connection is flaky on Linux.** Adapter reset between sessions, 2s post-scan delay, and 3-attempt retry are all necessary.

11. **Send chunks in 20-byte MTU pieces** with ~50ms inter-piece delay.

12. **Escape applies to length bytes too.** The 2-byte big-endian length is part of the escaped region.
