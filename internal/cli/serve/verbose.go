package serve

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	"github.com/kaeawc/krit/internal/diag"
)

type verboseBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
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
