// Package mqtt holds integration tests that talk to a live MQTT broker. Test
// files here must carry the build tag:
//
//	//go:build integration && mqtt
//
// Run with:  make test-integration-mqtt
// Requires:  COOLLEDUX_TEST_BROKER (e.g. tcp://localhost:1883).
package mqtt
