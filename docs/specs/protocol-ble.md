# BLE Protocol Specification (Verified Against Real Hardware)

The protocol details below were verified by live testing against a CoolLEDUX device (MAC `01:00:00:FB:A4:16`) on 2026-04-11. Several command codes differ from the Python SDK documentation. **Trust the "Verified" values below, not the SDK source.**

## Connection Parameters

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

## Packet Framing: Stream Frame Only

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

## Command Codes

Verified against APK `com.jtkj.led1248` decompilation and confirmed on real hardware where noted.

### Core Commands (Hardware Verified)

| Command | Code | Payload | Verified |
|---------|------|---------|----------|
| BRIGHTNESS | `0x04` | `[value:1]` (0-255) | Yes, device echoes value in ACK |
| POWER | `0x05` | `[0x01]`=on, `[0x00]`=off | Yes, device sends ACK |
| CHANNEL | `0x07` | `[slot:1]` (program/channel index) | Yes, cycled channels 0-9 visually verified |
| PROGRAM | `0x08` | See "Program Upload" below | Yes, device ACKs start + chunks |
| TIME_SYNC | `0x09` | `[year-2000][month][day][weekday_iso][hour][min][sec]` | Yes, timer fired after sync. No CRC. |
| SET_TIMER | `0x0A` | `[count][per item: enable, hour, min, days, power_on, 0x00]` | Yes, timer fired. No CRC. |
| GET_TIMER | `0x0B` | Empty (read back timer slots) | Yes, returns stored timer data. No CRC. |
| MIRROR | `0x0C` | `[mode:1]` 0=none, 1=H, 2=V, 3=both | Yes, all 4 modes visually verified |
| COUNTDOWN | `0x0F` | `[sub]` (see below) | Yes, visual countdown overlay on display |
| STOPWATCH | `0x10` | `[sub]` (see below) | Yes, hourglass animation + timer display |
| SET_COLOR | `0x13` | `[0x01][R_nibble:1][GB_nibble:1]` (RGB444) | Yes, red/green/white all changed display color |
| DEVICE_INFO | `0x1F` | Empty (request) | Yes, returns 19 bytes of device state |

### Countdown Timer (0x0F) -- Hardware Verified

| Sub-command | Payload | Result |
|-------------|---------|--------|
| Status | `[0x0F, 0x01]` | Returns current countdown state |
| Set | `[0x0F, 0x02, hour:2, min:2, sec:2]` | Sets countdown value |
| Start/Stop | `[0x0F, 0x03, 0x01 or 0x00]` | Starts/stops countdown. Displays blinking clock overlay. |

### Stopwatch (0x10) -- Hardware Verified

| Sub-command | Payload | Result |
|-------------|---------|--------|
| Status | `[0x10, 0x01]` | Returns current stopwatch state |
| Reset | `[0x10, 0x02]` | Resets to 00:00:00 |
| Start/Stop | `[0x10, 0x03, 0x01 or 0x00]` | Starts/stops. Shows hourglass animation while running. |

### Color Control (0x13) -- Hardware Verified

Sets the global display color for text/content. Uses RGB444 encoding (2 bytes: `[0x0R][0xGB]`).

```
[0x13, 0x01, R_nibble, GB_nibble]
```

Examples tested:
- Red: `[0x13, 0x01, 0x0F, 0x00]`
- Green: `[0x13, 0x01, 0x00, 0xF0]`
- White: `[0x13, 0x01, 0x0F, 0xFF]`

### Device Info (0x1F) -- Hardware Verified

Send `stream_frame([0x1F])`. Returns 19+ bytes of device state.

Field mapping (from APK `DeviceManager.java` line 4247, verified on hardware):

| Index | Field | Type | Example | Notes |
|-------|-------|------|---------|-------|
| 0 | command_type | byte | 0x1F | Always 0x1F |
| 1 | power_on_off | bool | 0x01 | 0=off, 1=on |
| 2 | brightness | uint8 | 0xFF | 0-255 |
| 3 | rotate_mirror | uint8 | 0x00 | Mirror mode (0=none, 1=H, 2=V, 3=both) |
| 4 | mic_supported | bool | 0x00 | 0=no mic hardware, 1=has mic |
| 5 | mic_on_off | bool | 0x00 | Mic enabled |
| 6 | mic_mode | uint8 | 0x00 | Rhythm/mic mode |
| 7 | show_device_id | bool | 0x01 | Display device ID on screen |
| 8 | max_program_number | uint8 | 0x09 | Max program slots (0-9 = 10 channels) |
| 9 | remote_enable | bool | 0x01 | Remote control enabled |
| 10-18 | extended | bytes | - | Unknown, possibly fw version, hw rev |
| 19-20 | package_size | uint16 BE | (absent) | Custom chunk size (only if response is 21 bytes) |

