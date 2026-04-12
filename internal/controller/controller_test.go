package controller

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/models"
	"github.com/liskl/coolledux-controller/internal/protocol"
)

// testController builds a Controller wired to a real (but disconnected) BLE
// client and transport. Public methods will fail at the BLE send, but the
// command-building and error-wrapping code still executes.
func testController() *Controller {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	cfg := &config.Config{
		BLE: config.BLEConfig{
			DeviceMAC: "01:00:00:FB:A4:16",
		},
		Display: config.DisplayConfig{
			Columns: 96,
			Rows:    16,
		},
	}
	return New(bleClient, transport, cfg, logger)
}

// makeSmallPNG returns the bytes of a 4x4 solid-red PNG.
func makeSmallPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// makeSmallGIF returns the bytes of a 2-frame 4x4 GIF.
func makeSmallGIF() []byte {
	g := &gif.GIF{
		Image: []*image.Paletted{
			image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White}),
			image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White}),
		},
		Delay: []int{10, 10},
	}
	var buf bytes.Buffer
	_ = gif.EncodeAll(&buf, g)
	return buf.Bytes()
}

// ---------- Constructor & Accessor Tests ----------

func TestNew(t *testing.T) {
	ctrl := testController()
	if ctrl == nil {
		t.Fatal("New returned nil")
	}
	if ctrl.State() != StateDisconnected {
		t.Errorf("initial state = %v, want StateDisconnected", ctrl.State())
	}
	if ctrl.IsConnected() {
		t.Error("IsConnected should be false on a fresh controller")
	}
}

func TestStateAndIsConnected(t *testing.T) {
	ctrl := testController()

	// Manually flip to Connected and verify.
	ctrl.mu.Lock()
	ctrl.state = StateConnected
	ctrl.mu.Unlock()

	if ctrl.State() != StateConnected {
		t.Errorf("State = %v, want StateConnected", ctrl.State())
	}
	if !ctrl.IsConnected() {
		t.Error("IsConnected should be true when state is Connected")
	}

	// Flip to Error.
	ctrl.mu.Lock()
	ctrl.state = StateError
	ctrl.mu.Unlock()

	if ctrl.State() != StateError {
		t.Errorf("State = %v, want StateError", ctrl.State())
	}
	if ctrl.IsConnected() {
		t.Error("IsConnected should be false when state is Error")
	}
}

// ---------- Pure Function Tests ----------

func TestBuildProgramCommand(t *testing.T) {
	payload := []byte{0xAA, 0xBB}
	result := buildProgramCommand(protocol.CMD_SUBTYPE_DATA_TRANSMISSION, payload)

	// The result is a stream frame (start byte 0x01, end byte 0x03).
	if len(result) == 0 {
		t.Fatal("buildProgramCommand returned empty")
	}
	if result[0] != 0x01 {
		t.Errorf("start byte = 0x%02X, want 0x01", result[0])
	}

	// Parse it back as a stream frame to verify roundtrip.
	pld, err := protocol.ParseStreamFrame(result)
	if err != nil {
		t.Fatalf("ParseStreamFrame: %v", err)
	}
	if !bytes.Equal(pld, payload) {
		t.Errorf("payload = %v, want %v", pld, payload)
	}
}

func TestBuildProgramStartPayload(t *testing.T) {
	crc := uint32(0xDEADBEEF)
	dataLen := uint32(0x12345678)
	index := uint8(2)
	count := uint8(5)
	showCount := uint8(3)

	p := buildProgramStartPayload(crc, dataLen, index, count, showCount)

	if len(p) != 12 {
		t.Fatalf("len = %d, want 12", len(p))
	}
	if p[0] != protocol.ProgramStartMarker {
		t.Errorf("p[0] = 0x%02X, want ProgramStartMarker (0x%02X)", p[0], protocol.ProgramStartMarker)
	}
	gotCRC := binary.BigEndian.Uint32(p[1:5])
	if gotCRC != crc {
		t.Errorf("CRC = 0x%08X, want 0x%08X", gotCRC, crc)
	}
	gotLen := binary.BigEndian.Uint32(p[5:9])
	if gotLen != dataLen {
		t.Errorf("dataLen = 0x%08X, want 0x%08X", gotLen, dataLen)
	}
	if p[9] != index {
		t.Errorf("index = %d, want %d", p[9], index)
	}
	if p[10] != count {
		t.Errorf("count = %d, want %d", p[10], count)
	}
	if p[11] != showCount {
		t.Errorf("showCount = %d, want %d", p[11], showCount)
	}
}

