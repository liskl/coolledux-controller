package protocol

import (
	"testing"
)

// buildFakeResponse constructs a fully framed response by creating an inner payload
// [responseType][status][data...], wrapping it in a BLE packet, then a stream frame.
func buildFakeResponse(responseType, status byte, data []byte) []byte {
	inner := make([]byte, 2+len(data))
	inner[0] = responseType
	inner[1] = status
	copy(inner[2:], data)

	blePkt := BuildBLEPacket(CMD_TYPE_CONTROL, CMD_SUBTYPE_CONTROL, inner)
	return BuildStreamFrame(blePkt)
}

func TestParseResponse_Success(t *testing.T) {
	tests := []struct {
		name         string
		responseType byte
		status       byte
		data         []byte
	}{
		{
			name:         "power_ack_success",
			responseType: RESPONSE_TYPE_POWER,
			status:       STATUS_SUCCESS,
			data:         nil,
		},
		{
			name:         "brightness_ack_with_data",
			responseType: RESPONSE_TYPE_BRIGHTNESS,
			status:       STATUS_SUCCESS,
			data:         []byte{0x80},
		},
		{
			name:         "program_start_error",
			responseType: RESPONSE_TYPE_PROGRAM_START,
			status:       STATUS_ERROR,
			data:         []byte{ERR_DEVICE_BUSY},
		},
		{
			name:         "device_info_with_payload",
			responseType: RESPONSE_TYPE_DEVICE_INFO,
			status:       STATUS_SUCCESS,
			data:         make([]byte, 10),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := buildFakeResponse(tt.responseType, tt.status, tt.data)
			resp, err := ParseResponse(frame)
			if err != nil {
				t.Fatalf("ParseResponse: %v", err)
			}

			if resp.Type != tt.responseType {
				t.Errorf("Type = 0x%02X, want 0x%02X", resp.Type, tt.responseType)
			}
			if resp.Status != tt.status {
				t.Errorf("Status = 0x%02X, want 0x%02X", resp.Status, tt.status)
			}

			if tt.data == nil {
				if len(resp.Data) != 0 {
					t.Errorf("Data length = %d, want 0", len(resp.Data))
				}
			} else {
				if len(resp.Data) != len(tt.data) {
					t.Fatalf("Data length = %d, want %d", len(resp.Data), len(tt.data))
				}
				for i := range tt.data {
					if resp.Data[i] != tt.data[i] {
						t.Errorf("Data[%d] = 0x%02X, want 0x%02X", i, resp.Data[i], tt.data[i])
					}
				}
			}
		})
	}
}

func TestResponsePredicates(t *testing.T) {
	tests := []struct {
		name             string
		responseType     byte
		status           byte
		wantSuccess      bool
		wantProgramStart bool
		wantProgramData  bool
		wantDeviceInfo   bool
	}{
		{
			name:             "success_program_start",
			responseType:     RESPONSE_TYPE_PROGRAM_START,
			status:           STATUS_SUCCESS,
			wantSuccess:      true,
			wantProgramStart: true,
		},
		{
			name:            "success_program_data",
			responseType:    RESPONSE_TYPE_PROGRAM_DATA,
			status:          STATUS_SUCCESS,
			wantSuccess:     true,
			wantProgramData: true,
		},
		{
			name:           "success_device_info",
			responseType:   RESPONSE_TYPE_DEVICE_INFO,
			status:         STATUS_SUCCESS,
			wantSuccess:    true,
			wantDeviceInfo: true,
		},
		{
			name:         "error_status",
			responseType: RESPONSE_TYPE_POWER,
			status:       STATUS_ERROR,
			wantSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{Type: tt.responseType, Status: tt.status}

			if got := resp.IsSuccess(); got != tt.wantSuccess {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.wantSuccess)
			}
			if got := resp.IsProgramStart(); got != tt.wantProgramStart {
				t.Errorf("IsProgramStart() = %v, want %v", got, tt.wantProgramStart)
			}
			if got := resp.IsProgramData(); got != tt.wantProgramData {
				t.Errorf("IsProgramData() = %v, want %v", got, tt.wantProgramData)
			}
			if got := resp.IsDeviceInfo(); got != tt.wantDeviceInfo {
				t.Errorf("IsDeviceInfo() = %v, want %v", got, tt.wantDeviceInfo)
			}
		})
	}
}

