package jsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func frame(body []byte) []byte {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	return append([]byte(header), body...)
}

func TestReadMessageBasic(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"test"}`)
	r := bufio.NewReader(bytes.NewReader(frame(body)))

	msg, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != string(body) {
		t.Fatalf("got %q, want %q", msg, body)
	}
}

func TestReadMessageMissingContentLength(t *testing.T) {
	input := "\r\n" // blank line with no Content-Length header
	r := bufio.NewReader(strings.NewReader(input))

	_, err := ReadMessage(r)
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
	if !strings.Contains(err.Error(), "missing Content-Length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadMessageInvalidContentLength(t *testing.T) {
	input := "Content-Length: abc\r\n\r\n"
	r := bufio.NewReader(strings.NewReader(input))

	_, err := ReadMessage(r)
	if err == nil {
		t.Fatal("expected error for invalid Content-Length")
	}
	if !strings.Contains(err.Error(), "invalid Content-Length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadMessageIgnoresOtherHeaders(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0"}`)
	input := fmt.Sprintf("Content-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	r := bufio.NewReader(strings.NewReader(input))

	msg, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != string(body) {
		t.Fatalf("got %q, want %q", msg, body)
	}
}

func TestReadMessageLargePayload(t *testing.T) {
	// 64 KB payload
	payload := strings.Repeat("x", 64*1024)
	body := []byte(fmt.Sprintf(`{"data":"%s"}`, payload))
	r := bufio.NewReader(bytes.NewReader(frame(body)))

	msg, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg) != len(body) {
		t.Fatalf("got %d bytes, want %d", len(msg), len(body))
	}
}

func TestReadMessagePartialBody(t *testing.T) {
	// Claim 100 bytes but only provide 10
	input := "Content-Length: 100\r\n\r\n0123456789"
	r := bufio.NewReader(strings.NewReader(input))

	_, err := ReadMessage(r)
	if err == nil {
		t.Fatal("expected error for truncated body")
	}
}

func TestReadMessageEOFOnHeaders(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(""))
	_, err := ReadMessage(r)
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}

func TestWriteMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	msg := Response{
		JSONRPC: "2.0",
		ID:      float64(1),
		Result:  "ok",
	}
	WriteMessage(&buf, &mu, msg)

	r := bufio.NewReader(&buf)
	body, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	var got Response
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.JSONRPC != "2.0" || got.Result != "ok" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestSendResponse(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	SendResponse(&buf, &mu, float64(42), map[string]string{"status": "ok"}, nil)

	r := bufio.NewReader(&buf)
	body, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != float64(42) {
		t.Fatalf("id: got %v, want 42", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestSendResponseError(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	SendResponse(&buf, &mu, float64(1), nil, &Error{Code: -32601, Message: "method not found"})

	r := bufio.NewReader(&buf)
	body, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("error code: got %d, want -32601", resp.Error.Code)
	}
}

func TestSendNotification(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	SendNotification(&buf, &mu, "textDocument/publishDiagnostics", map[string]string{"uri": "file:///test.kt"})

	r := bufio.NewReader(&buf)
	body, err := ReadMessage(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var notif Notification
	if err := json.Unmarshal(body, &notif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("method: got %q", notif.Method)
	}
}

func TestConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	// Use a pipe so writes don't interleave in a bytes.Buffer
	pr, pw := io.Pipe()
	done := make(chan struct{})

	var messages [][]byte
	go func() {
		defer close(done)
		r := bufio.NewReader(pr)
		for i := 0; i < 20; i++ {
			msg, err := ReadMessage(r)
			if err != nil {
				return
			}
			messages = append(messages, msg)
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			SendResponse(pw, &mu, float64(id), "ok", nil)
		}(i)
	}
	wg.Wait()
	pw.Close()
	<-done

	// Should not corrupt: all must be valid JSON
	_ = buf // unused but declared for clarity
	if len(messages) != 20 {
		t.Fatalf("got %d messages, want 20", len(messages))
	}
	for i, msg := range messages {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err != nil {
			t.Fatalf("message %d: invalid JSON: %v", i, err)
		}
	}
}
