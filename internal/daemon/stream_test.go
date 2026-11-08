package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func startStreamServer(t *testing.T, h Handler) string {
	t.Helper()
	socket := filepath.Join(shortTempDir(t), "d.sock")
	srv := NewServer(socket)
	srv.Register("stream", h)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	return socket
}

func TestCallStreamDeliversFramesBeforeResponse(t *testing.T) {
	release := make(chan struct{})
	socket := startStreamServer(t, func(ctx context.Context, _ json.RawMessage) (any, error) {
		sink := StreamSinkFrom(ctx)
		sink.Send("first\n")
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			return nil, context.DeadlineExceeded
		}
		sink.Send("second \"quoted\"\n")
		return "done", nil
	})
	var frames []string
	var got string
	err := CallStream(socket, "stream", nil, &got, func(text string) {
		frames = append(frames, text)
		if len(frames) == 1 {
			close(release) // the handler is still blocked, so this frame arrived live
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "done" || len(frames) != 2 || frames[0] != "first\n" || frames[1] != "second \"quoted\"\n" {
		t.Fatalf("got=%q frames=%q", got, frames)
	}
}

func TestCallIgnoresFramesAndPlainHandlersSendNone(t *testing.T) {
	socket := startStreamServer(t, func(ctx context.Context, raw json.RawMessage) (any, error) {
		if len(raw) > 0 {
			StreamSinkFrom(ctx).Send("progress\n")
		}
		return "ok", nil
	})
	for _, args := range []any{nil, true} {
		var got string
		if err := Call(socket, "stream", args, &got); err != nil || got != "ok" {
			t.Fatalf("args=%v got=%q err=%v", args, got, err)
		}
	}
}

func TestStreamSinkSealStopsFrames(t *testing.T) {
	var buf bytes.Buffer
	sink := NewStreamSink(bufio.NewWriter(&buf))
	if !sink.Send("a") {
		t.Fatal("send before seal failed")
	}
	sink.Seal()
	if sink.Send("b") {
		t.Fatal("send after seal succeeded")
	}
	if got := buf.String(); got != "{\"stream\":\"a\"}\n" {
		t.Fatalf("wire=%q", got)
	}
	var nilSink *StreamSink
	nilSink.Seal()
	if nilSink.Send("x") {
		t.Fatal("nil sink accepted a frame")
	}
}

func TestDecodeStreamFrameRejectsResponses(t *testing.T) {
	if _, ok := decodeStreamFrame([]byte("{\"ok\":true,\"data\":1}\n")); ok {
		t.Fatal("response parsed as frame")
	}
	if text, ok := decodeStreamFrame([]byte("{\"stream\":\"hi\"}\n")); !ok || text != "hi" {
		t.Fatalf("text=%q ok=%v", text, ok)
	}
}
