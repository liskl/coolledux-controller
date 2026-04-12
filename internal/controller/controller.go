package controller

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	ledimage "github.com/liskl/coolledux-controller/internal/image"
	"github.com/liskl/coolledux-controller/internal/models"
	"github.com/liskl/coolledux-controller/internal/protocol"
)

// Controller orchestrates BLE communication with the CoolLEDUX LED matrix.
// It manages connection state, sends commands, and handles program uploads.
type Controller struct {
	transport    *ble.Transport
	bleClient    *ble.Client
	cfg          *config.Config
	state        DeviceState
	programState ProgramSendingState
	deviceInfo   *models.DeviceInfo
	mu           sync.Mutex
	logger       *slog.Logger
}

// New creates a Controller wired to the given BLE client and transport.
func New(bleClient *ble.Client, transport *ble.Transport, cfg *config.Config, logger *slog.Logger) *Controller {
	return &Controller{
		transport:    transport,
		bleClient:    bleClient,
		cfg:          cfg,
		state:        StateDisconnected,
		programState: ProgramIdle,
		logger:       logger,
	}
}

// Connect establishes the BLE connection to the configured device.
func (c *Controller) Connect(ctx context.Context) error {
	c.mu.Lock()
	c.state = StateConnecting
	c.mu.Unlock()

	if err := c.bleClient.Connect(ctx, c.cfg.BLE.DeviceMAC); err != nil {
		c.mu.Lock()
		c.state = StateError
		c.mu.Unlock()
		return fmt.Errorf("connecting to %s: %w", c.cfg.BLE.DeviceMAC, err)
	}

	c.mu.Lock()
	c.state = StateConnected
	c.mu.Unlock()

	c.logger.Info("device connected", "mac", c.cfg.BLE.DeviceMAC)
	return nil
}

// Disconnect tears down the BLE connection.
func (c *Controller) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	c.state = StateDisconnecting
	c.mu.Unlock()

	if err := c.bleClient.Disconnect(); err != nil {
		c.mu.Lock()
		c.state = StateError
		c.mu.Unlock()
		return fmt.Errorf("disconnecting: %w", err)
	}

	c.mu.Lock()
	c.state = StateDisconnected
	c.mu.Unlock()

	c.logger.Info("device disconnected")
	return nil
}

// SetPower turns the display on or off.
func (c *Controller) SetPower(ctx context.Context, on bool) error {
	cmd := protocol.BuildPowerCommand(on)
	resp, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("setting power: %w", err)
	}
	if err := checkResponse(resp, protocol.RESPONSE_TYPE_POWER); err != nil {
		return fmt.Errorf("power command rejected: %w", err)
	}
	c.logger.Info("power set", "on", on)
	return nil
}

// SetBrightness sets the display brightness (0-255).
func (c *Controller) SetBrightness(ctx context.Context, brightness uint8) error {
	cmd := protocol.BuildBrightnessCommand(brightness)
	resp, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("setting brightness: %w", err)
	}
	if err := checkResponse(resp, protocol.RESPONSE_TYPE_BRIGHTNESS); err != nil {
		return fmt.Errorf("brightness command rejected: %w", err)
	}
	c.logger.Info("brightness set", "level", brightness)
	return nil
}

// SetFlip sets the display orientation flip mode.
func (c *Controller) SetFlip(ctx context.Context, mode models.FlipMode) error {
	cmd := protocol.BuildFlipCommand(uint8(mode))
	resp, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("setting flip: %w", err)
	}
	if err := checkResponse(resp, protocol.RESPONSE_TYPE_FLIP); err != nil {
		return fmt.Errorf("flip command rejected: %w", err)
	}
	c.logger.Info("flip set", "mode", mode)
	return nil
}

// SyncTime sets the device clock.
func (c *Controller) SyncTime(ctx context.Context, hour, minute, second uint8) error {
	cmd := protocol.BuildTimeCommand(hour, minute, second)
	_, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("syncing time: %w", err)
	}
	c.logger.Info("time synced", "hour", hour, "minute", minute, "second", second)
	return nil
}

// SetTimers configures the device's on/off timer schedule.
func (c *Controller) SetTimers(ctx context.Context, items []protocol.TimerItem) error {
	cmd := protocol.BuildTimerCommand(items)
	_, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("setting timers: %w", err)
	}
	c.logger.Info("timers set", "count", len(items))
	return nil
}

