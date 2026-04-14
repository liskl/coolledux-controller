# CoolLEDUX Controller

A standalone Go service that drives a **CoolLEDUX 16x96 full-color RGB LED matrix** (JT_HW358.02, sold under E-CrossStu / CoolLEDUX) over Bluetooth Low Energy. It exposes a REST API for direct control and integrates with Home Assistant via MQTT auto-discovery.

- **Module:** `github.com/liskl/coolledux-controller`
- **Minimum Go:** 1.23
- **Platform:** Linux (BlueZ); Docker-friendly
- **Reference:** Python SDK `NunoMiguelVeloso/coolledux-controller` (has incorrect command codes for this firmware; the APK decompile is the source of truth)

## Features

- BLE connection management (scan, connect, MTU negotiation, reconnect)
- REST API for power, brightness, text, images, GIFs, program upload
- Countdown, stopwatch, and scoreboard overlay endpoints (scoreboard is ACKed but not visible on 16x96)
- Image/GIF sprite placement with `x`/`y`/`width`/`height` and `fit` modes (`letterbox`, `stretch`, `cover`)
- MQTT client with Home Assistant auto-discovery (light + binary_sensor entities)
- Image/GIF rendering to RGB444 column-major frames
- Text rendering with three registered fonts: `7x13` (Plan 9, default), `7x14b` (X11 Misc Fixed Bold), `8x16` (Spleen)
- LZSS compression + stream framing matching the stock Android app

## Protocol Specifications

Detailed specs live in [`docs/specs/`](docs/specs/):

- [BLE Protocol](docs/specs/protocol-ble.md) -- command codes, packet framing, CRC32, LZSS, program upload, image encoding
- [REST API](docs/specs/protocol-rest.md) -- HTTP endpoints and JSON schemas
- [MQTT / Home Assistant](docs/specs/protocol-mqtt.md) -- topic structure and HA discovery payloads

## Build

Two assets are extracted from the CoolLED 1248 APK and are **not** checked in:

- `internal/text/fonts/unicode_16_bold.bin` -- legacy 16x16 glyph bitmap
- `internal/controller/assets/countdown_bg_1696.gif` -- countdown overlay background

Run the extraction script once before the first build (it expects `references/coolled-1248.apk`; see `CLAUDE.local.md` for how to obtain the APK). Both files are referenced via `//go:embed`, so `go build` fails if either is missing.

```bash
./scripts/extract-assets.sh
go build -o coolledux-controller ./cmd/coolledux-controller
go test ./...
```

## Configure

Copy the example config and edit for your device:

```bash
cp config.example.yaml config.yaml
```

Key knobs (full list in [`config.example.yaml`](config.example.yaml)):

| Section | Fields |
|---------|--------|
| `ble` | `device_name`, `device_mac`, `service_uuid`, `char_uuid`, `scan_timeout`, `reconnect_interval` |
| `display` | `columns`, `rows`, `default_brightness`, `default_font_size`, `default_color`, `default_speed` |
| `mqtt` | `broker`, `client_id`, `topic_prefix`, `ha_discovery_prefix`, `keepalive` |
| `api` | `listen`, `cors_origins` |
| `log` | `level`, `format` |

Env vars override config using the `COOLLEDUX_` prefix with underscores for nesting, e.g. `COOLLEDUX_BLE_DEVICE_MAC=01:00:00:FB:A4:16`.

## Run

### Foreground

```bash
./coolledux-controller --config config.yaml
```

Or run directly from source:

```bash
go run ./cmd/coolledux-controller --config config.yaml
```

Ctrl-C triggers the graceful shutdown path in `cmd/coolledux-controller/main.go`.

### Background

```bash
nohup ./coolledux-controller --config config.yaml > service.log 2>&1 &
```

### Stop

```bash
pkill -f '[c]oolledux-controller'      # SIGTERM, graceful
pkill -9 -f '[c]oolledux-controller'   # SIGKILL, if BLE hangs
```

### Check status

```bash
pgrep -af '[c]oolledux-controller'
```

### Docker

Compose bundles the service plus a Mosquitto broker. Requires `network_mode: host` and `privileged: true` for BLE access.

```bash
docker compose up -d
docker compose logs -f coolledux-controller
docker compose down
```

## Repository Layout

```
cmd/coolledux-controller/    Entrypoint (config load, DI, graceful shutdown)
cmd/probe-encode/            Dev utility: LZSS round-trip bisector
cmd/text-preview/            Dev utility: font baseline/ink metrics
internal/ble/                BLE client + chunked transport
internal/protocol/           Command codes, CRC32, LZSS, stream framing, program builders
internal/controller/         Orchestrator, state machine, countdown/stopwatch/scoreboard builders
internal/models/             DTOs and enums
internal/image/              Resize/rotate/flip + RGB444 column-major encoding
internal/text/               BDF rasterization and font registry
internal/api/                Fiber HTTP server, handlers, schemas
internal/mqtt/               MQTT client + Home Assistant discovery
internal/config/             Viper-based YAML + env config
scripts/                     extract-assets.sh, matrix-probe.py (REST pattern drawer)
docs/specs/                  Protocol specifications
```

See [`CLAUDE.md`](CLAUDE.md) for deeper implementation notes, critical firmware quirks, and code conventions.

## License

No license declared yet.