func TestBuildProgramDataPayload(t *testing.T) {
	totalLen := uint32(4096)
	chunkIdx := uint16(7)
	data := []byte{0x10, 0x20, 0x30, 0x40, 0x50}

	p := buildProgramDataPayload(totalLen, chunkIdx, data)

	expectedLen := 10 + len(data) + 1
	if len(p) != expectedLen {
		t.Fatalf("len = %d, want %d", len(p), expectedLen)
	}

	if p[0] != protocol.ProgramDataMarker {
		t.Errorf("p[0] = 0x%02X, want ProgramDataMarker", p[0])
	}
	if p[1] != 0x00 {
		t.Errorf("p[1] = 0x%02X, want 0x00", p[1])
	}
	gotTotal := binary.BigEndian.Uint32(p[2:6])
	if gotTotal != totalLen {
		t.Errorf("totalLen = %d, want %d", gotTotal, totalLen)
	}
	gotIdx := binary.BigEndian.Uint16(p[6:8])
	if gotIdx != chunkIdx {
		t.Errorf("chunkIdx = %d, want %d", gotIdx, chunkIdx)
	}
	gotChunkLen := binary.BigEndian.Uint16(p[8:10])
	if gotChunkLen != uint16(len(data)) {
		t.Errorf("chunkLen = %d, want %d", gotChunkLen, len(data))
	}
	if !bytes.Equal(p[10:10+len(data)], data) {
		t.Errorf("data mismatch")
	}

	// Verify XOR checksum: XOR of bytes from offset 1 through end of chunk data.
	var xor byte
	for i := 1; i < 10+len(data); i++ {
		xor ^= p[i]
	}
	if p[10+len(data)] != xor {
		t.Errorf("XOR checksum = 0x%02X, want 0x%02X", p[10+len(data)], xor)
	}
}

func TestBuildProgramDataPayload_EmptyData(t *testing.T) {
	p := buildProgramDataPayload(0, 0, nil)

	if len(p) != 11 { // 10 header + 0 data + 1 xor
		t.Fatalf("len = %d, want 11", len(p))
	}
	// XOR of bytes 1..9 (header fields only).
	var xor byte
	for i := 1; i < 10; i++ {
		xor ^= p[i]
	}
	if p[10] != xor {
		t.Errorf("XOR checksum = 0x%02X, want 0x%02X", p[10], xor)
	}
}

func TestWrapProgram(t *testing.T) {
	content := []byte{0xAA, 0xBB, 0xCC}
	result := wrapProgram(content)

	if len(result) != 10+len(content) {
		t.Fatalf("len = %d, want %d", len(result), 10+len(content))
	}

	// First 8 bytes must be zeros.
	for i := 0; i < 8; i++ {
		if result[i] != 0x00 {
			t.Errorf("result[%d] = 0x%02X, want 0x00", i, result[i])
		}
	}
	// Content count = 1.
	if result[8] != 0x01 {
		t.Errorf("result[8] = 0x%02X, want 0x01", result[8])
	}
	// Separator = 0.
	if result[9] != 0x00 {
		t.Errorf("result[9] = 0x%02X, want 0x00", result[9])
	}
	// Content.
	if !bytes.Equal(result[10:], content) {
		t.Error("content mismatch after wrapper")
	}
}

func TestWrapProgram_Empty(t *testing.T) {
	result := wrapProgram(nil)
	if len(result) != 10 {
		t.Fatalf("len = %d, want 10", len(result))
	}
	if result[8] != 0x01 {
		t.Errorf("content count = 0x%02X, want 0x01", result[8])
	}
}

