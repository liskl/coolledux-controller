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
	"github.com/liskl/coolledux-controller/internal/text"
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

// SetChannel switches the displayed program/channel slot on the device.
func (c *Controller) SetChannel(ctx context.Context, channel uint8) error {
	cmd := protocol.BuildChannelCommand(channel)
	resp, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("setting channel: %w", err)
	}
	if err := checkResponse(resp, protocol.RESPONSE_TYPE_CHANNEL); err != nil {
		return fmt.Errorf("channel command rejected: %w", err)
	}
	c.logger.Info("channel set", "channel", channel)
	return nil
}

// SetColor sets the device's global tint color (command 0x13 subtype 0x01).
// The low 24 bits of rgb are interpreted as 0xRRGGBB. Monochrome text content
// is rendered in this color until another color is set.
func (c *Controller) SetColor(ctx context.Context, rgb uint32) error {
	r := uint8((rgb >> 16) & 0xFF)
	g := uint8((rgb >> 8) & 0xFF)
	b := uint8(rgb & 0xFF)
	cmd := protocol.BuildColorCommand(r, g, b)
	resp, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("setting color: %w", err)
	}
	if err := checkResponse(resp, protocol.RESPONSE_TYPE_COLOR); err != nil {
		return fmt.Errorf("color command rejected: %w", err)
	}
	c.logger.Info("color set", "rgb", fmt.Sprintf("#%06X", rgb&0xFFFFFF))
	return nil
}

// SyncTime sets the device clock to the given time.
func (c *Controller) SyncTime(ctx context.Context, t time.Time) error {
	cmd := protocol.BuildTimeSyncCommand(t)
	_, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("syncing time: %w", err)
	}
	c.logger.Info("time synced", "time", t.Format(time.RFC3339))
	return nil
}

// SetTimers configures the device's on/off timer schedule.
func (c *Controller) SetTimers(ctx context.Context, items []protocol.TimerItem) error {
	cmd := protocol.BuildSetTimerCommand(items)
	_, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return fmt.Errorf("setting timers: %w", err)
	}
	c.logger.Info("timers set", "count", len(items))
	return nil
}

// GetTimers sends a get-timer command and returns the raw response bytes.
// The caller is responsible for parsing the device-specific timer slot data.
func (c *Controller) GetTimers(ctx context.Context) ([]byte, error) {
	cmd := protocol.BuildGetTimerCommand()
	resp, err := c.transport.SendAndWait(ctx, cmd, protocol.CommandTimeout)
	if err != nil {
		return nil, fmt.Errorf("getting timers: %w", err)
	}
	c.logger.Info("timers retrieved")
	return resp, nil
}

// GetDeviceInfo requests and parses device identity and state information.
func (c *Controller) GetDeviceInfo(ctx context.Context) (*models.DeviceInfo, error) {
	cmd := protocol.BuildDeviceInfoCommand()
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
		"power", info.Power,
		"brightness", info.Brightness,
		"flip_mode", info.FlipMode,
		"mic_supported", info.MicSupported,
		"remote_enabled", info.RemoteEnabled,
	)
	return info, nil
}

// ResetDevice is a stub. The reset command code has not been verified from the
// Android app analysis, so we refuse to send an unverified command.
func (c *Controller) ResetDevice(_ context.Context) error {
	return fmt.Errorf("reset command not verified on this device")
}

// DisplayImage decodes an image from raw bytes, resizes it to the requested
// region, encodes it as column-major RGB444, wraps it in a graffiti program,
// and uploads it to the device. The region is positioned at (x, y) on the
// matrix and is `width` × `height` pixels. A width or height of 0 fills the
// remaining display from the offset. fit chooses how the source image is
// mapped onto the region.
func (c *Controller) DisplayImage(ctx context.Context, imgData []byte, mode models.TextShowMode, speed, stayTime uint8, fit ledimage.FitMode, x, y, width, height int) error {
	img, err := ledimage.DecodeImage(imgData)
	if err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}

	x, y, width, height, err = c.resolveRegion(x, y, width, height)
	if err != nil {
		return err
	}

	resized := ledimage.ResizeForMatrix(img, width, height, fit)
	pixels := ledimage.ImageToRGBA(resized)
	encoded := ledimage.EncodeImageColumnMajor(pixels, width, height)

	programPayload := buildGraffitiProgram(x, y, width, height, mode, speed, stayTime, encoded)

	if err := c.sendProgram(ctx, programPayload); err != nil {
		return fmt.Errorf("sending image program: %w", err)
	}

	c.logger.Info("image displayed", "x", x, "y", y, "width", width, "height", height)
	return nil
}