// GetDeviceInfo requests and parses device identity and state information.
func (c *Controller) GetDeviceInfo(ctx context.Context) (*models.DeviceInfo, error) {
	cmd := protocol.BuildInfoCommand()
	resp, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return nil, fmt.Errorf("getting device info: %w", err)
	}

	info, err := parseDeviceInfoResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("parsing device info: %w", err)
	}

	c.mu.Lock()
	c.deviceInfo = info
	c.mu.Unlock()

	c.logger.Info("device info received",
		"model", info.Model,
		"firmware", info.FirmwareVersion,
		"columns", info.Columns,
		"rows", info.Rows,
	)
	return info, nil
}

// ResetDevice sends a factory reset command to the device.
func (c *Controller) ResetDevice(ctx context.Context) error {
	cmd := protocol.BuildResetCommand()
	_, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("resetting device: %w", err)
	}
	c.logger.Info("device reset")
	return nil
}

// DisplayImage decodes an image from raw bytes, resizes it to the display
// dimensions, encodes it as column-major RGB444, wraps it in a graffiti
// program, and uploads it to the device.
func (c *Controller) DisplayImage(ctx context.Context, imgData []byte, mode models.TextShowMode, speed, stayTime uint8) error {
	img, err := ledimage.DecodeImage(imgData)
	if err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}

	width := c.cfg.Display.Columns
	height := c.cfg.Display.Rows

	resized := ledimage.ResizeExact(img, width, height)
	pixels := ledimage.ImageToRGBA(resized)
	encoded := ledimage.EncodeImageColumnMajor(pixels, width, height)

	programPayload := buildGraffitiProgram(width, height, mode, speed, stayTime, encoded)

	if err := c.sendProgram(ctx, programPayload); err != nil {
		return fmt.Errorf("sending image program: %w", err)
	}

	c.logger.Info("image displayed", "width", width, "height", height)
	return nil
}

// DisplayGIF decodes a GIF, extracts and resizes each frame, encodes them
// as column-major RGB444, wraps them in an animation program, and uploads
// the result to the device.
func (c *Controller) DisplayGIF(ctx context.Context, gifData []byte, frameDuration uint16) error {
	g, err := ledimage.DecodeGIF(gifData)
	if err != nil {
		return fmt.Errorf("decoding gif: %w", err)
	}

	width := c.cfg.Display.Columns
	height := c.cfg.Display.Rows

	frames, delays := ledimage.ExtractFrames(g, width, height)
	if len(frames) == 0 {
		return fmt.Errorf("gif has no frames")
	}

	// Override delays if a custom frame duration was provided.
	if frameDuration > 0 {
		for i := range delays {
			delays[i] = frameDuration
		}
	}

	// Encode each frame as column-major RGB444.
	var encodedFrames [][]byte
	for _, frame := range frames {
		encodedFrames = append(encodedFrames, ledimage.EncodeImageColumnMajor(frame, width, height))
	}

	programPayload := buildAnimationProgram(width, height, encodedFrames, delays)

	if err := c.sendProgram(ctx, programPayload); err != nil {
		return fmt.Errorf("sending gif program: %w", err)
	}

	c.logger.Info("gif displayed", "frames", len(frames), "width", width, "height", height)
	return nil
}

// DisplayText creates a text program payload and uploads it to the device.
// Font rendering is not yet implemented; this sends a placeholder text program
// with empty glyph data.
func (c *Controller) DisplayText(ctx context.Context, text string, mode models.TextShowMode, speed, stayTime uint8, fontSize int, color uint32) error {
	width := c.cfg.Display.Columns
	height := c.cfg.Display.Rows

	// Placeholder: no rendered glyph data yet.
	programPayload := buildTextProgram(width, height, mode, speed, stayTime, 0, nil)

	if err := c.sendProgram(ctx, programPayload); err != nil {
		return fmt.Errorf("sending text program: %w", err)
	}

	c.logger.Info("text displayed (placeholder)", "text", text, "fontSize", fontSize, "color", color)
	return nil
}

// State returns the current device connection state.
func (c *Controller) State() DeviceState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// IsConnected returns true if the controller is in the connected state.
func (c *Controller) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == StateConnected
}

