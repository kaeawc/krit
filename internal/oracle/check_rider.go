package oracle

import (
	"errors"
	"fmt"
	"os"

	"github.com/kaeawc/krit/internal/perf"
)

// CheckRider asks the krit-fir oracle compilation to run a `check` request
// too (#739). Without it, a --fir scan compiles the module twice: once for
// the oracle's facts and once for the FIR rule checkers, which repeats
// nearly all of the compiler's work. krit-fir shares the compilation only
// when the check compiles the same sources in the same context, and
// otherwise leaves the response empty so the checker pass compiles on its
// own.
type CheckRider struct {
	// Prepare returns the check request JSON, in the krit-fir daemon's
	// shape, and the function that receives the response JSON when krit-fir
	// shared the compilation (it is not called otherwise). It runs only
	// when the oracle launches its one-shot compile, so a run the oracle
	// cache answers pays nothing; a nil request rides nothing.
	Prepare func() (request []byte, deliver func(response []byte))
}

// checkRiderRun is one oracle process's share of a CheckRider: the files it
// passes to krit-fir and reads back.
type checkRiderRun struct {
	deliver              func([]byte)
	requestPath, outPath string
}

// startCheckRider writes the rider's request for the oracle process and
// returns the krit-fir arguments that point at it. Nil when opts carries no
// rider or the backend is not krit-fir.
func startCheckRider(opts InvocationOptions) (*checkRiderRun, []string, error) {
	rider := opts.CheckRider
	if rider == nil || rider.Prepare == nil || opts.Backend != BackendFIR {
		return nil, nil, nil
	}
	body, deliver := rider.Prepare()
	if len(body) == 0 || deliver == nil {
		return nil, nil, nil
	}
	request, err := os.CreateTemp("", "krit-fir-check-request-*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("tempfile (check request): %w", err)
	}
	_, writeErr := request.Write(body)
	closeErr := request.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(request.Name())
		return nil, nil, fmt.Errorf("write check request: %w", errors.Join(writeErr, closeErr))
	}
	out, err := os.CreateTemp("", "krit-fir-check-out-*.json")
	if err != nil {
		_ = os.Remove(request.Name())
		return nil, nil, fmt.Errorf("tempfile (check out): %w", err)
	}
	_ = out.Close()
	run := &checkRiderRun{deliver: deliver, requestPath: request.Name(), outPath: out.Name()}
	return run, []string{"--check-request", run.requestPath, "--check-out", run.outPath}, nil
}

// finish delivers the check response after a successful oracle process.
// krit-fir writes the response before the oracle output, so it is complete
// once the process has produced its output.
func (r *checkRiderRun) finish(tracker perf.Tracker, processErr error) {
	if r == nil {
		return
	}
	status := "notShared"
	var size int64
	switch data, err := os.ReadFile(r.outPath); {
	case processErr != nil:
		status = "oracleFailed"
	case err != nil:
		status = "unreadable"
	case len(data) > 0:
		status = "shared"
		size = int64(len(data))
		r.deliver(data)
	}
	addOracleInstant(tracker, "firCheckRider", map[string]int64{"responseBytes": size}, map[string]string{"status": status})
}

// cleanup removes the rider's files; safe on a nil run.
func (r *checkRiderRun) cleanup() {
	if r == nil {
		return
	}
	_ = os.Remove(r.requestPath)
	_ = os.Remove(r.outPath)
}
