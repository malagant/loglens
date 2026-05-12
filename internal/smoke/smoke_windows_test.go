//go:build windows

package smoke_test

import "testing"

// TestTUIBootsInPTY is a no-op on Windows; we rely on the matrix Windows build
// and CI's e2e job (which uses a different harness) for runtime coverage.
func TestTUIBootsInPTY(t *testing.T) {
	t.Skip("pty smoke is unix-only; windows runtime is covered by build matrix")
}