// sendProgram compresses the program payload with LZSS, builds the start
// packet, sends it and waits for an ack, then sends each data chunk and
// waits for an ack after each one.
func (c *Controller) sendProgram(ctx context.Context, programPayload []byte) error {
	c.mu.Lock()
	c.programState = ProgramSendingStart
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.programState != ProgramError {
			c.programState = ProgramIdle
		}
		c.mu.Unlock()
	}()

	// Compute CRC over uncompressed data.
	rawCRC := protocol.Calculate(programPayload)
	rawLen := len(programPayload)

	// Compress with LZSS. Use compressed only if it's smaller.
	compressed, wasCompressed := protocol.Compress(programPayload)
	if !wasCompressed {
		compressed = programPayload
	}

	// Build and send the program start packet.
	startPayload := buildProgramStartPayload(rawCRC, uint32(rawLen), 0, 1, 1)
	startCmd := buildProgramCommand(protocol.CMD_SUBTYPE_DATA_TRANSMISSION, startPayload)

	resp, err := c.transport.SendAndWait(ctx, startCmd, protocol.CommandTimeout)
	if err != nil {
		c.setProgramError()
		return fmt.Errorf("sending program start: %w", err)
	}
	// Program start status: 0x00=new, 0x01=overwriting existing. Both are OK.
	if err := checkProgramStartResponse(resp); err != nil {
		c.setProgramError()
		return fmt.Errorf("program start rejected: %w", err)
	}

	// Split compressed data into chunks and send each one.
	c.mu.Lock()
	c.programState = ProgramSendingData
	c.mu.Unlock()

	totalLen := uint32(len(compressed))
	chunkIdx := 0
	for offset := 0; offset < len(compressed); offset += protocol.ProgramChunkSize {
		end := offset + protocol.ProgramChunkSize
		if end > len(compressed) {
			end = len(compressed)
		}
		chunk := compressed[offset:end]

		chunkPayload := buildProgramDataPayload(totalLen, uint16(chunkIdx), chunk)
		chunkCmd := buildProgramCommand(protocol.CMD_SUBTYPE_DATA_PACKET, chunkPayload)

		c.logger.Debug("sending program chunk",
			"chunk", chunkIdx,
			"chunkSize", len(chunk),
			"frameSize", len(chunkCmd),
			"totalCompressed", len(compressed),
		)

		if err := c.transport.SendCommand(ctx, chunkCmd); err != nil {
			c.setProgramError()
			return fmt.Errorf("sending program chunk %d: %w", chunkIdx, err)
		}

		// Wait for the device to process the chunk. The device sends a
		// notification ACK, but timing is tight. Use a generous wait.
		resp, err := c.transport.WaitForResponse(ctx, 5*time.Second)
		if err != nil {
			c.logger.Warn("no ACK for chunk, continuing", "chunk", chunkIdx, "error", err)
		} else {
			c.logger.Debug("chunk ACK", "chunk", chunkIdx, "resp", fmt.Sprintf("%x", resp))
		}

		chunkIdx++
	}

	c.mu.Lock()
	c.programState = ProgramComplete
	c.mu.Unlock()

	c.logger.Info("program upload complete",
		"rawSize", rawLen,
		"compressedSize", len(compressed),
		"chunks", chunkIdx,
	)
	return nil
}

func (c *Controller) setProgramError() {
	c.mu.Lock()
	c.programState = ProgramError
	c.mu.Unlock()
}

// buildProgramCommand wraps a program payload in stream framing.
// Verified against real device: all packets (control + program) use stream framing.
func buildProgramCommand(_ byte, payload []byte) []byte {
	return protocol.BuildStreamFrame(payload)
}

// buildProgramStartPayload builds the inner payload for a program start packet.
//
//	[0x02][CRC32:4 BE][DataLen:4 BE][Index:1][Count:1][ShowCount:1]
func buildProgramStartPayload(crc uint32, dataLen uint32, index, count, showCount uint8) []byte {
	p := make([]byte, 12)
	p[0] = protocol.ProgramStartMarker
	binary.BigEndian.PutUint32(p[1:5], crc)
	binary.BigEndian.PutUint32(p[5:9], dataLen)
	p[9] = index
	p[10] = count
	p[11] = showCount
	return p
}

// buildProgramDataPayload builds the inner payload for a program data chunk.
//
//	[0x03][0x00][TotalLen:4 BE][ChunkIdx:2 BE][ChunkLen:2 BE][Data...][XOR:1]
func buildProgramDataPayload(totalLen uint32, chunkIdx uint16, data []byte) []byte {
	p := make([]byte, 10+len(data)+1)
	p[0] = protocol.ProgramDataMarker
	p[1] = 0x00
	binary.BigEndian.PutUint32(p[2:6], totalLen)
	binary.BigEndian.PutUint16(p[6:8], chunkIdx)
	binary.BigEndian.PutUint16(p[8:10], uint16(len(data)))
	copy(p[10:], data)

	// XOR checksum: all bytes from offset 1 through end of chunk data.
	var xor byte
	for i := 1; i < 10+len(data); i++ {
		xor ^= p[i]
	}
	p[10+len(data)] = xor

	return p
}

