package controller

import "fmt"

// DeviceState represents the BLE connection lifecycle state.
type DeviceState int

const (
	StateDisconnected DeviceState = iota
	StateConnecting
	StateConnected
	StateDisconnecting
	StateError
)

var deviceStateNames = map[DeviceState]string{
	StateDisconnected:  "disconnected",
	StateConnecting:    "connecting",
	StateConnected:     "connected",
	StateDisconnecting: "disconnecting",
	StateError:         "error",
}

func (s DeviceState) String() string {
	if name, ok := deviceStateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("DeviceState(%d)", s)
}

// ProgramSendingState tracks progress of a multi-packet program upload.
type ProgramSendingState int

const (
	ProgramIdle ProgramSendingState = iota
	ProgramSendingStart
	ProgramSendingData
	ProgramComplete
	ProgramError
)

var programSendingStateNames = map[ProgramSendingState]string{
	ProgramIdle:         "idle",
	ProgramSendingStart: "sending_start",
	ProgramSendingData:  "sending_data",
	ProgramComplete:     "complete",
	ProgramError:        "error",
}

func (s ProgramSendingState) String() string {
	if name, ok := programSendingStateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("ProgramSendingState(%d)", s)
}
