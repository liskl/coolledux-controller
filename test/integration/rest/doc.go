// Package rest holds integration tests that exercise the REST API of a
// running coolledux-controller service. Test files here must carry the build
// tag:
//
//	//go:build integration && rest
//
// Run with:  make test-integration-rest
// Requires:  COOLLEDUX_TEST_API (e.g. http://localhost:8080) pointing at a
// running service. Start one first with `make run` or `make docker-up`.
package rest