// buildGraffitiProgram assembles a graffiti/image program payload with the
// standard wrapper and content header.
//
//	Wrapper: [0x00 x 8][contentCount:1][0x00][content...]
//	Content: [totalLen:4 BE][0x02][0x00 x 7][layerType:1][startCol:2 BE][startRow:2 BE]
//	         [width:2 BE][height:2 BE][mode:1][speed:1][stayTime:1][imgDataLen:4 BE][imgData...]
func buildGraffitiProgram(width, height int, mode models.TextShowMode, speed, stayTime uint8, imageData []byte) []byte {
	// Content block: 4 (length) + 1 (type) + 7 (reserved) + 1 (layer) +
	//   2+2 (start col/row) + 2+2 (width/height) + 1 (mode) + 1 (speed) +
	//   1 (stay) + 4 (img data len) + N (img data) = 28 + N
	contentLen := 28 + len(imageData)

	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x02 // graffiti content type
	// content[5:12] reserved zeros
	content[12] = 0x01 // layer type (verified: must be 1)
	binary.BigEndian.PutUint16(content[13:15], 0)             // start column
	binary.BigEndian.PutUint16(content[15:17], 0)             // start row
	binary.BigEndian.PutUint16(content[17:19], uint16(width)) // show width
	binary.BigEndian.PutUint16(content[19:21], uint16(height))
	content[21] = uint8(mode)
	content[22] = speed
	content[23] = stayTime
	binary.BigEndian.PutUint32(content[24:28], uint32(len(imageData)))
	copy(content[28:], imageData)

	return wrapProgram(content)
}

// buildAnimationProgram assembles an animation program payload.
//
//	Content: [totalLen:4 BE][0x03][0x01][0x00 x 6][layerType:1][startCol:2 BE][startRow:2 BE]
//	         [width:2 BE][height:2 BE][0x00][frameCount:2 BE][delays:2*N BE][frameData...]
func buildAnimationProgram(width, height int, frames [][]byte, delays []uint16) []byte {
	// Calculate total frame data size.
	var frameDataLen int
	for _, f := range frames {
		frameDataLen += len(f)
	}

	// Content size: 4 (length) + 1 (type) + 1 (mode flag) + 6 (reserved) +
	//   1 (layer) + 2+2 (start col/row) + 2+2 (width/height) + 1 (reserved) +
	//   2 (frame count) + 2*N (delays) + M (frame data)
	//   = 24 + 2*len(delays) + frameDataLen
	contentLen := 24 + 2*len(delays) + frameDataLen

	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x03 // animation content type
	content[5] = 0x01 // mode/loop flag
	// content[6:12] reserved zeros
	content[12] = 0x01 // layer type (verified: must be 1)
	binary.BigEndian.PutUint16(content[13:15], 0)             // start column
	binary.BigEndian.PutUint16(content[15:17], 0)             // start row
	binary.BigEndian.PutUint16(content[17:19], uint16(width)) // show width
	binary.BigEndian.PutUint16(content[19:21], uint16(height))
	content[21] = 0x00 // reserved
	binary.BigEndian.PutUint16(content[22:24], uint16(len(delays)))

	// Write frame delays.
	offset := 24
	for _, d := range delays {
		binary.BigEndian.PutUint16(content[offset:offset+2], d)
		offset += 2
	}

	// Write concatenated frame data.
	for _, f := range frames {
		copy(content[offset:], f)
		offset += len(f)
	}

	return wrapProgram(content)
}

// buildTextProgram assembles a text program payload.
//
//	Content: [totalLen:4 BE][0x01][0x00 x 7][layerType:1][startCol:2 BE][startRow:2 BE]
//	         [width:2 BE][height:2 BE][mode:1][speed:1][stayTime:1][moveSpace:2 BE][textData...]
func buildTextProgram(width, height int, mode models.TextShowMode, speed, stayTime uint8, moveSpace uint16, textData []byte) []byte {
	contentLen := 26 + len(textData)

	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x01 // text content type
	// content[5:12] reserved zeros
	content[12] = 0x01 // layer type (verified: must be 1)
	binary.BigEndian.PutUint16(content[13:15], 0)             // start column
	binary.BigEndian.PutUint16(content[15:17], 0)             // start row
	binary.BigEndian.PutUint16(content[17:19], uint16(width)) // show width
	binary.BigEndian.PutUint16(content[19:21], uint16(height))
	content[21] = uint8(mode)
	content[22] = speed
	content[23] = stayTime
	binary.BigEndian.PutUint16(content[24:26], moveSpace)
	if len(textData) > 0 {
		copy(content[26:], textData)
	}

	return wrapProgram(content)
}

