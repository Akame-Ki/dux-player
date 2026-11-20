package devices

import (
	"bufio"
	"context"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-go/log"
)

// defaultSinkMoved is how long after the default moved away a removed sink
// still counts as the default: the two events arrive in either order.
const defaultSinkMoved = 3 * time.Second

var defaultSinkLine = regexp.MustCompile(`key:'default\.audio\.sink' value:'\{ *"name": *"([^"]+)" *\}'`)

func defaultSinkIn(line string) string {
	if m := defaultSinkLine.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// defaultSink follows PipeWire's default output, so "System default" can tell
// its output going away from the user picking another one.
type defaultSink struct {
	mu      sync.Mutex
	now     string
	was     string
	movedAt time.Time
}

func (d *defaultSink) set(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if name != d.now {
		d.was, d.now, d.movedAt = d.now, name, time.Now()
	}
}

// lost says id was the default when it went away.
func (d *defaultSink) lost(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return id == d.now || id == d.was && time.Since(d.movedAt) < defaultSinkMoved
}

// watch runs until ctx ends, starting pw-metadata again if PipeWire restarts.
// A failure is logged once, and again only after pw-metadata worked in between.
func (d *defaultSink) watch(ctx context.Context) {
	warned := false
	for ctx.Err() == nil {
		worked, err := d.follow(ctx)
		warned = warned && !worked
		if err != nil && ctx.Err() == nil && !warned {
			log.Warnf("devices: following the default output: %v", err)
			warned = true
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

// follow reports whether it saw the default output before it ended.
func (d *defaultSink) follow(ctx context.Context) (worked bool, err error) {
	cmd := exec.CommandContext(ctx, "pw-metadata", "-n", "default", "-m")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	lines := bufio.NewScanner(out)
	for lines.Scan() {
		if name := defaultSinkIn(lines.Text()); name != "" {
			d.set(name)
			worked = true
		}
	}
	return worked, cmd.Wait()
}
