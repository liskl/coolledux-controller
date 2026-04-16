# BLE Protocol Specification

This document is derived from the decompiled CoolLED 1248 Android app (`references/apk-decompiled/`, not checked in). Where claims have also been confirmed against a CoolLEDUX 16x96 (MAC `01:00:00:FB:A4:16`), they are marked **HW-verified**. Citations are `file:line` into the decompiled Java source.

Primary sources:
- `light/utils/CoolledUXUtils.java` (command builders, program envelope, CRC, LZSS)
- `light/utils/LightUtils.java` (byte/hex helpers, stream de-escape)
- `light/device/DeviceManager.java` (response parsing)
- `light/emoji/TextEmojiManagerCoolLEDUX.java` (RGB444 conversion)

## Connection Parameters

| Parameter | Value | Source |
|-----------|-------|--------|
| BLE Service UUID | `0000fff0-0000-1000-8000-00805f9b34fb` | `DeviceManager.java` |
| BLE Characteristic UUID | `0000fff1-0000-1000-8000-00805f9b34fb` | `DeviceManager.java` |
| Characteristic properties | read, write-without-response, notify | HW-verified |
| Default device name | `CoolLEDUX` | Device advertisement |
| MTU | 247 requested by APK; 23 (20-byte payload) observed with tinygo on Linux | HW-verified |

**BLE connection quirks (tinygo bluetooth on Linux/WSL2):**
- Reset adapter (`bluetoothctl power off/on`) between sessions for reliability
- 2-second pause after scan completes before calling Connect
- Connection retry (up to 3 attempts, 2s between) helps with flaky `le-connection-abort-by-local`
- Uses write-without-response (only mode tinygo supports on Linux)

## Packet Framing

Every packet sent to or received from the device uses the same stream frame. There is **no `[0x52,0x52]` BLE-layer header**; the APK writes frames directly to the characteristic.

```
[0x01] [length:2 BE] [escaped(payload)] [0x03]
```

Builder: `CoolledUXUtils.getSendDataWithInfo` at `CoolledUXUtils.java:4401`.

- `0x01` — start byte (unescaped)
- `length` — 2-byte big-endian, count of **unescaped** bytes in `[length || payload]`. Written by `LightUtils.getDataStringLength` at `LightUtils.java:181`.
- `escaped(...)` — the length bytes AND the payload are run through the escape filter together
- `0x03` — end byte (unescaped)

### Escape rules

Escape is applied to any byte with value `0x01`, `0x02`, or `0x03`:

```
byte B in {0x01, 0x02, 0x03}  ->  [0x02, B ^ 0x04]
```

Builder: `CoolledUXUtils.convertData` at `CoolledUXUtils.java:2677`.
Inverse: `LightUtils.recoverData` at `LightUtils.java:399` (also strips start/end bytes and the 2-byte length prefix).

### Length field is escaped

The escape pass runs over `[length || payload]` as a single list, so a length byte of `0x01` becomes `[0x02, 0x05]` on the wire. Receivers must de-escape before reading the length.

## Command Codes

Every command is built by appending the command byte and payload to a list, then passing through `getSendDataWithInfo` (stream-frame). No CRC or checksum is added for simple control commands.

### Complete catalog from the APK

