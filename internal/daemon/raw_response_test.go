package daemon

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

// rawResponseFake is a test handler result that satisfies the
// RawResponseWriter interface. It writes a fixed payload so the test
// can assert dispatchAndWrite chose the streaming path instead of the
// default json.Marshal envelope.
type rawResponseFake struct {
	payload []byte
	called  *bool
}

func (r *rawResponseFake) WriteRawResponse(_ context.Context, w io.Writer) error {
	if r.called != nil {
		*r.called = true
	}
	_, err := w.Write(r.payload)
	return err
}

// TestRawResponseWriter_StreamingPathBypassesEnvelope verifies that a
// handler returning a value implementing RawResponseWriter has its
// WriteRawResponse method invoked verbatim — dispatch must NOT wrap
// the result in the standard `{"ok":true,"data":...}` envelope. This
// is the seam the analyze-project verb uses to stream its findings
// JSON directly into the connection (#60).
func TestRawResponseWriter_StreamingPathBypassesEnvelope(t *testing.T) {
	srv := NewServer("")
	called := false
	srv.Register("stream", func(_ context.Context, _ json.RawMessage) (any, error) {
		return &rawResponseFake{
			payload: []byte(`{"ok":true,"data":{"hello":"world"}}` + "\n"),
			called:  &called,
		}, nil
	})

	pr, pw := io.Pipe()
	go func() {
		_ = srv.dispatchAndWrite(context.Background(),
			Request{Verb: "stream"}, pw)
		_ = pw.Close()
	}()

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !called {
		t.Fatal("WriteRawResponse should have been invoked")
	}
	want := `{"ok":true,"data":{"hello":"world"}}` + "\n"
	if string(got) != want {
		t.Errorf("dispatchAndWrite output:\n got %q\nwant %q", got, want)
	}
}

// TestRawResponseWriter_FallbackEnvelopeForRegularResult is the
// counterpart contract: a handler returning a plain value (no
// RawResponseWriter) still goes through the json.Marshal + envelope
// path so existing verbs keep their wire shape.
func TestRawResponseWriter_FallbackEnvelopeForRegularResult(t *testing.T) {
	srv := NewServer("")
	srv.Register("echo", func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]string{"k": "v"}, nil
	})

	pr, pw := io.Pipe()
	go func() {
		_ = srv.dispatchAndWrite(context.Background(),
			Request{Verb: "echo"}, pw)
		_ = pw.Close()
	}()

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := `{"ok":true,"data":{"k":"v"}}` + "\n"
	if string(got) != want {
		t.Errorf("dispatchAndWrite output:\n got %q\nwant %q", got, want)
	}
}
