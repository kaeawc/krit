package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kaeawc/krit/internal/jsonrpc"
)

// TestUnknownNotificationProducesNoResponse verifies JSON-RPC 2.0 §4.1: a
// notification (a request without an `id` member) MUST NOT receive a
// response, even when the method is unknown to the server. Pre-fix, the
// dispatcher's default case already guarded on req.ID, but several known-
// method handlers (handleToolsList, handleInitialize, ...) called
// sendResponse unconditionally and emitted `"id": null` replies for
// notifications. The gate now lives in sendResponse itself.
func TestUnknownNotificationProducesNoResponse(t *testing.T) {
	notif := []byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":42}}` + "\n")

	var output bytes.Buffer
	reader := bufio.NewReader(bytes.NewReader(notif))
	server := NewServer(reader, &output)
	server.buildDispatcher()

	for {
		msg, err := jsonrpc.ReadMessageNDJSON(reader)
		if err != nil {
			break
		}
		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		server.handleMessage(&req)
	}

	if output.Len() != 0 {
		t.Fatalf("notification must produce zero response bytes; got %d bytes: %s", output.Len(), output.Bytes())
	}
}

// TestKnownMethodAsNotificationProducesNoResponse exercises the regression
// directly: a client invokes a known request method without an id.
// tools/list is normally a request — its handler unconditionally calls
// sendResponse(req.ID, ...). Pre-fix that emitted a several-kilobyte
// response with `"id": null`. Post-fix sendResponse short-circuits when
// the id is nil.
func TestKnownMethodAsNotificationProducesNoResponse(t *testing.T) {
	notif := []byte(`{"jsonrpc":"2.0","method":"tools/list"}` + "\n")

	var output bytes.Buffer
	reader := bufio.NewReader(bytes.NewReader(notif))
	server := NewServer(reader, &output)
	server.buildDispatcher()

	for {
		msg, err := jsonrpc.ReadMessageNDJSON(reader)
		if err != nil {
			break
		}
		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		server.handleMessage(&req)
	}

	if output.Len() != 0 {
		t.Fatalf("known method sent as notification must produce zero response bytes; got %d bytes: %s", output.Len(), output.Bytes())
	}
}

// TestRequestStillGetsResponseAfterNotificationFix is a sanity check that
// ordinary requests still get replies — the notification gate must not
// affect the request path.
func TestRequestStillGetsResponseAfterNotificationFix(t *testing.T) {
	out := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(7),
		Method:  "tools/list",
	})
	if len(out) != 1 {
		t.Fatalf("expected 1 response, got %d", len(out))
	}
	if out[0].ID == nil {
		t.Errorf("expected non-null id in response, got null")
	}
}