func TestBuildGraffitiProgram(t *testing.T) {
	width, height := 96, 16
	mode := models.TextShowModeStatic
	speed := uint8(5)
	stayTime := uint8(10)
	imageData := []byte{0x11, 0x22, 0x33}

	result := buildGraffitiProgram(width, height, mode, speed, stayTime, imageData)

	// Result = wrapper(10 bytes) + content(28 + len(imageData))
	expectedLen := 10 + 28 + len(imageData)
	if len(result) != expectedLen {
		t.Fatalf("len = %d, want %d", len(result), expectedLen)
	}

	// Verify wrapper header.
	if result[8] != 0x01 {
		t.Errorf("content count = 0x%02X, want 0x01", result[8])
	}

	// Content starts at offset 10.
	content := result[10:]
	contentLen := binary.BigEndian.Uint32(content[0:4])
	if contentLen != uint32(28+len(imageData)) {
		t.Errorf("content length field = %d, want %d", contentLen, 28+len(imageData))
	}
	if content[4] != 0x02 {
		t.Errorf("content type = 0x%02X, want 0x02 (graffiti)", content[4])
	}
	gotWidth := binary.BigEndian.Uint16(content[17:19])
	if gotWidth != uint16(width) {
		t.Errorf("width = %d, want %d", gotWidth, width)
	}
	gotHeight := binary.BigEndian.Uint16(content[19:21])
	if gotHeight != uint16(height) {
		t.Errorf("height = %d, want %d", gotHeight, height)
	}
	if content[21] != uint8(mode) {
		t.Errorf("mode = %d, want %d", content[21], mode)
	}
	if content[22] != speed {
		t.Errorf("speed = %d, want %d", content[22], speed)
	}
	if content[23] != stayTime {
		t.Errorf("stayTime = %d, want %d", content[23], stayTime)
	}
	imgDataLen := binary.BigEndian.Uint32(content[24:28])
	if imgDataLen != uint32(len(imageData)) {
		t.Errorf("imgDataLen = %d, want %d", imgDataLen, len(imageData))
	}
	if !bytes.Equal(content[28:], imageData) {
		t.Error("imageData mismatch")
	}
}

func TestBuildAnimationProgram(t *testing.T) {
	width, height := 32, 16
	frames := [][]byte{
		{0xAA, 0xBB},
		{0xCC, 0xDD, 0xEE},
	}
	delays := []uint16{100, 200}

	result := buildAnimationProgram(width, height, frames, delays)

	// Content = 24 + 2*2 delays + (2+3) frame data = 24 + 4 + 5 = 33
	expectedContentLen := 24 + 2*len(delays) + 2 + 3
	expectedLen := 10 + expectedContentLen
	if len(result) != expectedLen {
		t.Fatalf("len = %d, want %d", len(result), expectedLen)
	}

	content := result[10:]
	if content[4] != 0x03 {
		t.Errorf("content type = 0x%02X, want 0x03 (animation)", content[4])
	}
	if content[5] != 0x01 {
		t.Errorf("mode flag = 0x%02X, want 0x01", content[5])
	}
	gotWidth := binary.BigEndian.Uint16(content[17:19])
	if gotWidth != uint16(width) {
		t.Errorf("width = %d, want %d", gotWidth, width)
	}
	frameCount := binary.BigEndian.Uint16(content[22:24])
	if frameCount != uint16(len(delays)) {
		t.Errorf("frameCount = %d, want %d", frameCount, len(delays))
	}

	// Check delays.
	delay0 := binary.BigEndian.Uint16(content[24:26])
	delay1 := binary.BigEndian.Uint16(content[26:28])
	if delay0 != 100 {
		t.Errorf("delay[0] = %d, want 100", delay0)
	}
	if delay1 != 200 {
		t.Errorf("delay[1] = %d, want 200", delay1)
	}

	// Check frame data.
	frameData := content[28:]
	if !bytes.Equal(frameData[:2], frames[0]) {
		t.Error("frame 0 data mismatch")
	}
	if !bytes.Equal(frameData[2:5], frames[1]) {
		t.Error("frame 1 data mismatch")
	}
}

