package devices

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// PATH holds only failing stand-ins for the tools that watch or query the
// host's audio and sleep, so no test reaches the real PipeWire or PulseAudio. A test
// that needs other tools sets its own PATH.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dplayer-devices-bin")
	if err != nil {
		panic(err)
	}
	for _, tool := range []string{"pw-metadata", "pw-dump", "wpctl", "pactl"} {
		if err := os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
			panic(err)
		}
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		panic(err)
	}
	if err := os.Symlink(sleep, filepath.Join(dir, "sleep")); err != nil {
		panic(err)
	}
	os.Setenv("PATH", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