| Hex | Name | Payload | Builder | Line | HW |
|-----|------|---------|---------|------|----|
| `0x01` | Music/rhythm data | `[count:1][freq_bytes...]` | `getMusicDataString` | 4311 | **not implemented — see "Intentionally unsupported" below** |
| `0x02` | Program start (with count) | `[CRC32:4 BE][rawLen:4 BE][index:1][count:1][showCount:1]` (+ optional trailer for 6-arg variant) | `getStartDataForProgram` (3-arg / 5-arg) | 4493 / 4505 | verified |
| `0x03` | Program data chunk | See "Program Upload" below | `getDataPacket` (prefix `"03"`) | 3001 | verified |
| `0x04` | Set brightness | `[value:1]` (0-255) | `getSetBrightness` | 4412 | verified |
| `0x05` | Power on/off | `[0x01]` on, `[0x00]` off | `getSwitchData` | 4572 | verified |
| `0x06` | Set rhythm type | `[type:1]` | `getSetRyhthmType` | 4457 | **not implemented — see "Intentionally unsupported" below** |
| `0x09` | Time sync | `[year-2000][month][day][weekday_iso_mon=1..sun=7][hour][min][sec]` | `getSynchronizeTime` | 4583 | verified |
| `0x0A` | Set timer switch | `[count:1] { [enable:1][hour:1][min:1][daysBitmask:1][power_on:1][0x00] } * count` | `setTimerSwitch` | 4868 | verified |
| `0x0B` | Get timer switch | Empty | `getTimerSwitch` | 4643 | verified |
| `0x0C` | Set mirror (boolean) | `[0x00]` off / `[0x01]` on | `getSetMirror` | 4427 | verified |
| `0x0C` | Set rotate (0..3) | `[mode:1]` | `setRotate` | 4861 | verified |
| `0x0D` | Check password | `[random:1] { digit ^ random } * n [xorChecksum:1]` | `getCheckPasswordData` | 2770 | verified (check path); see "Password Commands" |
| `0x0E` | Set password | `[random:1] { digit ^ random } * n [xorChecksum:1]` | `getSetPasswordData` | 4438 | verified (hardware round-trip); see "Password Commands" |
| `0x0F` | Countdown status | `[0x01]` | `getCountDownStatus` | 2811 | verified |
| `0x0F` | Countdown set value | `[0x02][hour:2 BE][min:2 BE][sec:2 BE]` (each as uint16) | `getCountDownReset` | 2789 | verified |
| `0x0F` | Countdown start/stop | `[0x03][0x01 or 0x00]` | `getCountDownStartOrStop` | 2799 | verified |
| `0x10` | Stopwatch status | `[0x01]` | `getStopwatchStatus` | 4565 | verified |
| `0x10` | Stopwatch reset | `[0x02]` | `getStopwatchReset` | 4546 | verified |
| `0x10` | Stopwatch start/stop | `[0x03][0x01 or 0x00]` | `getStopwatchStartOrStop` | 4553 | verified |
| `0x11` | Scoreboard status | `[0x01]` | `getScoreBoardStatus` | 4394 | verified |
| `0x11` | Scoreboard set scores | `[0x02][scoreA:2 BE][scoreB:2 BE][totalA:1][totalB:1]` | `getScoreBoardSetCore` | 4357 | verified |
| `0x11` | Scoreboard set time | `[0x03][hour:1][min:1][isTimer:1]` | `getScoreBoardSetTime` | 4368 | verified |
| `0x11` | Scoreboard start/stop | `[0x04][0x01 or 0x00]` | `getScoreBoardStartOrStop` | 4382 | verified |
| `0x13` | Set color | `[0x01][R4:nibble][GB:byte]` (RGB444) | `setColor` | 4706 | verified |
| `0x13` | Set color speed | `[0x02][speed:1]` | `setColorSpeed` | 4835 | verified (hardware probe 2026-04-15) |
| `0x13` | Set color mode (palette) | `[0x03][i3][i4?][i2][palette...]` — 31 preset cycling palettes | `setColorMode` | 4714 | verified (smali-grounded, hardware probe 2026-04-15); see "Color Mode and Speed" |
| `0x1A` | Program start (simple) | `[CRC32:4 BE][rawLen:4 BE][index:1]` | `getStartDataForProgram` (2-arg) | 4484 | not used in current Go service |
| `0x1C` | Drive state set | `[0x01][state:1]` | `getSetDriveState` | 4419 | **not supported on 16x96** (probed 2026-04-16, see "Drive State" note) |
| `0x1C` | Drive state get | `[0x02]` | `getDriveState` | 4277 | **not supported on 16x96** (no response; see "Drive State" note) |
| `0x1E` | Toggle device info flag | `[0x01=show-device-id \| 0x02=remote-enable][0 \| 1]` | `setDeviceInfo` | 4843 | verified (hardware 2026-04-16); subtype 0x03 is iLedClock-only |
| `0x1F` | Get device info | Empty | `getDeviceInfo` | 4165 | verified |
| `0xFD` | OTA version query | Empty | `getDeviceOTAVersion` | 4171 | untested |
| `0xFE` | OTA upgrade start | `[CRC32:4 BE][rawLen:4 BE][chunkSize:1][firstChunk...]` | `getStartDataForOtaUpgrade` / `getStartOTAUpdate` | 4473 / 4538 | untested |
| `0xFF` | OTA data chunk | Same chunk framing as `0x03` (see below) | `getDataPacket` (prefix `"ff"`) | 4331 | untested |

### Not present in APK, but observed on hardware

- `0x07` (observed cycling through 10 program slots during earlier testing). No builder in the APK; the APK changes the active channel via `0x02` program-start with a new index. If `0x07` is a firmware feature it is undocumented by the app. Treat as tentative.
- `0x08` — no builder. Our earlier docs listed this as `PROGRAM`; the APK uses `0x02` / `0x1A` instead.

### Intentionally unsupported

The following APK commands relate to the device's microphone/rhythm feature and are **not implemented in this Go service** and will not be. This is a deliberate scope decision, not a TODO.

- `0x01` Music/rhythm data (`getMusicDataString`, `CoolledUXUtils.java:4311`) — streams audio-frequency data to the device for rhythm visualization.
- `0x06` Set rhythm type (`getSetRyhthmType`, `CoolledUXUtils.java:4457`) — selects a rhythm visualization mode.
- The `mic_on_off` (index 5) and `mic_mode` (index 6) fields in the `0x1F` device-info response are parsed for completeness but no setter is exposed.

**Why:** the 16x96 target hardware (`mic_supported = 0` in its `0x1F` response) has no microphone; the commands are ACKed but do nothing. There is no use case for this service to drive audio-reactive visualizations, and supporting it would mean carrying an audio pipeline and frequency analyzer that no deployment needs. Any future rhythm support belongs in a separate tool, not here.

If you are reading this because you want to add music visualization: don't add it to this service. Fork, or build a sibling binary that imports `internal/ble` and `internal/protocol`.

## CRC32

Polynomial `0x4C11DB7` (decimal 79764919). Initial value `0xFFFFFFFF` (signed `-1` in the Java source). No final XOR. 32 iterations per byte (bitwise, not tabled).

Source: `CrcCode.getCrc32CheckCode2` at `CoolledUXUtils.java:2328`.

A tabled variant (`getCrc32CheckCode` at `:2317`) also exists but is not the one used by `getCrcCode` — that always uses `getCrc32CheckCode2`.