func TestBuildTextProgram(t *testing.T) {
	width, height := 48, 16
	mode := models.TextShowModeScrollLeft
	speed := uint8(3)
	stayTime := uint8(0)
	moveSpace := uint16(8)
	textData := []byte("hello")

	result := buildTextProgram(width, height, mode, speed, stayTime, moveSpace, textData)

	expectedContentLen := 26 + len(textData)
	expectedLen := 10 + expectedContentLen
	if len(result) != expectedLen {
		t.Fatalf("len = %d, want %d", len(result), expectedLen)
	}

	content := result[10:]
	if content[4] != 0x01 {
		t.Errorf("content type = 0x%02X, want 0x01 (text)", content[4])
	}
	if content[21] != uint8(mode) {
		t.Errorf("mode = %d, want %d", content[21], mode)
	}
	if content[22] != speed {
		t.Errorf("speed = %d, want %d", content[22], speed)
	}
	if content[23] != stayTime {
		t.Errorf("stayTime = %d, want %d", content[23], stayTime)
	}
	gotMoveSpace := binary.BigEndian.Uint16(content[24:26])
	if gotMoveSpace != moveSpace {
		t.Errorf("moveSpace = %d, want %d", gotMoveSpace, moveSpace)
	}
	if !bytes.Equal(content[26:], textData) {
		t.Error("textData mismatch")
	}
}

func TestBuildTextProgram_NoData(t *testing.T) {
	result := buildTextProgram(96, 16, models.TextShowModeStatic, 1, 0, 0, nil)
	expectedLen := 10 + 26
	if len(result) != expectedLen {
		t.Fatalf("len = %d, want %d", len(result), expectedLen)
	}
}

// ---------- checkResponse Tests ----------