// wrapProgram wraps content in the standard program wrapper.
//
//	[0x00 x 8][contentCount:1][0x00][content...]
func wrapProgram(content []byte) []byte {
	wrapper := make([]byte, 10+len(content))
	// wrapper[0:8] = 8 zero bytes (reserved)
	wrapper[8] = 0x01 // content count = 1
	wrapper[9] = 0x00 // separator
	copy(wrapper[10:], content)
	return wrapper
}

// checkResponse does basic validation of a device response. It parses the
// stream frame and BLE packet, then checks the response type and status byte.
func checkResponse(data []byte, expectedType byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty response")
	}

	// Try to parse as stream frame -> BLE packet.
	inner, err := protocol.ParseStreamFrame(data)
	if err != nil {
		// Some devices send raw BLE packets without stream framing.
		inner = data
	}

	// Device responses are stream-framed only (no BLE packet header).
	// After ParseStreamFrame, inner is the raw payload: [type][status][data...].
	payload := inner

	if len(payload) < 2 {
		return fmt.Errorf("response payload too short: %d bytes", len(payload))
	}

	respType := payload[0]
	status := payload[1]

	if respType != expectedType {
		return fmt.Errorf("unexpected response type: got 0x%02X, want 0x%02X", respType, expectedType)
	}

	if status != protocol.STATUS_SUCCESS {
		return fmt.Errorf("device returned error status 0x%02X", status)
	}

	return nil
}

// checkProgramStartResponse validates a program start ACK. The device returns
// status 0x00 for a new program or 0x01 when overwriting an existing one.
// Both are acceptable.
func checkProgramStartResponse(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty response")
	}

	inner, err := protocol.ParseStreamFrame(data)
	if err != nil {
		inner = data
	}

	if len(inner) < 2 {
		return fmt.Errorf("response too short: %d bytes", len(inner))
	}

	if inner[0] != protocol.RESPONSE_TYPE_PROGRAM_START {
		return fmt.Errorf("unexpected response type: got 0x%02X, want 0x%02X",
			inner[0], protocol.RESPONSE_TYPE_PROGRAM_START)
	}

	// 0x00 = new program accepted, 0x01 = overwriting existing. Both OK.
	return nil
}

// parseDeviceInfoResponse extracts DeviceInfo from a device info response.
func parseDeviceInfoResponse(data []byte) (*models.DeviceInfo, error) {
	inner, err := protocol.ParseStreamFrame(data)
	if err != nil {
		inner = data
	}

	// Device responses are stream-framed only (no BLE packet header).
	payload := inner

	if len(payload) < 2 {
		return nil, fmt.Errorf("info response too short: %d bytes", len(payload))
	}

	if payload[0] != protocol.RESPONSE_TYPE_DEVICE_INFO {
		return nil, fmt.Errorf("not a device info response: type 0x%02X", payload[0])
	}

	// The info payload layout after the type byte varies by firmware.
	// We do best-effort extraction of what's available.
	info := &models.DeviceInfo{
		Connected: true,
	}

	rest := payload[1:]
	if len(rest) == 0 {
		return info, nil
	}

	// Status byte.
	if rest[0] != protocol.STATUS_SUCCESS {
		return nil, fmt.Errorf("info response error status: 0x%02X", rest[0])
	}
	rest = rest[1:]

	// Parse null-terminated strings and numeric fields from the remaining bytes.
	// Field order: model, firmware version, hardware version, serial number,
	// columns (2 bytes BE), rows (2 bytes BE), power (1), brightness (1), flip (1).
	info.Model = extractNullString(&rest)
	info.FirmwareVersion = extractNullString(&rest)
	info.HardwareVersion = extractNullString(&rest)
	info.SerialNumber = extractNullString(&rest)

	if len(rest) >= 2 {
		info.Columns = int(binary.BigEndian.Uint16(rest[0:2]))
		rest = rest[2:]
	}
	if len(rest) >= 2 {
		info.Rows = int(binary.BigEndian.Uint16(rest[0:2]))
		rest = rest[2:]
	}
	if len(rest) >= 1 {
		info.Power = rest[0] != 0
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.Brightness = rest[0]
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.FlipMode = models.FlipMode(rest[0])
	}

	return info, nil
}

// extractNullString reads a null-terminated string from the front of *data.
// Returns empty string if data is empty or starts with null.
func extractNullString(data *[]byte) string {
	d := *data
	for i, b := range d {
		if b == 0x00 {
			s := string(d[:i])
			*data = d[i+1:]
			return s
		}
	}
	// No null terminator found; consume everything.
	s := string(d)
	*data = nil
	return s
}
