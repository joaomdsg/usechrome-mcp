//go:build acceptance

package acceptance_test

import (
	"os"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestMain(m *testing.M) {
	os.Exit(dsl.Run(m))
}