func TestCheckResponse_ValidSuccess(t *testing.T) {
	// Build a proper stream-framed BLE packet with type=0x05 (power), status=0x00.
	payload := []byte{protocol.RESPONSE_TYPE_POWER, protocol.STATUS_SUCCESS}
	frame := protocol.BuildStreamFrame(payload)

	err := checkResponse(frame, protocol.RESPONSE_TYPE_POWER)
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestCheckResponse_WrongType(t *testing.T) {
	payload := []byte{protocol.RESPONSE_TYPE_BRIGHTNESS, protocol.STATUS_SUCCESS}
	frame := protocol.BuildStreamFrame(payload)

	err := checkResponse(frame, protocol.RESPONSE_TYPE_POWER)
	if err == nil {
		t.Fatal("expected error for wrong response type")
	}
}

func TestCheckResponse_EchoedValue(t *testing.T) {
	// Device echoes the command value in the second byte (e.g. power ON returns
	// [0x05, 0x01]). checkResponse only validates the type, not the value.
	payload := []byte{protocol.RESPONSE_TYPE_POWER, 0x01}
	frame := protocol.BuildStreamFrame(payload)

	err := checkResponse(frame, protocol.RESPONSE_TYPE_POWER)
	if err != nil {
		t.Errorf("expected nil error for echoed value, got: %v", err)
	}
}

func TestCheckResponse_EmptyData(t *testing.T) {
	err := checkResponse(nil, protocol.RESPONSE_TYPE_POWER)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestCheckResponse_TooShort(t *testing.T) {
	// A single byte: can't parse as stream frame or BLE packet; payload < 2.
	err := checkResponse([]byte{0xFF}, protocol.RESPONSE_TYPE_POWER)
	if err == nil {
		t.Fatal("expected error for short response")
	}
}

func TestCheckResponse_RawPayload(t *testing.T) {
	// If stream frame parsing fails, the raw data is used as the payload directly.
	payload := []byte{protocol.RESPONSE_TYPE_FLIP, protocol.STATUS_SUCCESS}

	err := checkResponse(payload, protocol.RESPONSE_TYPE_FLIP)
	if err != nil {
		t.Errorf("expected nil error for raw payload, got: %v", err)
	}
}

func TestCheckResponse_MalformedInner(t *testing.T) {
	// Data that is exactly 2 bytes but not a valid frame or BLE packet.
	// Falls through to treating raw data as payload: type=0x05, status=0x00.
	data := []byte{protocol.RESPONSE_TYPE_POWER, protocol.STATUS_SUCCESS}
	err := checkResponse(data, protocol.RESPONSE_TYPE_POWER)
	if err != nil {
		t.Errorf("expected nil for raw 2-byte payload, got: %v", err)
	}
}

// ---------- parseDeviceInfoResponse Tests ----------

func TestParseDeviceInfoResponse_Full(t *testing.T) {
	// Build a device info response matching device format:
	// [0x1F][power][brightness][mirror][mic_sup][mic_on][mic_mode][show_id][max_prog][remote][extended...]
	payload := []byte{
		protocol.RESPONSE_TYPE_DEVICE_INFO,
		0x01, // power = on
		200,  // brightness
		0x01, // flip/mirror = horizontal
		0x01, // mic_supported = true
		0x01, // mic_enabled = true
		0x03, // mic_mode = 3
		0x00, // show_device_id = false
		0x08, // max_program_number = 8
		0x01, // remote_enabled = true
		0xAA, 0xBB, // extended data
	}

	frame := protocol.BuildStreamFrame(payload)

	info, err := parseDeviceInfoResponse(frame)
	if err != nil {
		t.Fatalf("parseDeviceInfoResponse: %v", err)
	}

	if !info.Power {
		t.Error("Power should be true")
	}
	if info.Brightness != 200 {
		t.Errorf("Brightness = %d, want 200", info.Brightness)
	}
	if info.FlipMode != models.FlipModeHorizontal {
		t.Errorf("FlipMode = %d, want FlipModeHorizontal", info.FlipMode)
	}
	if !info.MicSupported {
		t.Error("MicSupported should be true")
	}
	if !info.MicEnabled {
		t.Error("MicEnabled should be true")
	}
	if info.MicMode != 3 {
		t.Errorf("MicMode = %d, want 3", info.MicMode)
	}
	if info.ShowDeviceID {
		t.Error("ShowDeviceID should be false")
	}
	if info.MaxProgramNumber != 8 {
		t.Errorf("MaxProgramNumber = %d, want 8", info.MaxProgramNumber)
	}
	if !info.RemoteEnabled {
		t.Error("RemoteEnabled should be true")
	}
	if !bytes.Equal(info.ExtendedData, []byte{0xAA, 0xBB}) {
		t.Errorf("ExtendedData = %v, want [0xAA 0xBB]", info.ExtendedData)
	}
	if !info.Connected {
		t.Error("Connected should be true")
	}
}

func TestParseDeviceInfoResponse_TooShort(t *testing.T) {
	_, err := parseDeviceInfoResponse([]byte{0xFF})
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestParseDeviceInfoResponse_WrongType(t *testing.T) {
	// type byte is not RESPONSE_TYPE_DEVICE_INFO.
	data := []byte{0x01, protocol.STATUS_SUCCESS}
	_, err := parseDeviceInfoResponse(data)
	if err == nil {
		t.Fatal("expected error for wrong response type")
	}
}

func TestParseDeviceInfoResponse_MinimalPayload(t *testing.T) {
	// Type byte + power byte only, all other fields missing.
	data := []byte{protocol.RESPONSE_TYPE_DEVICE_INFO, 0x01}
	info, err := parseDeviceInfoResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("info is nil")
	}
	if !info.Power {
		t.Error("Power should be true")
	}
	if info.Brightness != 0 {
		t.Errorf("Brightness = %d, want 0 (not enough data)", info.Brightness)
	}
}

func TestParseDeviceInfoResponse_PartialFields(t *testing.T) {
	// Type + power + brightness + flip only.
	payload := []byte{protocol.RESPONSE_TYPE_DEVICE_INFO, 0x00, 128, 0x02}

	info, err := parseDeviceInfoResponse(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Power {
		t.Error("Power should be false")
	}
	if info.Brightness != 128 {
		t.Errorf("Brightness = %d, want 128", info.Brightness)
	}
	if info.FlipMode != models.FlipModeVertical {
		t.Errorf("FlipMode = %d, want FlipModeVertical", info.FlipMode)
	}
	if info.MicSupported {
		t.Error("MicSupported should be false (not enough data)")
	}
}

// ---------- Public Method Error-Path Tests (disconnected BLE) ----------

func TestSetPower_NotConnected(t *testing.T) {
	ctrl := testController()
	err := ctrl.SetPower(context.Background(), true)
	if err == nil {
		t.Fatal("expected error from SetPower when not connected")
	}
}

func TestSetBrightness_NotConnected(t *testing.T) {
	ctrl := testController()
	err := ctrl.SetBrightness(context.Background(), 128)
	if err == nil {
		t.Fatal("expected error from SetBrightness when not connected")
	}
}

func TestSetFlip_NotConnected(t *testing.T) {
	ctrl := testController()
	err := ctrl.SetFlip(context.Background(), models.FlipModeHorizontal)
	if err == nil {
		t.Fatal("expected error from SetFlip when not connected")
	}
}

func TestSyncTime_NotConnected(t *testing.T) {
	ctrl := testController()
	err := ctrl.SyncTime(context.Background(), time.Now())
	if err == nil {
		t.Fatal("expected error from SyncTime when not connected")
	}
}

func TestSetTimers_NotConnected(t *testing.T) {
	ctrl := testController()
	items := []protocol.TimerItem{
		{Enable: true, Hour: 8, Minute: 0, Days: protocol.DayDaily, PowerOn: true},
		{Enable: true, Hour: 22, Minute: 0, Days: protocol.DayDaily, PowerOn: false},
	}
	err := ctrl.SetTimers(context.Background(), items)
	if err == nil {
		t.Fatal("expected error from SetTimers when not connected")
	}
}

func TestGetTimers_NotConnected(t *testing.T) {
	ctrl := testController()
	resp, err := ctrl.GetTimers(context.Background())
	if err == nil {
		t.Fatal("expected error from GetTimers when not connected")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
}

func TestGetDeviceInfo_NotConnected(t *testing.T) {
	ctrl := testController()
	info, err := ctrl.GetDeviceInfo(context.Background())
	if err == nil {
		t.Fatal("expected error from GetDeviceInfo when not connected")
	}
	if info != nil {
		t.Error("expected nil info on error")
	}
}

func TestResetDevice_Stub(t *testing.T) {
	ctrl := testController()
	err := ctrl.ResetDevice(context.Background())
	if err == nil {
		t.Fatal("expected error from ResetDevice stub")
	}
	if err.Error() != "reset command not verified on this device" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDisplayImage_NotConnected(t *testing.T) {
	ctrl := testController()
	pngBytes := makeSmallPNG()
	err := ctrl.DisplayImage(context.Background(), pngBytes, models.TextShowModeStatic, 5, 0)
	if err == nil {
		t.Fatal("expected error from DisplayImage when not connected")
	}
}

func TestDisplayImage_InvalidImage(t *testing.T) {
	ctrl := testController()
	err := ctrl.DisplayImage(context.Background(), []byte("not an image"), models.TextShowModeStatic, 5, 0)
	if err == nil {
		t.Fatal("expected error for invalid image data")
	}
}

func TestDisplayGIF_NotConnected(t *testing.T) {
	ctrl := testController()
	gifBytes := makeSmallGIF()
	err := ctrl.DisplayGIF(context.Background(), gifBytes, 100)
	if err == nil {
		t.Fatal("expected error from DisplayGIF when not connected")
	}
}

func TestDisplayGIF_InvalidGIF(t *testing.T) {
	ctrl := testController()
	err := ctrl.DisplayGIF(context.Background(), []byte("not a gif"), 100)
	if err == nil {
		t.Fatal("expected error for invalid GIF data")
	}
}

func TestDisplayText_NotConnected(t *testing.T) {
	ctrl := testController()
	err := ctrl.DisplayText(context.Background(), "Hello", models.TextShowModeStatic, 5, 0, 16, 0xFFFFFF)
	if err == nil {
		t.Fatal("expected error from DisplayText when not connected")
	}
}

func TestDisconnect_WhenNotConnected(t *testing.T) {
	ctrl := testController()
	// Disconnect on a fresh (not-connected) controller should succeed
	// because the BLE client's Disconnect returns nil if not connected.
	err := ctrl.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect on not-connected should succeed, got: %v", err)
	}
	if ctrl.State() != StateDisconnected {
		t.Errorf("state after disconnect = %v, want StateDisconnected", ctrl.State())
	}
}

func TestConnect_NoHardware(t *testing.T) {
	ctrl := testController()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Connect will fail because there's no BLE hardware.
	err := ctrl.Connect(ctx)
	if err == nil {
		t.Fatal("expected error from Connect without BLE hardware")
	}
	if ctrl.State() != StateError {
		t.Errorf("state after failed connect = %v, want StateError", ctrl.State())
	}
}

// ---------- sendProgram Error Path ----------

func TestSendProgram_NotConnected(t *testing.T) {
	ctrl := testController()
	payload := buildGraffitiProgram(96, 16, models.TextShowModeStatic, 1, 0, []byte{0xFF})
	err := ctrl.sendProgram(context.Background(), payload)
	if err == nil {
		t.Fatal("expected error from sendProgram when not connected")
	}
	// Program state should be set to ProgramError.
	ctrl.mu.Lock()
	ps := ctrl.programState
	ctrl.mu.Unlock()
	if ps != ProgramError {
		t.Errorf("programState = %v, want ProgramError", ps)
	}
}

// ---------- setProgramError ----------

func TestSetProgramError(t *testing.T) {
	ctrl := testController()
	ctrl.setProgramError()
	ctrl.mu.Lock()
	ps := ctrl.programState
	ctrl.mu.Unlock()
	if ps != ProgramError {
		t.Errorf("programState = %v, want ProgramError", ps)
	}
}

// ---------- buildAnimationProgram edge cases ----------

func TestBuildAnimationProgram_SingleFrame(t *testing.T) {
	frame := []byte{0x01, 0x02, 0x03}
	delays := []uint16{50}
	result := buildAnimationProgram(16, 8, [][]byte{frame}, delays)

	content := result[10:]
	if content[4] != 0x03 {
		t.Errorf("content type = 0x%02X, want 0x03", content[4])
	}
	frameCount := binary.BigEndian.Uint16(content[22:24])
	if frameCount != 1 {
		t.Errorf("frameCount = %d, want 1", frameCount)
	}
	delay := binary.BigEndian.Uint16(content[24:26])
	if delay != 50 {
		t.Errorf("delay = %d, want 50", delay)
	}
	if !bytes.Equal(content[26:29], frame) {
		t.Error("frame data mismatch")
	}
}

func TestBuildAnimationProgram_EmptyFrames(t *testing.T) {
	result := buildAnimationProgram(16, 8, nil, nil)
	// 10 wrapper + 24 header = 34
	if len(result) != 34 {
		t.Fatalf("len = %d, want 34", len(result))
	}
}

// ---------- parseDeviceInfoResponse edge cases ----------

// ---------- Success-path tests with fake BLE transport ----------

// connectedController builds a Controller where the BLE client's connected
// flag is set and Send is stubbed out (no real hardware). The transport's
// response channel is fed with responses via injectResponse.
func connectedController() (*Controller, *ble.Transport, *ble.Client) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	cfg := &config.Config{
		BLE: config.BLEConfig{
			DeviceMAC: "01:00:00:FB:A4:16",
		},
		Display: config.DisplayConfig{
			Columns: 96,
			Rows:    16,
		},
	}
	ctrl := New(bleClient, transport, cfg, logger)

	// Mark the client as connected and stub out the real BLE write.
	bleClient.OverrideConnectedForTest(true)
	bleClient.OverrideSendFuncForTest(func(ctx context.Context, data []byte) error {
		return nil // silently accept all writes
	})

	return ctrl, transport, bleClient
}

// fakeResponse builds a stream-framed BLE response with the given type and
// success status.
func fakeResponse(respType byte) []byte {
	payload := []byte{respType, protocol.STATUS_SUCCESS}
	return protocol.BuildStreamFrame(payload)
}

func TestSetPower_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_POWER)) }()

	err := ctrl.SetPower(context.Background(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPower_Off_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_POWER)) }()

	err := ctrl.SetPower(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetBrightness_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_BRIGHTNESS)) }()

	err := ctrl.SetBrightness(context.Background(), 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetFlip_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_FLIP)) }()

	err := ctrl.SetFlip(context.Background(), models.FlipModeHorizontal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncTime_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	// SyncTime doesn't check response type, just checks for error.
	go func() { transport.InjectResponseForTest(fakeResponse(0x00)) }()

	err := ctrl.SyncTime(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetTimers_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeResponse(0x00)) }()

	items := []protocol.TimerItem{
		{Enable: true, Hour: 8, Minute: 0, Days: protocol.DayDaily, PowerOn: true},
	}
	err := ctrl.SetTimers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTimers_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	go func() { transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_GET_TIMER)) }()

	resp, err := ctrl.GetTimers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil response")
	}
}

