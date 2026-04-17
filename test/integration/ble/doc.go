// Package ble holds integration tests that talk to a real CoolLEDUX panel
// over BLE. Test files here must carry the build tag:
//
//	//go:build integration && ble_hw
//
// Run with:  make test-integration-ble
// Requires:  COOLLEDUX_TEST_MAC pointing at a live panel.
package ble
