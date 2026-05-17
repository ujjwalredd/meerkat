package processrunner

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type Result struct {
	ExitCode   int
	DurationMs int64
	Err        error
}

// Run executes argv with inherited stdio. Forwards signals.
func Run(argv []string) Result {
	if len(argv) == 0 {
		return Result{ExitCode: 2, Err: errEmpty}
	}
	start := time.Now()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return Result{ExitCode: 127, Err: err, DurationMs: time.Since(start).Milliseconds()}
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	for {
		select {
		case s := <-sigs:
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, s.(syscall.Signal))
			}
		case err := <-done:
			r := Result{DurationMs: time.Since(start).Milliseconds()}
			if err == nil {
				r.ExitCode = 0
				return r
			}
			if ee, ok := err.(*exec.ExitError); ok {
				r.ExitCode = ee.ExitCode()
				return r
			}
			r.ExitCode = 1
			r.Err = err
			return r
		}
	}
}

type strErr string

func (e strErr) Error() string { return string(e) }

const errEmpty = strErr("empty command")