func TestResetDevice_Connected_StillStubbed(t *testing.T) {
	ctrl, _, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	// Even when connected, ResetDevice returns an error because the command is not verified.
	err := ctrl.ResetDevice(context.Background())
	if err == nil {
		t.Fatal("expected error from ResetDevice stub")
	}
}

func TestGetDeviceInfo_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	// Build a device info response in the verified device format.
	payload := []byte{
		protocol.RESPONSE_TYPE_DEVICE_INFO,
		0x01, // power on
		128,  // brightness
		0x00, // flip none
		0x01, // mic_supported
		0x00, // mic_enabled
		0x00, // mic_mode
		0x00, // show_device_id
		0x04, // max_program_number
		0x01, // remote_enabled
	}

	frame := protocol.BuildStreamFrame(payload)
	go func() { transport.InjectResponseForTest(frame) }()

	info, err := ctrl.GetDeviceInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Power {
		t.Error("Power should be true")
	}
	if info.Brightness != 128 {
		t.Errorf("Brightness = %d, want 128", info.Brightness)
	}
	if info.MaxProgramNumber != 4 {
		t.Errorf("MaxProgramNumber = %d, want 4", info.MaxProgramNumber)
	}
}

func TestDisplayText_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	// sendProgram needs: 1 response for program start, N responses for data chunks.
	// For a small text program the payload fits in 1 chunk, so 2 responses total.
	go func() {
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
	}()

	err := ctrl.DisplayText(context.Background(), "Hello", models.TextShowModeStatic, 5, 0, 16, 0xFFFFFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDisplayImage_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	pngBytes := makeSmallPNG()

	// The graffiti program for a 96x16 display with RGB444 encoding is
	// relatively small. After LZSS compression it should fit in a few chunks.
	// Inject enough responses for start + multiple data chunks.
	go func() {
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 10; i++ {
			transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	err := ctrl.DisplayImage(context.Background(), pngBytes, models.TextShowModeStatic, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDisplayGIF_Connected(t *testing.T) {
	ctrl, transport, client := connectedController()
	defer client.OverrideConnectedForTest(false)

	gifBytes := makeSmallGIF()

	go func() {
		transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_START))
		for i := 0; i < 10; i++ {
			transport.InjectResponseForTest(fakeResponse(protocol.RESPONSE_TYPE_PROGRAM_DATA))
		}
	}()

	err := ctrl.DisplayGIF(context.Background(), gifBytes, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDeviceInfoResponse_RawPayload(t *testing.T) {
	// Raw payload without stream framing (fallback path).
	payload := []byte{
		protocol.RESPONSE_TYPE_DEVICE_INFO,
		0x01, // power on
		0xFF, // brightness 255
	}

	info, err := parseDeviceInfoResponse(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Power {
		t.Error("Power should be true")
	}
	if info.Brightness != 255 {
		t.Errorf("Brightness = %d, want 255", info.Brightness)
	}
}

