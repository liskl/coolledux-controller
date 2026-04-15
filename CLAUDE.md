# CoolLEDUX Controller -- Go Service

A standalone Go service that controls a CoolLEDUX 16x96 full-color RGB LED matrix sign over Bluetooth Low Energy (BLE). Exposes a REST API for direct control and integrates with Home Assistant via MQTT Auto-Discovery.

**Target hardware:** JT_HW358.02 16x96 LED matrix (E-CrossStu / CoolLEDUX)
**Reference implementation:** Python SDK at `NunoMiguelVeloso/coolledux-controller` on GitHub
**Go module:** `github.com/liskl/coolledux-controller`
**Minimum Go version:** 1.25.5

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
    probe-encode/                   # LZSS round-trip bisector (dev utility)
    text-preview/                   # Prints baseline/ink metrics for each registered font
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
      controller.go                 # Orchestrator: state machine, program upload, overlay commands
      state.go                      # DeviceState, ProgramSendingState enums
      timecount.go                  # Countdown/stopwatch/scoreboard builders; APK 140-byte digit bitmap
      assets/                       # countdown_bg_1696.gif (APK-extracted, gitignored)
    models/
      device.go                     # DeviceInfo struct
      program.go                    # GraffitiProgram, AnimationProgram, TextProgram
      enums.go                      # TextShowMode, BorderMode, BorderType, FullColorType, FlipMode
      display.go                    # DrawItem
      response.go                   # ActionResponse, InfoResponse
    image/
      processor.go                  # Resize, rotate, flip, RGBA-to-DrawItem extraction
      gif.go                        # GIF frame extraction, duration parsing
      color.go                      # RGB888->RGB444 (47/14 transfer), column-major encoding
    text/
      renderer.go                   # Rasterize strings to RGB444 column-major bytes via font.Drawer
      fonts.go                      # Font registry; embeds spleen-8x16.bdf, 7x14B.bdf; basicfont.Face7x13
      font.go                       # Legacy 16x16 APK-extracted glyph helpers (unused by default path)
      bdf/                          # Embedded BDF files: spleen-8x16.bdf, 7x14B.bdf
      fonts/                        # unicode_16_bold.bin (APK-extracted, gitignored)
    api/
      server.go                     # Fiber HTTP server setup, route registration
      handlers.go                   # Route handlers for all endpoints
      middleware.go                 # Structured logging, error recovery, CORS
      schemas.go                    # Request/response JSON structs
    mqtt/
      client.go                     # MQTT connection, publish/subscribe, LWT, reconnect
      discovery.go                  # Home Assistant MQTT auto-discovery payload builders
      handlers.go                   # Command topic handlers (power, brightness, text, image, gif)
      entities.go                   # Entity definitions (light, binary_sensor)
    config/
      config.go                     # Viper-based config: YAML file + env vars + defaults
  scripts/
    extract-assets.sh               # Pull APK assets into internal/text/fonts and internal/controller/assets
    matrix-probe.py                 # Ad-hoc pattern drawing via REST (pixel/line/rect/poly)
  docs/specs/                       # Protocol specifications
  config.example.yaml               # Example configuration file
  Dockerfile                        # Multi-stage build
  docker-compose.yml                # Service + Mosquitto broker
```

Every package (except `cmd/probe-encode` and `cmd/text-preview`) ships with a matching `*_test.go`. Overall coverage sits near 86% (weakest spot: `internal/ble`, which talks to real BLE hardware).

### Key Dependencies

| Package | Purpose |
|---------|---------|
| `tinygo.org/x/bluetooth` | BLE client (Linux BlueZ) |
| `github.com/eclipse/paho.mqtt.golang` | MQTT v3.1.1 client |
| `github.com/gofiber/fiber/v2` | HTTP framework |
| `github.com/spf13/viper` | Configuration (YAML + env) |
| `github.com/disintegration/imaging` | Image resize/rotate/flip |
| `golang.org/x/image/font/basicfont` | Default monospace face (Face7x13, Plan 9 bitmap) |
| `github.com/zachomedia/go-bdf` | Parse embedded BDF bitmap fonts (Spleen 8x16, X11 7x14B) |
| `log/slog` (stdlib) | Structured logging |

---

## Configuration

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
  cors_origins: ["*"]

log:
  level: "info"
  format: "json"
```