Output encoding: 4 bytes via `LightUtils.getHexListStringForIntWithFourByte` — MSB first (big-endian) in the byte list. That is the encoding that goes on the wire in program-start packets.

```go
func CRC32(data []byte) uint32 {
    crc := uint32(0xFFFFFFFF)
    for _, b := range data {
        xbit := uint32(0x80000000)
        tmp := uint32(b) & 0xFF
        for i := 0; i < 32; i++ {
            if crc&0x80000000 != 0 {
                crc = (crc << 1) ^ 0x4C11DB7
            } else {
                crc = crc << 1
            }
            if tmp&xbit != 0 {
                crc ^= 0x4C11DB7
            }
            xbit >>= 1
        }
    }
    return crc
}
```

## LZSS Compression

| Parameter | Value |
|-----------|-------|
| Window size (N) | 512 |
| Lookahead size (F) | 18 |
| Match threshold | 2 |
| NIL sentinel | 512 |

Source: `LzssCompress` nested class at `CoolledUXUtils.java:2356`.

Flag byte packs 8 ops, LSB first: bit `1` = literal (1 byte follows), bit `0` = match (2 bytes follow). Match encoding: `byte0 = pos & 0xFF`, `byte1 = ((pos >> 4) & 0xF0) | (len - 3)`.

LZSS is mandatory for program data. The device accepts uncompressed data (ACKs the packets) but renders default text instead.

## Program Upload

Flow assembled in `getDataResult` at `CoolledUXUtils.java:3061` (and its overloads at `:3071`, `:3081`, `:3092`).

### 1. Build content block (example: graffiti/image, content type `0x02`)

Builder: `getDataWithGraffitiCombineProgram` at `CoolledUXUtils.java:3532`.

```
Offset  Size  Field                       Notes
------  ----  --------------------------  -----------------------------
0       4     Total length (incl. these)  BE uint32 = inner.size() + 4
4       1     Content type                0x02 (graffiti)
5       7     Reserved                    0x00 * 7
12      1     Layer type                  from CoolleduxGraffitiProgramContent.layerType
13      2     Start column                BE uint16
15      2     Start row                   BE uint16
17      2     Show width                  BE uint16 (e.g. 96)
19      2     Show height                 BE uint16 (e.g. 16)
21      1     Display mode                1 = static
22      1     Speed                       1-10
23      1     Stay time                   0 = infinite
24      4     Image data length           BE uint32
28      N     Image data                  column-major RGB444 (2 bytes/pixel)
```

`layerType` is copied verbatim from the program object; the APK does not force it. Empirically, a value of `0` causes the device to display default text on the 16x96 firmware, so set it to `1` for image content. (See `program_upload_findings.md`.)

Other content types follow the same `[totalLen:4][typeByte][7 zero bytes][layerType][...fields...]` pattern:

| Type | Builder | Source |
|------|---------|--------|
| `0x01` text | `getDataWithTextContentProgramContent` | `CoolledUXUtils.java:3897` |
| `0x02` graffiti | `getDataWithGraffitiCombineProgram` | `:3532` |
| `0x03` animation (frame-by-frame) | `getDataWithAnimationCombineProgram` | `:3147` |
| `0x04` frame | `getDataWithFrameProgramContent` | `:3304` |
| `0x05` text auto-color | `getDataWithTextAutoColorProgramContent` | `:3655` |
| `0x06` text custom-color | `getDataWithTextCustomColorProgramContent` | `:3920` |
| `0x0A` time-count overlay | `getDataWithTimeCountCombineProgram` | `:3946` |
| `0x0B` scoreboard overlay | `getDataWithScoreBoardCombineProgram` | `:3647` (jadx fails; baksmali line 12685 of `CoolledUXUtils.smali`) |
| `0x0C` raw GIF (firmware >= v30) | `getDataWithAnimationCombineProgram` (GIF overload) | `:3103` |

### Raw GIF content (type `0x0C`, firmware v30+)

Four builder overloads live at `CoolledUXUtils.java:3103-3230`. Byte layout differs from `0x03` in two ways: the layerType slot is followed by an extra reserved zero byte before the region fields, and the payload is the **entire GIF file verbatim** (not pre-extracted frames).

```
Offset  Size  Field                       Notes
------  ----  --------------------------  -----------------------------
0       4     Total length (incl. these)  BE uint32 = inner.size() + 4
4       1     Content type                0x0C (raw GIF)
5       7     Reserved                    0x00 * 7
12      1     Layer type                  must be 0x01 (same gotcha as 0x02/0x03)
13      1     Reserved                    0x00
14      2     Start column                BE uint16
16      2     Start row                   BE uint16
18      2     Show width                  BE uint16
20      2     Show height                 BE uint16
22      4     GIF length                  BE uint32
26      N     GIF file bytes              verbatim (GIF87a/GIF89a header intact)
```

Wrap, compress, and upload the same way as any other content (`WrapProgramPayload` → LZSS → `0x02` 3-arg program start with CRC32). Program-start transport is identical to `0x03`.

**Version gate.** The APK (`:2826`) only picks `0x0C` when `DeviceManager.CoolleduxDeviceVersion >= 30 && < 255`, reading the version from BLE scan record byte 21 (`DeviceManager.java:5633`). Older firmware ACKs the upload but renders nothing. BlueZ on Linux hands us parsed advertisement fields, not the raw scan record, so we can't replicate that check directly; our `/display/gif` endpoint exposes an opt-in `raw: true` flag instead, and the caller is responsible for knowing their device firmware.

