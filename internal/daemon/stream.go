package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Streamed progress frames let a verb send text to the client before its
// final Response. A frame is one line, `{"stream":"<text>"}`, and never
// starts with `{"ok"`, so a client tells it apart from the final response
// by prefix. Only a client that sets a verb's opt-in argument (for
// analyze-project, AnalyzeProjectArgs.StreamVerbose) receives frames, so an
// older client never sees one. An older daemon ignores the argument and
// returns the same text in the final response instead.

// streamFramePrefix starts every streamed frame line.
var streamFramePrefix = []byte(`{"stream":`)

type streamFrame struct {
	Stream string `json:"stream"`
}

// streamWriteTimeout bounds one frame write. Handlers send frames while
// holding analysis locks, so a client that stops reading must not stall them.
var streamWriteTimeout = 10 * time.Second

// writeDeadliner is the part of net.Conn a sink uses to bound its writes.
type writeDeadliner interface {
	SetWriteDeadline(t time.Time) error
}

// StreamSink writes streamed frames to one connection. Safe for concurrent
// use. Seal stops further frames, which a handler calls before it begins
// the final response so a frame can never land inside that envelope.
type StreamSink struct {
	mu     sync.Mutex
	w      *bufio.Writer
	conn   writeDeadliner
	sealed bool
}

// NewStreamSink returns a sink that writes frames to w and flushes each.
func NewStreamSink(w *bufio.Writer) *StreamSink { return &StreamSink{w: w} }

// newConnStreamSink returns a sink whose frame writes to conn time out after
// streamWriteTimeout. A timed-out write fails w, so the connection's final
// response fails too and the server closes it.
func newConnStreamSink(w *bufio.Writer, conn writeDeadliner) *StreamSink {
	return &StreamSink{w: w, conn: conn}
}

// Send writes text as one frame and flushes it. It returns false when the
// sink is nil, sealed, or the write failed, so the caller keeps the text
// for the final response.
func (s *StreamSink) Send(text string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed {
		return false
	}
	buf, err := json.Marshal(streamFrame{Stream: text})
	if err != nil {
		return false
	}
	buf = append(buf, '\n')
	if s.conn != nil {
		if err := s.conn.SetWriteDeadline(time.Now().Add(streamWriteTimeout)); err != nil {
			s.sealed = true
			return false
		}
		defer func() { _ = s.conn.SetWriteDeadline(time.Time{}) }()
	}
	if _, err := s.w.Write(buf); err != nil {
		s.sealed = true
		return false
	}
	if err := s.w.Flush(); err != nil {
		s.sealed = true
		return false
	}
	return true
}

// Seal makes every later Send return false. Nil-safe.
func (s *StreamSink) Seal() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sealed = true
	s.mu.Unlock()
}

type streamSinkKey struct{}

// WithStreamSink returns ctx carrying sink.
func WithStreamSink(ctx context.Context, sink *StreamSink) context.Context {
	return context.WithValue(ctx, streamSinkKey{}, sink)
}

// StreamSinkFrom returns the sink the server attached to a request context,
// or nil when there is none.
func StreamSinkFrom(ctx context.Context) *StreamSink {
	sink, _ := ctx.Value(streamSinkKey{}).(*StreamSink)
	return sink
}

// decodeStreamFrame returns the frame text when line is a stream frame.
func decodeStreamFrame(line []byte) (string, bool) {
	if !bytes.HasPrefix(line, streamFramePrefix) {
		return "", false
	}
	var frame streamFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return "", false
	}
	return frame.Stream, true
}