Env vars override config: prefix `COOLLEDUX_`, underscores for nesting (e.g. `COOLLEDUX_BLE_DEVICE_MAC`).

---

## Build and Deployment

Two assets are extracted from the CoolLED 1248 APK at build time and are *not* committed (third-party origin):

- `internal/text/fonts/unicode_16_bold.bin` -- the legacy 16x16 glyph bitmap
- `internal/controller/assets/countdown_bg_1696.gif` -- the countdown overlay background

Run `scripts/extract-assets.sh` once (expects `references/coolled-1248.apk`; see `CLAUDE.local.md` for how to obtain it) before the first build. Both paths are covered by `//go:embed`, so the build fails noisily if either is missing.

```bash
./scripts/extract-assets.sh                                     # one-time, requires the APK
go build -o coolledux-controller ./cmd/coolledux-controller
go test ./...
```

Docker: multi-stage build (golang:1.25-alpine -> alpine:3.20 with bluez+dbus). Requires `network_mode: host` and `privileged: true` for BLE. The image build must also see the extracted assets, so run `extract-assets.sh` on the host before `docker compose build`.

### Dev utilities

Small helper binaries live under `cmd/`. None of them are part of the shipped service; they're for reverse-engineering and hand-testing:

- `cmd/probe-encode` -- bisects LZSS round-trip bugs by finding the shortest prefix that fails.
- `cmd/text-preview` -- prints baseline and ink metrics for every registered font.
- `scripts/matrix-probe.py` -- draws pixel/line/rect/poly patterns on the matrix via REST.

Three more dev binaries (`bletest`, `crctest`, `digit-dump`) are gitignored and only exist on developer machines.

---

## Running the Service

Foreground (Ctrl-C for graceful shutdown, which is wired in `cmd/coolledux-controller/main.go`):

```bash
./coolledux-controller --config config.yaml
# or from source:
go run ./cmd/coolledux-controller --config config.yaml
```

Background with logs:

```bash
nohup ./coolledux-controller --config config.yaml > service.log 2>&1 &
```

Stop:

```bash
pkill -f '[c]oolledux-controller'      # SIGTERM, graceful
pkill -9 -f '[c]oolledux-controller'   # SIGKILL, if BLE hangs
```

Check if it's running:

```bash
pgrep -af '[c]oolledux-controller'
```

Docker:

```bash
docker compose up -d     # starts service + Mosquitto
docker compose down      # stops both
docker compose logs -f coolledux-controller
```

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

4. **LZSS compression is required** for program uploads. Uncompressed data is accepted (ACKed) but not displayed correctly. The encoder must cap match length at `dist` (no self-referential matches): the firmware decoder doesn't handle them correctly and produces a phantom byte one past the reference.

5. **CRC32 uses 32 iterations per byte**, polynomial 0x4C11DB7, no final XOR. Output little-endian for control commands, big-endian in program start metadata.

6. **RGB444 transfer is piecewise**: `>=238->15, <=47->0, else (value-47)/14+1`. Not a bit shift. (Source: `TextEmojiManagerCoolLEDUX.java:396`.)

7. **Column-major pixel ordering.** Outer loop columns, inner loop rows. Index = `row * width + col`.

8. **Program wrapper:** 8 zero bytes + content count (1) + separator (0x00) + content data.

9. **XOR checksum** on data chunks (not CRC32). XOR of bytes from offset 1 through end of chunk data.

10. **BLE connection is flaky on Linux.** Adapter reset between sessions, 2s post-scan delay, and 3-attempt retry are all necessary.

11. **Send chunks in 20-byte MTU pieces** with ~50ms inter-piece delay.

12. **Escape applies to length bytes too.** The 2-byte big-endian length is part of the escaped region.

13. **Device quirk — uniform full column produces a phantom pixel.** When 16 identical pixel values fill a complete column, the firmware lights a stray pixel at (col+1, row 0). Reproduces with LZSS disabled too, so it's not our encoder. Doesn't affect normal text rendering (glyphs rarely have 16px-tall uniform strokes).

14. **Countdown/stopwatch/scoreboard overlays all share a content-type-plus-control-byte split.** The panel has to have a composite program uploaded (animation `0x03` + overlay content) before the control-byte commands (`0x0F`/`0x10`/`0x11`) do anything visible. Without an upload they ACK but render nothing — that's why scoreboard looked "broken" for so long. See `internal/controller/timecount.go` and `scoreboard.go` for the live builders.