**Encrypted variant (`getDataWithAnimationCombineProgramEncryped`, `:3202`).** First 32 bytes of the GIF are XOR'd with `0xDA`. Only used for app-bundled "material" GIFs; user-supplied GIFs are sent plain. Not implemented here.

For the countdown UI the APK uploads a **composite program** with two content blocks: a `0x03` animation (18-frame purple frame + hourglass, from `ic_countdown_bg_animation_1696.gif`) and a `0x0A` time-count overlay. The `0x0A` body is:

```
offset  size  value
4       1     0x0A
5       7     zero padding
12      1     layerType (1)
13      1     timeCountMode — 0=count down (countdown), 1=count up (stopwatch). APK default is 1; DiscoverCountdownActivity explicitly overrides to 0. Missing this byte keeps the digits frozen.
14      2     numHeight (10)
16      2     numWidth (7)
18      2     digitBitmapLen (140)
20      N     digitBitmap (10 digits × 14 bytes — 7 cols × 2 bytes per col, MSB = row 0)
...     10    hour: color(2) + col(2) + row(2) + w(2) + h(2)
...     10    spaceHour: color + col + row + w + h
...     2+M   separatorLen + separator (2+4 bytes on 16x96: "51, 0, 51, 0")
...     10    minute: same shape
...     10    spaceMinute
...     2+M   separatorLen + separator (reused)
...     10    seconds
```

Device-specific bitmaps (16x32, 16x64, 16x144, 16x192, 24x*, 32x*) differ in both dimensions and byte count; see `CoolledUXUtils.smali` `getDataWithTimeCountCombineProgram` registers v4/v7/v13/v15/v17/etc. for the verbatim constants. The jadx-decompiled Java mangles the control flow; use baksmali to trace which register applies per `DEVICE_ROW`/`DEVICE_COLUMN`.

#### Scoreboard overlay (`0x0B`)

Built by `CoolledUXUtils.getDataWithScoreBoardCombineProgram` (smali line 12685 — jadx bails with "Code decompiled incorrectly" at 1264 instructions; baksmali is the source of truth). Driven by command family `0x11` (status / setScores / setTime / startStop). The APK's `DiscoverScoreboardActivity` uploads a composite program (animation `0x03` + scoreboard content `0x0B`).

The `0x0B` body on 16x96 (`DEVICE_ROW=16 && DEVICE_COLUMN>=96` branch):

```
offset  size   value
4       1      0x0B
5       7      zero padding
12      1      layerType (1)
13      1      zero
14      2      scoreNumHeight (10)       ← main team-score glyph height
16      2      scoreNumWidth (7)         ← main team-score glyph width
18      2      scoreDigitsBitmapLen (140)
20      140    scoreDigitsBitmap          ← same 7×10 hollow digits as countdown/stopwatch (v7)
...     2+8    hostScore color(2) + col(2) + row(2) + w(2) + h(2)  [col=14 row=4 w=21 h=10]
...     2+8    visitScore color + pos                              [col=61 row=4 w=21 h=10]
...     2      scoreTotalNumHeight (5)   ← small period-counter glyph height
...     2      scoreTotalNumWidth (4)    ← small period-counter glyph width
...     2      smallDigitsBitmapLen (40)
...     40     smallDigitsBitmap          ← 3-col glyph + 1 spacer per digit, 1 byte per col, 5-row MSB-packed
...     2+8    totalHost color + pos                               [col=41 row=2 w=4 h=5]
...     2+8    totalVisit color + pos                              [col=51 row=2 w=4 h=5]
...     2      timeNumHeight (5)         ← clock digit height (same as total)
...     2      timeNumWidth (4)          ← clock digit width
...     2      timeDigitsBitmapLen (40)
...     40     timeDigitsBitmap           ← same 40-byte small-digit bitmap (reused)
...     2+8    minute color + pos                                  [col=39 row=11 w=8 h=5]
...     2+8    spaceMinute color + pos (colon slot)                [col=47 row=11 w=1 h=5]
...     2      colonBitmapLen (1)
...     1      colonBitmap                ← single byte 0x50 (dots at rows 1 & 3 of 5-row cell)
...     2+8    seconds color + pos                                 [col=49 row=11 w=8 h=5]
```

Gotcha that burned us: `v13` in the smali is **reassigned twice** — once early (line 127) as a 220-byte candidate for score-digit selection on other panel sizes, and again at line 480 as the 40-byte small-digit bitmap. By the time the scoreTotalNum and time-digit selectors run, `v13` is the 40-byte version. Reading the first assignment and stopping there produces a program that uploads successfully but renders garbled clock/counter glyphs.

### 2. Wrap in program envelope

Builder: `getDataWithProgram` at `CoolledUXUtils.java:3578`.

```
[0x00 * 8] [contentCount:1] [0x00] [content1] [content2] ...
```

### 3. LZSS-compress the envelope

`LzssCompress.getLzssCompressData` on the envelope from step 2.

### 4. Send the start packet

For the current Go service (3-arg form, `CoolledUXUtils.java:4493`):

```
stream_frame([0x02] [CRC32:4 BE] [rawLen:4 BE] [index:1] [count:1] [showCount:1])
```

