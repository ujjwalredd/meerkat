// Package awake keeps the machine awake during a workflow.
// Backends: macOS (caffeinate), Linux (systemd-inhibit), Windows (stub).
package awake

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

type Keeper struct {
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	backend  string
	started  bool
	maxUntil time.Time
}

// Start runs a keep-awake child process. If maxDuration <= 0, no enforced max.
func Start(maxDuration time.Duration) (*Keeper, error) {
	k := &Keeper{}
	if maxDuration > 0 {
		k.maxUntil = time.Now().Add(maxDuration)
	}
	ctx, cancel := context.WithCancel(context.Background())
	k.cancel = cancel

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("caffeinate"); err != nil {
			return nil, fmt.Errorf("awake: caffeinate not found")
		}
		// -i prevent idle sleep, -m prevent disk sleep, -s system sleep, -d display.
		args := []string{"-imsd"}
		if maxDuration > 0 {
			args = append(args, "-t", fmt.Sprintf("%d", int(maxDuration.Seconds())))
		}
		k.cmd = exec.CommandContext(ctx, "caffeinate", args...)
		k.backend = "caffeinate"
	case "linux":
		if _, err := exec.LookPath("systemd-inhibit"); err != nil {
			return nil, fmt.Errorf("awake: systemd-inhibit not found")
		}
		// hold inhibitor; sleep keeps the child alive.
		k.cmd = exec.CommandContext(ctx, "systemd-inhibit",
			"--what=idle:sleep",
			"--who=meerkat",
			"--why=meerkat workflow active",
			"--mode=block",
			"sleep", fmt.Sprintf("%d", int64(maxOrDefault(maxDuration).Seconds())))
		k.backend = "systemd-inhibit"
	case "windows":
		cancel()
		return nil, fmt.Errorf("awake: Windows keep-awake not implemented (experimental)")
	default:
		cancel()
		return nil, fmt.Errorf("awake: unsupported OS %s", runtime.GOOS)
	}

	if err := k.cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("awake: start %s: %w", k.backend, err)
	}
	k.started = true
	return k, nil
}

func maxOrDefault(d time.Duration) time.Duration {
	if d <= 0 {
		return 3 * time.Hour
	}
	return d
}

func (k *Keeper) Backend() string { return k.backend }

// Stop terminates the keep-awake child. Safe to call multiple times.
func (k *Keeper) Stop() {
	if k == nil || !k.started {
		return
	}
	k.started = false
	if k.cancel != nil {
		k.cancel()
	}
	if k.cmd != nil && k.cmd.Process != nil {
		_ = k.cmd.Process.Kill()
		_, _ = k.cmd.Process.Wait()
	}
}

// BackendAvailable reports whether a usable backend exists on this OS.
func BackendAvailable() (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("caffeinate"); err == nil {
			return "caffeinate", true
		}
	case "linux":
		if _, err := exec.LookPath("systemd-inhibit"); err == nil {
			return "systemd-inhibit", true
		}
	}
	return "", false
}
