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
| `0x0D` | Check password | `[random:1] { digit ^ random } * n [xorChecksum:1]` | `getCheckPasswordData` | 2770 | untested |
| `0x0E` | Set password | `[random:1] { digit ^ random } * n [xorChecksum:1]` | `getSetPasswordData` | 4438 | untested |
| `0x0F` | Countdown status | `[0x01]` | `getCountDownStatus` | 2811 | verified |
| `0x0F` | Countdown set value | `[0x02][hour:2 BE][min:2 BE][sec:2 BE]` (each as uint16) | `getCountDownReset` | 2789 | verified |
| `0x0F` | Countdown start/stop | `[0x03][0x01 or 0x00]` | `getCountDownStartOrStop` | 2799 | verified |
| `0x10` | Stopwatch status | `[0x01]` | `getStopwatchStatus` | 4565 | verified |
| `0x10` | Stopwatch reset | `[0x02]` | `getStopwatchReset` | 4546 | verified |
| `0x10` | Stopwatch start/stop | `[0x03][0x01 or 0x00]` | `getStopwatchStartOrStop` | 4553 | verified |
| `0x11` | Scoreboard status | `[0x01]` | `getScoreBoardStatus` | 4394 | ACK only (no visual on 16x96) |
| `0x11` | Scoreboard set scores | `[0x02][scoreA:2 BE][scoreB:2 BE]` | `getScoreBoardSetCore` | 4357 | no visual |
| `0x11` | Scoreboard set time | `[0x03][hour:1][min:1][isTimer:1]` | `getScoreBoardSetTime` | 4368 | no visual |
| `0x11` | Scoreboard start/stop | `[0x04][0x01 or 0x00]` | `getScoreBoardStartOrStop` | 4382 | no visual |
| `0x13` | Set color | `[0x01][R4:nibble][GB:byte]` (RGB444) | `setColor` | 4706 | verified |
| `0x13` | Set color speed | `[0x02][speed:1]` | `setColorSpeed` | 4835 | untested |
| `0x13` | Set color mode (palette) | `[0x03][modeBytes...]` — preset cycling palettes | `setColorMode` | 4714 | untested |
| `0x1A` | Program start (simple) | `[CRC32:4 BE][rawLen:4 BE][index:1]` | `getStartDataForProgram` (2-arg) | 4484 | not used in current Go service |
| `0x1C` | Drive state set | `[0x01][state:1]` | `getSetDriveState` | 4419 | no response on this device |
| `0x1C` | Drive state get | `[0x02]` | `getDriveState` | 4277 | no response |
| `0x1E` | Set device info field | `[0x01=brightness, 0x02=mirror, 0x03=rotate][value:1]` | `setDeviceInfo` | 4843 | untested; alternate path to 0x04/0x0C |
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
| `0x0C` raw GIF (firmware >= v30) | `getDataWithAnimationCombineProgram` (GIF overload) | `:3103` |

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