// resolveRegion fills in zero width/height with "rest of display from offset"
// and validates that the rectangle is on-screen.
func (c *Controller) resolveRegion(x, y, width, height int) (int, int, int, int, error) {
	dispW := c.cfg.Display.Columns
	dispH := c.cfg.Display.Rows
	if width == 0 {
		width = dispW - x
	}
	if height == 0 {
		height = dispH - y
	}
	if x < 0 || y < 0 {
		return 0, 0, 0, 0, fmt.Errorf("placement: x and y must be non-negative (got %d, %d)", x, y)
	}
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("placement: width and height must be positive (got %d, %d)", width, height)
	}
	if x+width > dispW || y+height > dispH {
		return 0, 0, 0, 0, fmt.Errorf("placement: %dx%d at (%d,%d) extends past %dx%d display", width, height, x, y, dispW, dispH)
	}
	return x, y, width, height, nil
}

// DisplayGIF decodes a GIF, extracts and resizes each frame, encodes them
// as column-major RGB444, wraps them in an animation program, and uploads
// the result to the device. Region semantics match DisplayImage.
func (c *Controller) DisplayGIF(ctx context.Context, gifData []byte, frameDuration uint16, fit ledimage.FitMode, x, y, width, height int) error {
	g, err := ledimage.DecodeGIF(gifData)
	if err != nil {
		return fmt.Errorf("decoding gif: %w", err)
	}

	x, y, width, height, err = c.resolveRegion(x, y, width, height)
	if err != nil {
		return err
	}

	frames, delays := ledimage.ExtractFrames(g, width, height, fit)
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

	programPayload := buildAnimationProgram(x, y, width, height, encodedFrames, delays)

	if err := c.sendProgram(ctx, programPayload); err != nil {
		return fmt.Errorf("sending gif program: %w", err)
	}

	c.logger.Info("gif displayed", "frames", len(frames), "x", x, "y", y, "width", width, "height", height)
	return nil
}