- `CRC32` is computed over the **uncompressed** program envelope (step 2 output).
- `rawLen` = length of the uncompressed envelope.
- CRC bytes are big-endian here (unlike some simple ack payloads).
- A simpler variant (`0x1A`, 1-arg) exists at `:4484` with `[CRC32:4][rawLen:4][index:1]`. Either is valid; use whichever the firmware accepts.

### 5. Send data chunks

Builder: `getDataPacket` at `CoolledUXUtils.java:3001`. Default chunk size is 1024 bytes; an overload at `:3031` takes a caller-specified `UX_PACKAGE_SIZE` (the device can advertise one in the `0x1F` response, see below).

For each chunk of the LZSS-compressed data:

```
stream_frame([0x03]
             [0x00]
             [totalCompressedLen:4 BE]
             [chunkIndex:2 BE]
             [chunkLen:2 BE]
             [chunkData...]
             [xorChecksum:1])
```

`xorChecksum` is `convertEnd` (`CoolledUXUtils.java:2697`) — the XOR of every byte from `0x00` (the padding byte after `0x03`) through the last chunk-data byte. It does **not** include the `0x03` command byte.

Send each stream-framed chunk in 20-byte MTU pieces with ~50 ms delay between pieces. Wait for the chunk ACK before sending the next chunk.

### OTA chunks

Identical packet shape but with `0xFF` command prefix instead of `0x03`. See `getOTAUpdate` at `:4331`.

## Image Data Encoding

### RGB888 → RGB444 per-channel transfer

Source: `TextEmojiManagerCoolLEDUX.rgb444Transfer` at `TextEmojiManagerCoolLEDUX.java:396`.

```go
func rgb444Transfer(v uint8) uint8 {
    if v >= 238 { return 15 }
    if v <= 47  { return 0 }
    return uint8((int(v) - 47) / 14 + 1)
}
```

Thresholds and divisor are **not** the same as a bit shift. Earlier internal docs used `<=30` and `/15`; both are wrong.

### Per-pixel byte layout (RGB444)

Source: `getColorDataWithColorWithRGB444Transfer` at `TextEmojiManagerCoolLEDUX.java:93`.

```
byte 0: 0x0R          (high nibble = 0, low nibble = red 4-bit)
byte 1: (G << 4) | B  (high nibble = green, low nibble = blue)
```

### Pixel ordering

Column-major: outer loop columns (0..width-1), inner loop rows (0..height-1). The source index is `row * width + col`. See `getDrawListDataFColor` at `CoolledUXUtils.java:4267`.

## Color Mode and Speed (`0x13/0x02`, `0x13/0x03`)

Two control commands that configure automated color cycling. Jadx can't cleanly recover the `setColorMode` control flow (register reuse, multi-entry `goto` labels), so the table below is derived from the smali at `classes3.dex:com/jtkj/led1248/light/utils/CoolledUXUtils.smali:20288` rather than the `.java` at `CoolledUXUtils.java:4714`. The palette fields `colorMode1` through `colorMode31` are declared at `CoolledUXUtils.java:34-63` and are byte-faithful.

### `0x13/0x02` — Set color speed

Trivial: one byte of speed, stream-framed. Built at `CoolledUXUtils.java:4835`.

```
[0x13][0x02][speed:1]
```

