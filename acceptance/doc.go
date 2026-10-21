// Package acceptance holds the executable specification of usechrome-mcp:
// tests that drive the built binary over stdio MCP against real Chromium.
// They need the acceptance build tag:
//
//	go test -tags acceptance ./acceptance/...
//
// Set USECHROME_MCP_BIN to test a binary that is already built.
package acceptance