// DisplayText renders a string using the embedded 16-row bold bitmap font and
// uploads it as a text content program (content type 0x01).
//
// The device colorizes monochrome text using its current global color; the
// color argument is accepted for API stability but currently ignored pending
// wiring of the 0x13 color-control command.
//
// fontSize is reserved for future multi-size support; only the 16-row font is
// shipped today.
func (c *Controller) DisplayText(ctx context.Context, s string, mode models.TextShowMode, speed, stayTime uint8, fontSize int, color uint32, fontName string) error {
	width := c.cfg.Display.Columns
	height := c.cfg.Display.Rows

	pixelColor := color
	if pixelColor == 0 {
		pixelColor = 0xFFFFFF
	}

	// Rasterize text locally into an RGB444 pixel buffer and ship it via the
	// proven graffiti path (content type 0x02). Content type 0x01 appears to
	// not be honored on this firmware revision; rasterizing locally sidesteps
	// that while reusing our verified image upload. Scroll/static modes are
	// supported by the graffiti packet's own mode byte.
	runes := []rune(s)
	// Horizontal alignment: only the horizontal-scroll modes need to anchor
	// text at the exit edge so it traverses the full width on each cycle.
	// Everything else (static, vertical scrolls, effect modes) centers.
	hAlign := text.AlignCenter
	switch mode {
	case models.TextShowModeScrollLeft:
		hAlign = text.AlignLeft
	case models.TextShowModeScrollRight:
		hAlign = text.AlignRight
	}
	pixels, err := text.RasterizeByName(runes, pixelColor, fontName, width, height, hAlign)
	if err != nil {
		return fmt.Errorf("selecting font: %w", err)
	}
	programPayload := buildGraffitiProgram(0, 0, width, height, mode, speed, stayTime, pixels)

	if err := c.sendProgram(ctx, programPayload); err != nil {
		return fmt.Errorf("sending text program: %w", err)
	}

	resolvedFont := fontName
	if resolvedFont == "" {
		resolvedFont = text.DefaultFontName
	}

	c.logger.Info("text displayed",
		"text", s,
		"mode", mode.String(),
		"runes", len(runes),
		"pixel_bytes", len(pixels),
		"font", resolvedFont,
		"font_size", fontSize,
		"color", fmt.Sprintf("#%06X", color&0xFFFFFF),
	)
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
func buildGraffitiProgram(startCol, startRow, width, height int, mode models.TextShowMode, speed, stayTime uint8, imageData []byte) []byte {
	contentLen := 28 + len(imageData)

	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x02 // graffiti content type
	content[12] = 0x01 // layer type (verified: must be 1)
	binary.BigEndian.PutUint16(content[13:15], uint16(startCol))
	binary.BigEndian.PutUint16(content[15:17], uint16(startRow))
	binary.BigEndian.PutUint16(content[17:19], uint16(width))
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
func buildAnimationProgram(startCol, startRow, width, height int, frames [][]byte, delays []uint16) []byte {
	var frameDataLen int
	for _, f := range frames {
		frameDataLen += len(f)
	}

	contentLen := 24 + 2*len(delays) + frameDataLen

	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x03 // animation content type
	content[5] = 0x01 // mode/loop flag
	content[12] = 0x01 // layer type (verified: must be 1)
	binary.BigEndian.PutUint16(content[13:15], uint16(startCol))
	binary.BigEndian.PutUint16(content[15:17], uint16(startRow))
	binary.BigEndian.PutUint16(content[17:19], uint16(width))
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

// buildCustomColorTextProgram assembles a text content packet that uses the
// device's embedded font and a per-character RGB444 color list (content
// type 0x06). The payload carries only widths + colors; the device renders
// the glyphs from its own font ROM.
//
//	Content: [totalLen:4 BE][0x06][0x00 x 5][moveSpace:2 BE]
//	         [startCol:2 BE][startRow:2 BE][showWidth:2 BE][showHeight:2 BE]
//	         [mode:1][speed:1][stayTime:1][0x00]
//	         [textNumber:2 BE][allTextWidth:2 BE]
//	         [widths: N bytes][colors: 2N bytes, each [0R, GB]]
func buildCustomColorTextProgram(width, height int, mode models.TextShowMode, speed, stayTime uint8, widths []byte, colors []byte) []byte {
	contentLen := 24 + 4 + len(widths) + len(colors)

	content := make([]byte, contentLen)
	binary.BigEndian.PutUint32(content[0:4], uint32(contentLen))
	content[4] = 0x06 // custom-color text content type
	// content[5:10] reserved zeros
	binary.BigEndian.PutUint16(content[10:12], 0)             // moveSpace
	binary.BigEndian.PutUint16(content[12:14], 0)             // start column
	binary.BigEndian.PutUint16(content[14:16], 0)             // start row
	binary.BigEndian.PutUint16(content[16:18], uint16(width)) // show width
	binary.BigEndian.PutUint16(content[18:20], uint16(height))
	content[20] = uint8(mode)
	content[21] = speed
	content[22] = stayTime
	content[23] = 0x00 // reserved
	binary.BigEndian.PutUint16(content[24:26], uint16(len(widths)))
	totalCols := 0
	for _, w := range widths {
		totalCols += int(w)
	}
	binary.BigEndian.PutUint16(content[26:28], uint16(totalCols))
	off := 28
	copy(content[off:], widths)
	off += len(widths)
	copy(content[off:], colors)

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

// checkResponse validates a device response by verifying the response type.
//
// The second byte in control command responses is the echoed value (not a
// status code). For example, brightness 255 returns [0x04, 0xFF] and power
// ON returns [0x05, 0x01]. We only check the type byte matches.
func checkResponse(data []byte, expectedType byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty response")
	}

	inner, err := protocol.ParseStreamFrame(data)
	if err != nil {
		inner = data
	}

	if len(inner) < 1 {
		return fmt.Errorf("response payload too short: %d bytes", len(inner))
	}

	if inner[0] != expectedType {
		return fmt.Errorf("unexpected response type: got 0x%02X, want 0x%02X", inner[0], expectedType)
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
//
// Device info field mapping (0x1F response, verified on hardware):
//
//	payload[0]  = 0x1F (command/response type)
//	payload[1]  = power (0=off, 1=on)
//	payload[2]  = brightness (0-255)
//	payload[3]  = rotate/mirror (0-3)
//	payload[4]  = mic_supported (bool)
//	payload[5]  = mic_on_off (bool)
//	payload[6]  = mic_mode
//	payload[7]  = show_device_id (bool)
//	payload[8]  = max_program_number
//	payload[9]  = remote_enable (bool)
//	payload[10:] = extended data
func parseDeviceInfoResponse(data []byte) (*models.DeviceInfo, error) {
	inner, err := protocol.ParseStreamFrame(data)
	if err != nil {
		inner = data
	}

	payload := inner

	if len(payload) < 2 {
		return nil, fmt.Errorf("info response too short: %d bytes", len(payload))
	}

	if payload[0] != protocol.RESPONSE_TYPE_DEVICE_INFO {
		return nil, fmt.Errorf("not a device info response: type 0x%02X", payload[0])
	}

	info := &models.DeviceInfo{
		Connected: true,
	}

	rest := payload[1:]
	if len(rest) == 0 {
		return info, nil
	}

	// payload[1]: power
	info.Power = rest[0] != 0
	rest = rest[1:]

	if len(rest) >= 1 {
		info.Brightness = rest[0]
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.FlipMode = models.FlipMode(rest[0])
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.MicSupported = rest[0] != 0
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.MicEnabled = rest[0] != 0
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.MicMode = rest[0]
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.ShowDeviceID = rest[0] != 0
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.MaxProgramNumber = rest[0]
		rest = rest[1:]
	}
	if len(rest) >= 1 {
		info.RemoteEnabled = rest[0] != 0
		rest = rest[1:]
	}
	if len(rest) > 0 {
		info.ExtendedData = make([]byte, len(rest))
		copy(info.ExtendedData, rest)
	}

	return info, nil
}
