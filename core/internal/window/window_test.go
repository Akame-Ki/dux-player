package window

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A log that passes its limit while the UI runs moves to .1 and the new
// output starts a fresh file.
func TestRotatingLogRotatesWhileRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui.log")
	l, err := newRotatingLog(path, 10)
	require.NoError(t, err)
	defer l.Close()

	for _, s := range []string{"123456", "789012", "tail"} {
		_, err := l.Write([]byte(s))
		require.NoError(t, err)
	}

	cur, err := os.ReadFile(path)
	require.NoError(t, err)
	old, err := os.ReadFile(path + ".1")
	require.NoError(t, err)
	assert.Equal(t, "789012tail", string(cur))
	assert.Equal(t, "123456", string(old))
}
