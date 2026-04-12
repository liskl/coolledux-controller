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

## Command Codes (Verified)

| Command | Code | Payload | Verified |
|---------|------|---------|----------|
| BRIGHTNESS | `0x04` | `[value:1]` (0-255) | Yes, device echoes value in ACK |
| POWER | `0x05` | `[0x01]`=on, `[0x00]`=off | Yes, device sends ACK |
| CMD 0x06 | `0x06` | - | Device ignores ALL packets with this code |
| CHANNEL | `0x07` | `[slot:1]` (program/channel index) | Yes, cycled channels 0-9 visually verified |
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
