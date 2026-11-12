package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
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

func TestStreamSinkBoundsWritesToStalledClient(t *testing.T) {
	old := streamWriteTimeout
	streamWriteTimeout = 50 * time.Millisecond
	t.Cleanup(func() { streamWriteTimeout = old })

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	sink := newConnStreamSink(bufio.NewWriter(server), server)

	// A reading client receives each frame. The second Send comes after the
	// first frame's deadline would have passed, so it proves the deadline is cleared.
	reader := bufio.NewReader(client)
	for _, text := range []string{"a", "a2"} {
		got := make(chan string, 1)
		go func() {
			line, _ := reader.ReadString('\n')
			got <- line
		}()
		if !sink.Send(text) {
			t.Fatalf("send %q to a reading client failed", text)
		}
		if line, want := <-got, "{\"stream\":\""+text+"\"}\n"; line != want {
			t.Fatalf("frame=%q want %q", line, want)
		}
		time.Sleep(2 * streamWriteTimeout)
	}

	// A client that stops reading must not block Send past the timeout.
	done := make(chan bool, 1)
	go func() { done <- sink.Send("b") }()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("send to a stalled client reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("send to a stalled client did not time out")
	}
	if sink.Send("c") {
		t.Fatal("send after a timed-out write succeeded")
	}
}