`speed` is 1-10 per the global default convention (the method passes the caller's integer through unchanged). Higher values cycle faster.

### `0x13/0x03` — Set color mode

Structure after the `0x13 0x03` header:

```
[i3:1] [i4:1, present only if i4 >= 0] [i2:1] [palette bytes...]
```

- `i3` — probable meaning: frame/repeat group. Values observed: 0, 1, 2, 3, 4.
- `i4` — probable meaning: sub-effect modifier (color channel, strobe style). Present as a byte only when `>= 0`. Skipping it entirely (not just sending 0) changes the packet length — the firmware distinguishes "no i4" from "i4 = 0".
- `i2` — probable meaning: duration / period / stride. Values observed: 0, 2, 6, 8, 18, 30, 48, 90.
- `palette` — comma-split RGB444 bytes from the `colorModeN` static fields. Odd byte = `0x0R`, even byte = `(G << 4) | B` (see RGB444 section above).

#### Mode table

Every row below is grounded in the smali trace at `setColorMode` labels `:goto_f6`, `:goto_46`, `:goto_65`, `:goto_67`, `:goto_95`, `:goto_9f`, `:goto_1c`. Several palettes alias each other (e.g. `colorMode1 == colorMode2 == colorMode4 == colorMode5 == colorMode6`); the "Palette source" column names the register-loaded string, not necessarily the `colorModeN` matching the mode index.

| Mode | `i3` | `i4` | `i2` | Palette source | Bytes |
|-----:|:---:|:----:|:---:|----------------|------:|
|   1  |  2  |  0   | 90  | `colorMode1`   | 180 |
|   2  |  2  |  1   | 90  | `colorMode1`   | 180 |
|   3  |  0  |  —   |  0  | (empty)        |   0 |
|   4  |  0  |  —   |  0  | (empty)        |   0 |
|   5  |  2  |  4   | 90  | `colorMode1`   | 180 |
|   6  |  2  |  5   | 90  | `colorMode1`   | 180 |
|   7  |  2  |  0   | 12  | `colorMode7` (inline) | 24 |
|   8  |  2  |  1   | 12  | `colorMode7`   |  24 |
|   9  |  2  |  0   | 18  | `colorMode9`   |  36 |
|  10  |  2  |  1   | 18  | `colorMode10`  |  36 |
|  11  |  2  |  2   | 18  | `colorMode9` (aliases `colorMode11`) | 36 |
|  12  |  2  |  3   | 18  | `colorMode9` (aliases `colorMode12`) | 36 |
|  13  |  1  |  —   |  6  | `colorMode13`  |  12 |
|  14  |  1  |  —   |  2  | `colorMode14` (inline) |   4 |
|  15  |  3  |  4   | 18  | `colorMode9` (aliases `colorMode15`) | 36 |
|  16  |  3  |  5   | 18  | `colorMode9` (aliases `colorMode16`) | 36 |
|  17  |  2  |  0   | 30  | `colorMode17` (inline) | 60 |
|  18  |  2  |  1   | 30  | `colorMode17` (aliases `colorMode18`) | 60 |
|  19  |  2  |  0   |  8  | `colorMode19`  |  16 |
|  20  |  2  |  1   |  8  | `colorMode20`  |  16 |
|  21  |  2  |  0   |  8  | `colorMode21`  |  16 |
|  22  |  2  |  1   |  8  | `colorMode22`  |  16 |
|  23  |  2  |  0   |  8  | `colorMode23`  |  16 |
|  24  |  2  |  1   |  8  | `colorMode24`  |  16 |
|  25  |  2  |  0   |  8  | `colorMode25`  |  16 |
|  26  |  2  |  1   |  8  | `colorMode26`  |  16 |
|  27  |  2  |  0   |  8  | `colorMode27`  |  16 |
|  28  |  2  |  1   |  8  | `colorMode28`  |  16 |
|  29  |  2  |  0   | 48  | `colorMode29`  |  96 |
|  30  |  2  |  1   | 48  | `colorMode30`  |  96 |
|  31  |  4  |  —   |  6  | `colorMode13` (aliases `colorMode31`) | 12 |
| other|  0  |  —   |  0  | (empty)        |   0 |

Modes 3 and 4 fall through every enumerated branch to the `:cond_f0` empty default (zero palette, zero params), same as out-of-range inputs. They're effectively no-ops that the app should never send; the corresponding Java cases in `setColorMode` don't exist.

`colorMode21`'s declaration at `CoolledUXUtils.java:47` has a trailing comma (`"...00,30,00,10,"`). `getSplitDataStringByDot` emits an extra empty-string entry that likely produces a single `0x00` trailing byte; verify on hardware if it ever matters.

#### Palette patterns

Pattern constants live at `CoolledUXUtils.java:34-63`. A few worth understanding visually:

- **`colorMode1` (180 bytes)** — full RGB444 hue sweep: red → red/green ramp → yellow → yellow/blue ramp → magenta → magenta/red ramp. Six rows of 30 bytes each. Aliased by `colorMode2`, `colorMode4`, `colorMode5`, `colorMode6`.
- **`colorMode9` (36 bytes)** — six distinct solid colors (red, magenta, blue, cyan, green, yellow), each repeated three times. `colorMode11`, `colorMode12`, `colorMode15`, `colorMode16` have the same 36-byte layout but the second row in `colorMode10` is doubled instead of stepping to blue, giving a slightly different cycle.
- **`colorMode14` (4 bytes)** — `0F,00,00,0F`, a pure red ↔ blue toggle. With `i2=2` this is the fastest preset.
- **`colorMode17` (60 bytes)** — six 10-byte rows, each ending in 4 black bytes (`00,00,00,00`), so each color spends 40% of its row as a gap. Produces discrete color-plus-blank pulses.
- **`colorMode19`..`colorMode28` (16 bytes each)** — single-channel fade ramps. 19/21/23 are high-to-low on R/G/B respectively; 20/22/24 are low-to-high; 25-28 mix channels for white/secondary fades.
- **`colorMode29`, `colorMode30` (96 bytes)** — six 16-byte rows, each a fade ramp. `colorMode29` ramps high→low, `colorMode30` ramps low→high.

### Validated semantics (hardware-confirmed 2026-04-15)

Probe results from bead `o2l` on the 16x96 v10 firmware pinned down all three parameter bytes:

- **`i4` is the animation direction code.** Mutually exclusive with the temporal-fade family (see `i3` below).

  | `i4` | Direction |
  |-----:|-----------|
  |  0   | scroll right → left |
  |  1   | scroll left → right |
  |  2   | scroll bottom → top |
  |  3   | scroll top → bottom |
  |  4   | converge to center (both sides in) |
  |  5   | diverge from center (both sides out) |

  When `i4` is present the palette is laid out spatially — one color per column (for horizontal scrolls) or per row (for vertical scrolls) — and shifted every speed tick.

- **`i2` is the palette position count** (`len(palette)/2`, since each position is two RGB444 bytes). Redundant information the firmware still expects explicitly; sending a mismatched value likely desyncs the renderer. Our builder computes it directly from the palette bytes.

- **`i3` selects the transition style**, and critically determines whether `i4` is present at all:

  | `i3` | `i4` byte | Observed behavior |
  |-----:|:---------:|-------------------|
  |  1   | omitted   | Temporal brightness pulse: black → full color → black, looped through the palette one color at a time. ~7 brightness steps each direction. |
  |  2   | present   | Spatial scroll / converge / diverge per `i4` (the most common mode). |
  |  3   | present   | Spatial (same family as `i3=2`); exact i3=2 vs i3=3 visual difference not isolated because every `i3=3` mode also has a distinct `i4` value. |
  |  4   | omitted   | Continuous crossfade: one color fades directly into the next, no black gap. |

  Presence vs absence of the `i4` byte is the single switch between spatial-scroll and temporal-fade animation families. That's why the builder emits zero length bytes for `i4` when the table marks it `-1` — sending `0` instead would flip the animation type.

### Remaining unknowns

- Fine distinction between `i3=2` and `i3=3` (both spatial): every `i3=3` mode in the APK combines with a different `i4` than its `i3=2` sibling, so an APK replay can't A/B them cleanly. Would need a custom builder that holds palette + `i4` constant and sweeps `i3`.
- Whether `i3` ≥ 5 is meaningful. The APK never emits values above 4.
- Whether `i4` values ≥ 6 are accepted or error out; the APK emits at most 5.
- Speed scaling formula — we confirmed speed=1 is clearly slower than speed=10 on mode 1, but didn't measure the exact ms-per-step function.

## Device Info Toggles (`0x1E`)

Two boolean settings surfaced by the CoolLEDUX app's `SettingsCoolleduxFragment` and confirmed writable on 16x96:

| Subtype | Meaning | APK UI binding |
|--------:|---------|----------------|
| `0x01`  | Show device ID on panel | `show_device_id_cb` at `SettingsCoolleduxFragment.java:200` |
| `0x02`  | Enable remote control    | `remote_cb` at `SettingsCoolleduxFragment.java:195` |
| `0x03`  | (iLedClock product line) | Not wired on CoolLEDUX — don't send |

Command body is `[0x1E, subtype, 0x00 \| 0x01]` stream-framed, no CRC. The device echoes a response frame with type byte `0x1E` (shape `[0x1E, some_byte]` per `DeviceManager.java:4418`).

### Hardware findings (2026-04-16, v10 firmware)

- Both subtypes are accepted and round-trip through the `0x1F` device-info response. Baseline `show_device_id=true, remote_enabled=true` → `false` → `true` observed on both fields during probing, so there is no sticky one-way interlock (an earlier-looking "stuck false" pattern on remote_enabled turned out to be a state-read race, see below).
- No visible change on the panel while a program is actively displaying content. These toggles almost certainly affect the default/idle scroll (the `"1. CoolLED"` fallback) rather than an active program's rendering. Verifying that specifically would require clearing the program and observing idle state.
- The `0x1E` response frame is not silently consumed anywhere — it arrives on the same notification pipe as the `0x1F` device-info response. Our current `GetDeviceInfo` parser rejects any frame whose first byte is not `0x1F`, so a stale `0x1E` ACK from a previous toggle can cause an unrelated `/device/info` call to return `"not a device info response: type 0x1E"`. Implementation bead `5i7` needs to either filter `0x1E` frames at the transport layer or make the device-info parser tolerant of them.

## Password Commands (`0x0D`, `0x0E`)

The CoolLED 1248 app sends XOR-encoded hex-digit passwords via `getCheckPasswordData` (`CoolledUXUtils.java:2770`) and `getSetPasswordData` (`:4438`). Layout after the stream frame:

```
[cmd][xorKey][nibble[0]^xorKey]...[nibble[N-1]^xorKey][xorChecksum]
```

- `cmd`: `0x0D` for check, `0x0E` for set.
- `xorKey`: random byte chosen per call.
- `nibble[i]`: the i-th password character parsed as a single hex digit (0-9, a-f, case-insensitive), so the allowed alphabet is hex only.
- `xorChecksum`: XOR of every encoded nibble byte (NOT including `cmd` or `xorKey`).

No CRC, no BLE-header wrapper. Response shape is `[cmd, status]` where `status == 0` means success; any nonzero byte is a rejection.

### Hardware findings (2026-04-16, 16x96 v10)

- The check path (`0x0D`) is fully verified end-to-end against a panel with a real stored password ("123456"): the builder-encoded packet decodes on the firmware side back to the exact nibbles we input, and the device's compare matches the stored value.
- Concrete round-trip: `123456` → 200 verified; `abcdef`, `654321` → 401 rejected; any wrong length → 401 rejected.
- Case-insensitive: `ABCDEF` and `abcdef` produce identical on-wire packets and identical device responses (both rejected in the same run), consistent with parsing each char as a single hex digit regardless of case.
- Before a password was stored on this panel, **4- and 6-character hex inputs of any value returned 200** and other lengths returned 401. Reading: the firmware's length gate runs first (user-PIN is 4 chars, admin-PIN is 6), and with no stored value the comparison is a no-op. Once a 6-character password was set via the APK, only that exact value (`123456`) passed.
- The set path (`0x0E`) is also directly verified: wrote `654321` via `POST /device/:id/password/set`, then `check 654321` returned 200 and `check 123456` (the previous password) returned 401. State transition is atomic from the client's perspective — the set response arrives before the next check sees the new value.
- **There is no BLE-level password-clear mechanism.** The APK has no UI flow for it (`PasswordSetDialog.java:54` hard-requires a 6-char input before send), no `clearPassword`/`resetPassword` method, and no factory-reset BLE command anywhere in `CoolledUXUtils.java`. The only way back to the "no password" state is the **physical reset button on the controller held for 5 seconds**, which clears the stored password (and resets brightness to 255; other settings also reset). After a physical reset the firmware accepts any 6-char hex value for 0x0D — consistent with the "no password stored" reading. If a panel is going to be exposed to untrusted clients, treat `POST /device/:id/password/set` as a one-way trap that only the physical reset button can undo.

## Drive State (`0x1C/0x01`, `0x1C/0x02`) — not supported on 16x96

The APK ships a "drive state" UI (`ILedCarDriveFragment.java:25-30`) for car-mounted signs: NORMAL, LEFT, RIGHT, BACK, PARK, EMERGENCY. The builders live at `CoolledUXUtils.java:4277` (get) and `:4419` (set); byte layout is trivial — set is `[0x1C, 0x01, state]`, get is `[0x1C, 0x02]`, both stream-framed with no CRC.

**Hardware validation (2026-04-16, JT_HW358.02 16x96 v10 firmware):**

- `get` (0x1C/0x02) times out after 5 s with no response bytes.
- `set` (0x1C/0x01) for states 0 (NORMAL), 1 (LEFT), and 5 (EMERGENCY) produces no visible change on a panel displaying static text. States 2, 3, 4 were not tested but a dead command code is unlikely to have per-state behavior.

The CoolLEDUX sign packs the same BLE service UUIDs as the iLedCar / iDevilEyes family but does not implement this overlay. Our Go service intentionally does not expose builders, controller methods, or endpoints for `0x1C` — if we ever add iLedCar support under `23i`, regenerate the builders from the APK rather than carrying dead code. Bead `v8w` (discovery) closed with this finding; bead `jbt` (implementation) closed as not-applicable.

## Device Info Response (`0x1F`)

Parsed in `DeviceManager.java` (search for `getCoolLEDUXDeviceInfo`). The first byte of the response payload is `0x1F` echoed back.

| Index | Field | Type | Notes |
|-------|-------|------|-------|
| 0 | command | u8 | `0x1F` |
| 1 | power | u8 | 0=off, 1=on |
| 2 | brightness | u8 | 0-255 |
| 3 | rotate/mirror | u8 | 0-3 (0=none, 1=H, 2=V, 3=both) |
| 4 | mic_supported | u8 | 0/1 |
| 5 | mic_on_off | u8 | 0/1 |
| 6 | mic_mode | u8 | rhythm mode |
| 7 | show_device_id | u8 | 0/1 |
| 8 | max_program_number | u8 | number of program slots |
| 9 | remote_enable | u8 | 0/1 |
| 10-18 | reserved | bytes | not parsed by the APK |
| 19-20 | package_size | u16 BE | present only if response is 21 bytes. `0` → default 1024. Caps the program chunk size used in step 5. |

## Notifications / Response Types

Stream-framed just like outgoing packets. First byte identifies the type.

| Code | Meaning | Notes |
|------|---------|-------|
| `0x02` | Program start / chunk ACK | byte 1: 0=success, 1=continue, else=error |
| `0x03` | (Used outbound as chunk; not observed inbound) | |
| `0x04` | Brightness echo | byte 1 = brightness |
| `0x05` | Power echo | byte 1: 0=off, 1=on |
| `0x09` | Time sync ACK | byte 1: 0=success |
| `0x0A` | Set-timer ACK | byte 1: 0=success |
| `0x0B` | Get-timer response | byte 1 = count, then 6 bytes per slot: `[enable, hour, min, daysMask, power_on, 0x00]` |
| `0x0C` | Mirror/rotate echo | byte 1 = mode |
| `0x0D` | Password check result | byte 1: 0=correct |
| `0x0E` | Set-password ACK | byte 1: 0=success |
| `0x1C` | Drive state response | sub-code + state |
| `0x1E` | Set-device-info ACK | sub-code specific |
| `0x1F` | Device info (see above) | |
| `0xFD` | OTA version | |
| `0xFE` | OTA packet ACK | |

## Device-Specific Constants (16x96)

| Constant | Value | Source |
|----------|-------|--------|
| Display width | 96 columns | `DeviceManager.DEVICE_COLUMN` |
| Display height | 16 rows | fixed |
| Device type code | 37 (`COOLEDUX_16X_COLUMN_LESS_AND_EQUAL_96`) | `DeviceManager.java` |
| Max brightness | 255 | u8 |
| Default program chunk size | 1024 | `getDataPacket` no-arg variant |
| Default BLE MTU | 23 (20-byte payload) observed; APK may request up to 247 | HW / `DeviceManager.java:63` |

## Summary of corrections vs. older internal notes

| Claim in older docs | Actual (per APK) |
|---------------------|------------------|
| `CHANNEL = 0x07` | Not a builder in the APK. Channel switching is done via `0x02` program-start with the target program `index`. `0x07` responses on hardware may be firmware-private. |
| `PROGRAM = 0x08` | Not used. Program start is `0x02` (3- or 5-arg variant) or `0x1A` (simple). |
| RGB444 transfer: `<=30 → 0`, `(v-30)/15 + 1` | `<=47 → 0`, `(v-47)/14 + 1` |
| `layer_type` must be 1 | APK forwards the program-object value unchanged; `1` is required empirically on this firmware for image content. |
| `[0x52,0x52]` BLE header | Never sent. The APK writes stream frames directly. |
| Only `0x02` for program start | Both `0x02` and `0x1A` exist; `0x02` has count + showCount, `0x1A` has only index. |