Our device: power=ON, brightness=255, no mic, 10 program slots, remote enabled. The `mic_supported=0` confirms why rhythm commands (0x06) had no effect.

### Extended Commands (APK Confirmed)

| Command | Code | Payload | Hardware |
|---------|------|---------|----------|
| RHYTHM_TYPE | `0x06` | `[type:1]` | No response. Likely needs microphone hardware. |
| CHECK_PASSWORD | `0x0D` | `[random_key:1][xor_encoded_digits...]` | Not tested |
| SET_PASSWORD | `0x0E` | `[random_key:1][xor_encoded_digits...]` | Not tested |
| SCOREBOARD | `0x11` | `[sub][data...]` | ACKed but no visual output on this device |
| SET_COLOR_MODE | `0x13` | `[0x03][palette_data...]` | Not tested |
| DRIVE_STATE | `0x1C` | `[0x01][state:1]` / `[0x02]` | No response on this device |
| SET_PASSWORD_EXT | `0x1E` | Password variant | Not tested |
| OTA_VERSION | `0xFD` | Empty (request) | Not tested |
| OTA_START | `0xFE` | OTA init data | Not tested |
| OTA_DATA | `0xFF` | Firmware chunk data | Not tested |

### Password Encoding (from APK)

Passwords are NOT sent in plaintext. The APK XOR-encodes each digit:
```
[CMD 0x0D or 0x0E][random_key:1][digit1 XOR key][digit2 XOR key]...
```
Each password character is parsed as a hex nibble (0-F), then XOR'd with a random byte.

### GIF Upload (Firmware v30+)

For devices with firmware version >= 30, the APK can upload raw GIF files using content type `0x0C`:
```
[0x0C][0x00 x 7][layerType][0x00]
[startCol:2][startRow:2][showWidth:2][showHeight:2]
[gifFileSize:4][gifFileData...]
```
This is an alternative to the frame-by-frame animation upload (content type `0x03`).

### Command Packet Format

The APK never uses CRC for simple commands. All commands are:
```
stream_frame([CMD_CODE:1][data_bytes...])
```

Our Go service adds CRC to some commands (brightness, power, flip, channel) which still works because the device ignores trailing bytes. But the canonical format from the APK has no CRC.

CRC32 and XOR checksums are only used for:
- Program upload start packets (CRC32 over raw program data)
- Program data chunk packets (XOR checksum)
- OTA firmware upload packets

## Response Format

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
- `0x09` = Time sync ACK
- `0x0A` = Set timer ACK
- `0x0B` = Get timer response (contains timer slot data)
- `0x0C` = Flip ACK (echoes flip mode)
- `0x0D` = Device info response

## CRC32 Algorithm

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

## LZSS Compression

| Parameter | Value |
|-----------|-------|
| Window size | 512 bytes |
| Lookahead size | 18 bytes |
| Match threshold | 2 |
| Initial buffer position | 494 (`WINDOW_SIZE - LOOKAHEAD_SIZE`) |

Flag byte (8 ops): bit=1 literal (1 byte), bit=0 match ref (2 bytes). LSB-first.
Match: `byte0 = pos & 0xFF`, `byte1 = ((pos >> 4) & 0xF0) | (len - 3)`.

**LZSS is required for program uploads.** The device accepts uncompressed data (ACKs everything) but displays default text instead of the image. Compression must be applied.

## Program Upload Protocol (Verified)

Verified by uploading a 96x16 full-white image and a rainbow gradient. Both displayed correctly.

### Step 1: Build Program Content

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

### Step 2: Wrap in Program Envelope

```
[0x00 x 8][content_count:1 = 0x01][separator:1 = 0x00][content_data...]
```

### Step 3: LZSS Compress

Compress the entire program envelope. For a 96x16 white image: 3110 bytes -> 392 bytes.

### Step 4: Send Program Start

```
stream_frame([0x02][CRC32_of_raw:4 BE][raw_length:4 BE][index:1][count:1][show_count:1])
```

- CRC32 is computed over the raw (uncompressed) program envelope
- raw_length = byte count of uncompressed program envelope
- CRC32 is big-endian here (unlike control command CRC which is LE)

### Step 5: Send Data Chunks

Split compressed data into chunks of up to 1024 bytes. For each chunk:

```
stream_frame([0x03][0x00][total_compressed_len:4 BE][chunk_idx:2 BE][chunk_len:2 BE][chunk_data...][XOR:1])
```

XOR checksum = XOR of all bytes from offset 1 through end of chunk data.

Send each stream-framed chunk in 20-byte MTU pieces with ~50ms delay between pieces. Wait for ACK after each complete chunk.

## Image Data Encoding

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
