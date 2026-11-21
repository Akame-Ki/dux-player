// Package testenv tells tests about the machine they run on.
package testenv

import (
	"os"
	"testing"
)

// NeedsPermissions is for a test of a file permission refusing access, which
// root ignores. As root it skips, and fails under CI, whose job runs the tests
// as a normal user so these count.
func NeedsPermissions(t testing.TB) {
	t.Helper()
	if os.Geteuid() != 0 {
		return
	}
	if os.Getenv("CI") != "" {
		t.Fatal("running as root, which ignores file permissions: CI must run the tests as a normal user")
	}
	t.Skip("root ignores file permissions")
}

// maxTempDir leaves room under a unix socket's 107-byte path limit for a
// test's name and a socket file.
const maxTempDir = 40

// ShortTempDir points TMPDIR at /tmp when the current one is too long for a
// socket under t.TempDir(). Call it from TestMain.
func ShortTempDir() {
	if len(os.TempDir()) > maxTempDir {
		_ = os.Setenv("TMPDIR", "/tmp")
	}
}
