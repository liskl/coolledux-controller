package controller

import (
	"strings"
	"testing"
)

func TestDeviceState_String(t *testing.T) {
	tests := []struct {
		state DeviceState
		want  string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateDisconnecting, "disconnecting"},
		{StateError, "error"},
		{DeviceState(99), "DeviceState(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("DeviceState(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestProgramSendingState_String(t *testing.T) {
	tests := []struct {
		state ProgramSendingState
		want  string
	}{
		{ProgramIdle, "idle"},
		{ProgramSendingStart, "sending_start"},
		{ProgramSendingData, "sending_data"},
		{ProgramComplete, "complete"},
		{ProgramError, "error"},
		{ProgramSendingState(99), "ProgramSendingState(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("ProgramSendingState(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestDeviceState_UnknownContainsTypeName(t *testing.T) {
	got := DeviceState(42).String()
	if !strings.Contains(got, "DeviceState") {
		t.Errorf("unknown DeviceState string = %q, want to contain 'DeviceState'", got)
	}
}

func TestProgramSendingState_UnknownContainsTypeName(t *testing.T) {
	got := ProgramSendingState(42).String()
	if !strings.Contains(got, "ProgramSendingState") {
		t.Errorf("unknown ProgramSendingState string = %q, want to contain 'ProgramSendingState'", got)
	}
}

func TestDeviceState_IotaValues(t *testing.T) {
	// Verify the iota ordering is as expected.
	if StateDisconnected != 0 {
		t.Errorf("StateDisconnected = %d, want 0", StateDisconnected)
	}
	if StateConnecting != 1 {
		t.Errorf("StateConnecting = %d, want 1", StateConnecting)
	}
	if StateConnected != 2 {
		t.Errorf("StateConnected = %d, want 2", StateConnected)
	}
	if StateDisconnecting != 3 {
		t.Errorf("StateDisconnecting = %d, want 3", StateDisconnecting)
	}
	if StateError != 4 {
		t.Errorf("StateError = %d, want 4", StateError)
	}
}

func TestProgramSendingState_IotaValues(t *testing.T) {
	if ProgramIdle != 0 {
		t.Errorf("ProgramIdle = %d, want 0", ProgramIdle)
	}
	if ProgramSendingStart != 1 {
		t.Errorf("ProgramSendingStart = %d, want 1", ProgramSendingStart)
	}
	if ProgramSendingData != 2 {
		t.Errorf("ProgramSendingData = %d, want 2", ProgramSendingData)
	}
	if ProgramComplete != 3 {
		t.Errorf("ProgramComplete = %d, want 3", ProgramComplete)
	}
	if ProgramError != 4 {
		t.Errorf("ProgramError = %d, want 4", ProgramError)
	}
}
