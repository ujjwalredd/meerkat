package processrunner

import "time"

type Result struct {
	ExitCode   int
	DurationMs int64
	Err        error
}

// Run executes argv with inherited stdio. Forwards signals.
// Implementation is OS-specific (see runner_unix.go, runner_windows.go).
func Run(argv []string) Result {
	if len(argv) == 0 {
		return Result{ExitCode: 2, Err: errEmpty, DurationMs: 0}
	}
	start := time.Now()
	r := runOS(argv)
	if r.DurationMs == 0 {
		r.DurationMs = time.Since(start).Milliseconds()
	}
	return r
}

type strErr string

func (e strErr) Error() string { return string(e) }

const errEmpty = strErr("empty command")