func TestParseDeviceInfoResponse(t *testing.T) {
	tests := []struct {
		name        string
		resp        *Response
		wantErr     bool
		wantModel   uint8
		wantFW      string
		wantHW      string
		wantCols    uint16
		wantRows    uint16
	}{
		{
			name: "valid_device_info",
			resp: &Response{
				Type:   RESPONSE_TYPE_DEVICE_INFO,
				Status: STATUS_SUCCESS,
				Data: []byte{
					0x42,       // ModelID
					0x01, 0x02, 0x03, // FW 1.2.3
					0x04, 0x05, // HW 4.5
					0x00, 0x60, // Columns = 96 (big-endian)
					0x00, 0x10, // Rows = 16 (big-endian)
				},
			},
			wantModel: 0x42,
			wantFW:    "1.2.3",
			wantHW:    "4.5",
			wantCols:  96,
			wantRows:  16,
		},
		{
			name: "larger_display",
			resp: &Response{
				Type:   RESPONSE_TYPE_DEVICE_INFO,
				Status: STATUS_SUCCESS,
				Data: []byte{
					0x01,       // ModelID
					0x02, 0x00, 0x01, // FW 2.0.1
					0x01, 0x00, // HW 1.0
					0x01, 0x00, // Columns = 256
					0x00, 0x40, // Rows = 64
				},
			},
			wantModel: 0x01,
			wantFW:    "2.0.1",
			wantHW:    "1.0",
			wantCols:  256,
			wantRows:  64,
		},
		{
			name:    "nil_response",
			resp:    nil,
			wantErr: true,
		},
		{
			name: "data_too_short",
			resp: &Response{
				Type:   RESPONSE_TYPE_DEVICE_INFO,
				Status: STATUS_SUCCESS,
				Data:   []byte{0x01, 0x02, 0x03},
			},
			wantErr: true,
		},
		{
			name: "data_exactly_9_bytes",
			resp: &Response{
				Type:   RESPONSE_TYPE_DEVICE_INFO,
				Status: STATUS_SUCCESS,
				Data:   make([]byte, 9),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseDeviceInfoResponse(tt.resp)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDeviceInfoResponse: %v", err)
			}

			if info.ModelID != tt.wantModel {
				t.Errorf("ModelID = 0x%02X, want 0x%02X", info.ModelID, tt.wantModel)
			}
			if info.FirmwareVersion != tt.wantFW {
				t.Errorf("FirmwareVersion = %q, want %q", info.FirmwareVersion, tt.wantFW)
			}
			if info.HardwareVersion != tt.wantHW {
				t.Errorf("HardwareVersion = %q, want %q", info.HardwareVersion, tt.wantHW)
			}
			if info.Columns != tt.wantCols {
				t.Errorf("Columns = %d, want %d", info.Columns, tt.wantCols)
			}
			if info.Rows != tt.wantRows {
				t.Errorf("Rows = %d, want %d", info.Rows, tt.wantRows)
			}
		})
	}
}

func TestParseResponse_Errors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty_data",
			data: []byte{},
		},
		{
			name: "no_start_byte",
			data: []byte{0xFF, 0x00, 0x04, 0x03},
		},
		{
			name: "no_end_byte",
			data: []byte{0x01, 0x00, 0x04, 0xFF},
		},
		{
			name: "too_short_for_response",
			// Valid stream frame wrapping a BLE packet with only 1-byte payload (need at least 2).
			data: func() []byte {
				// Inner payload is just 1 byte, which is too short for a response.
				blePkt := BuildBLEPacket(CMD_TYPE_CONTROL, CMD_SUBTYPE_CONTROL, []byte{0x05})
				return BuildStreamFrame(blePkt)
			}(),
		},
		{
			name: "empty_ble_payload",
			data: func() []byte {
				blePkt := BuildBLEPacket(CMD_TYPE_CONTROL, CMD_SUBTYPE_CONTROL, nil)
				return BuildStreamFrame(blePkt)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseResponse(tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
