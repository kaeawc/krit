package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/diag"
)

type verboseBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	// sink, when set, receives writes live while it accepts them. Text it
	// rejects (sealed or failed) stays in buffer for the final response.
	sink *daemon.StreamSink
}

// streamVerboseOutput sends the request's verbose output live over the
// connection when the client asked for it. Strict-verify runs the pipeline
// twice and resets the buffer between runs, so it keeps the buffered form.
func streamVerboseOutput(ctx context.Context, state *daemonState, args daemon.AnalyzeProjectArgs, reporter *diag.Reporter) {
	if !args.StreamVerbose || state.strictVerify || reporter == nil {
		return
	}
	if buffer, ok := reporter.Verbose.(*verboseBuffer); ok {
		buffer.attachSink(daemon.StreamSinkFrom(ctx))
	}
}

// attachSink starts live streaming and flushes text buffered so far.
func (b *verboseBuffer) attachSink(sink *daemon.StreamSink) {
	if sink == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buffer.Len() > 0 && sink.Send(b.buffer.String()) {
		b.buffer.Reset()
	}
	b.sink = sink
}

func newVerboseReporter(enabled bool) *diag.Reporter {
	if !enabled {
		return nil
	}
	buffer := &verboseBuffer{}
	return &diag.Reporter{Verbose: buffer, Warning: buffer}
}

func resetVerboseOutput(reporter *diag.Reporter) {
	if reporter == nil {
		return
	}
	if buffer, ok := reporter.Verbose.(*verboseBuffer); ok {
		buffer.mu.Lock()
		defer buffer.mu.Unlock()
		buffer.buffer.Reset()
	}
}

func (b *verboseBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sink != nil && b.sink.Send(string(data)) {
		return len(data), nil
	}
	return b.buffer.Write(data)
}

func writeVerboseOutput(w io.Writer, reporter *diag.Reporter) error {
	if reporter == nil {
		return nil
	}
	buffer, ok := reporter.Verbose.(*verboseBuffer)
	if !ok {
		return nil
	}
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.buffer.Len() == 0 {
		return nil
	}
	output, err := json.Marshal(buffer.buffer.String())
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(`,"verbose_output":`)); err != nil {
		return err
	}
	_, err = w.Write(output)
	return err
}
