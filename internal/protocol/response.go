package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrResponseTooShort = errors.New("response payload too short")
	ErrDeviceInfoShort  = errors.New("device info data too short")
)

// Response represents a parsed device response after both framing layers
// have been stripped.
type Response struct {
	Type   uint8
	Status uint8
	Data   []byte
}

// IsSuccess returns true when the device reports status 0x00.
func (r *Response) IsSuccess() bool {
	return r.Status == STATUS_SUCCESS
}

// IsProgramStart returns true when this is a program start acknowledgment.
func (r *Response) IsProgramStart() bool {
	return r.Type == RESPONSE_TYPE_PROGRAM_START
}

// IsProgramData returns true when this is a program data chunk acknowledgment.
func (r *Response) IsProgramData() bool {
	return r.Type == RESPONSE_TYPE_PROGRAM_DATA
}

// IsDeviceInfo returns true when this is a device info response.
func (r *Response) IsDeviceInfo() bool {
	return r.Type == RESPONSE_TYPE_DEVICE_INFO
}

// ParseResponse unwraps a raw byte stream through both framing layers and
// extracts the response type, status, and remaining data.
func ParseResponse(data []byte) (*Response, error) {
	// Layer 2: strip stream frame.
	inner, err := ParseStreamFrame(data)
	if err != nil {
		return nil, fmt.Errorf("stream frame: %w", err)
	}

	// Layer 1: strip BLE packet frame.
	_, _, payload, err := ParseBLEPacket(inner)
	if err != nil {
		return nil, fmt.Errorf("ble packet: %w", err)
	}

	if len(payload) < 2 {
		return nil, ErrResponseTooShort
	}

	resp := &Response{
		Type:   payload[0],
		Status: payload[1],
	}
	if len(payload) > 2 {
		resp.Data = make([]byte, len(payload)-2)
		copy(resp.Data, payload[2:])
	}
	return resp, nil
}

// DeviceInfoData holds parsed fields from a DEVICE_INFO response.
type DeviceInfoData struct {
	ModelID         uint8
	FirmwareVersion string
	HardwareVersion string
	Columns         uint16
	Rows            uint16
}

// ParseDeviceInfoResponse extracts device identity and display dimensions
// from a DEVICE_INFO response.
//
// Expected data layout (after type+status bytes):
//
//	[0]    ModelID
//	[1]    FW major
//	[2]    FW minor
//	[3]    FW patch
//	[4]    HW major
//	[5]    HW minor
//	[6:8]  Columns (big-endian uint16)
//	[8:10] Rows    (big-endian uint16)
func ParseDeviceInfoResponse(resp *Response) (*DeviceInfoData, error) {
	if resp == nil {
		return nil, ErrResponseTooShort
	}
	if len(resp.Data) < 10 {
		return nil, fmt.Errorf("%w: need 10 bytes, have %d", ErrDeviceInfoShort, len(resp.Data))
	}
	d := resp.Data
	info := &DeviceInfoData{
		ModelID:         d[0],
		FirmwareVersion: fmt.Sprintf("%d.%d.%d", d[1], d[2], d[3]),
		HardwareVersion: fmt.Sprintf("%d.%d", d[4], d[5]),
		Columns:         binary.BigEndian.Uint16(d[6:8]),
		Rows:            binary.BigEndian.Uint16(d[8:10]),
	}
	return info, nil
}