15. **`timeCountMode` (offset 13 of content `0x0A`) distinguishes countdown from stopwatch.** APK default is `1` (count up, stopwatch); `DiscoverCountdownActivity` explicitly overrides to `0` (count down). `DiscoverStopwatchActivity` leaves the default. Hardcoding `0` turns a stopwatch into a stalled countdown with no visible change when start/stop/reset is sent.

16. **Scoreboard uses content type `0x0B`**, not `0x0A`. `CoolledUXUtils.getDataWithScoreBoardCombineProgram` (line 3647 in Java) is one of the two methods jadx bails on — use `baksmali` on `classes3.dex` and trace `CoolledUXUtils.smali` line 12685 for the real byte layout. Key gotcha: register `v13` is reassigned **twice**; the first (220-byte) value is only used on some device sizes, and the second (40-byte small-digit bitmap at smali line 480) is what 16x96 actually emits for the scoreTotalNum and time-digit slots. Read far enough to see both assignments.

17. **Scoreboard `SetScores` (`0x11 0x02`) takes four fields, not two.** APK signature `getScoreBoardSetCore(hostScore, visitScore, hostTotalScore, visitTotalScore)` — the two trailing bytes are uint8 period/set counters displayed above each main score. Zeroing them blanks those counters; omitting them (treating the command as 2-arg) still sends valid bytes but silently strips data.

18. **Scoreboard `SetTime` only latches while the clock is stopped.** On 16x96 the firmware ignores `0x11 0x03 hh mm flag` sent while the clock is running — the display stays at whatever value was latched at the last start. Proper sequence: `StartStop(false)` → `SetTime(...)` → `StartStop(true)`. Main scores via `SetScores` (`0x11 0x02`) update immediately regardless of run state; only the clock is picky.

19. **Two text-rendering paths coexist.** Default: BDF-driven `font.Drawer` rasterization via the registry in `internal/text/fonts.go` (Plan 9 7x13, X11 7x14B, Spleen 8x16). Legacy: the 16x16 APK glyph bitmap in `internal/text/font.go` (column-major, 2 bytes per column, MSB = row 0). The legacy path is not wired into `/display/text` by default; it's kept because the countdown builder and some probes still read from it.

20. **Raw-GIF content type `0x0C` is opt-in, not auto-detected.** Firmware v30+ decodes GIFs on-device via `0x0C` (APK `CoolledUXUtils.java:3103-3200`). Byte layout matches `0x03` except: extra reserved zero at offset 13 (region fields shift to offsets 14-21), and the payload is the GIF file verbatim with a 4-byte BE length prefix at offset 22. The APK gates on `CoolleduxDeviceVersion >= 30` read from BLE scan-record byte 21 (`DeviceManager.java:5633`); BlueZ on Linux exposes byte 21 via `btmon`, but `tinygo.org/x/bluetooth` gives us only parsed AD fields, so we can't replicate that check in the BLE client. `/display/gif` takes a `raw: true` flag; caller is responsible for knowing their firmware. Observed negative signature on the v10 test panel (2026-04-15): program-start packet is ACKed, but the first data chunk times out with no ACK (unlike `0x03` which ACKs every chunk), and the panel reverts to the default `"1. CoolLED"` rainbow scroll. Same "firmware fell back to default program" failure mode as every other case where content is ACKed-but-not-understood (layer_type=0, LZSS disabled, etc.).

21. **Color-mode command `0x13/0x03` has a conditional `i4` byte, and its presence changes animation family.** Layout is `[0x13][0x03][i3][i4 if present][i2][palette]`. Hardware-validated meanings (see `docs/specs/protocol-ble.md` "Color Mode and Speed"): `i4` absent → temporal brightness pulse (`i3=1` black-between-colors, `i3=4` continuous crossfade); `i4` present → spatial animation with `i4` as direction code (0=scroll R→L, 1=L→R, 2=B→T, 3=T→B, 4=converge, 5=diverge). The builder's int8 `-1` sentinel for "omit i4" is load-bearing; sending `0` instead would flip the animation family. `i2` equals palette position count (byte count / 2), redundant but required. Modes 3 and 4 are APK no-ops (empty palette) and the builder rejects them.
