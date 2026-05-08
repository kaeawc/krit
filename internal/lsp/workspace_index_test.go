package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

func TestSourceWorkspaceIndexerBuildsDeclarationIndex(t *testing.T) {
	root := t.TempDir()
	writeLSPTestFile(t, filepath.Join(root, "src", "main", "kotlin", "test", "A.kt"), `
package test
class A
fun top() = Unit
`)
	idx, err := SourceWorkspaceIndexer{}.BuildWorkspaceIndex(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.FindDeclarationByFQN("test.A"); !ok {
		t.Fatal("missing declaration test.A")
	}
	if top, ok := idx.FindDeclarationByFQN("test.top"); !ok || top.Line == 0 {
		t.Fatalf("missing source-positioned top function declaration: %+v ok=%v", top, ok)
	}
}

func TestInitializeStartsWorkspaceIndexWithoutBlocking(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	fake := &blockingWorkspaceIndexer{started: started, release: release}

	output := newSyncBuffer()
	server := NewServer(nil, output)
	server.SetWorkspaceIndexer(fake)
	defer server.cancelWorkspaceIndex()

	req := initializeRequest(t, pathToURI(t.TempDir()), nil)
	server.handleInitialize(req)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("indexer did not start")
	}
	msgs, err := collectMessages(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected initialize response before indexer release")
	}
	var resp Response
	if err := json.Unmarshal(msgs[0], &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID == nil {
		t.Fatalf("first message is not initialize response: %s", msgs[0])
	}
	server.cancelWorkspaceIndex()
}

func TestWorkspaceIndexInstallsOracleIndexAndProgress(t *testing.T) {
	output := newSyncBuffer()
	server := NewServer(nil, output)
	server.SetWorkspaceIndexer(staticWorkspaceIndexer{idx: workspaceIndexFixture()})

	server.handleInitialize(initializeRequest(t, pathToURI(t.TempDir()), nil))
	deadline := time.Now().Add(time.Second)
	for server.oracleIndex() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if server.oracleIndex() == nil {
		t.Fatal("workspace index was not installed")
	}
	// Wait for the progress "end" notification to land on the buffer
	// before reading. The workspace-index goroutine writes after
	// installing oracleIndex, so the install signal alone doesn't
	// guarantee the notification has been flushed.
	output.waitUntil(time.Second, func(b []byte) bool {
		return bytes.Contains(b, []byte(`"kind":"end"`))
	})
	msgs, err := collectMessages(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var sawEnd bool
	for _, msg := range msgs {
		var n Notification
		if err := json.Unmarshal(msg, &n); err == nil && n.Method == "$/progress" {
			params, _ := json.Marshal(n.Params)
			if bytes.Contains(params, []byte(`"kind":"end"`)) {
				sawEnd = true
			}
		}
	}
	if !sawEnd {
		t.Fatalf("missing progress end notification; messages=%d", len(msgs))
	}
}

func TestWorkspaceIndexSkipsProgressWhenClientDoesNotSupportIt(t *testing.T) {
	output := newSyncBuffer()
	server := NewServer(nil, output)
	server.SetWorkspaceIndexer(staticWorkspaceIndexer{idx: workspaceIndexFixture()})
	req := initializeRequest(t, pathToURI(t.TempDir()), nil)
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatal(err)
	}
	params.Capabilities.Window = &WindowClientCapabilities{WorkDoneProgress: false}
	raw, _ := json.Marshal(params)
	req.Params = raw

	server.handleInitialize(req)
	deadline := time.Now().Add(time.Second)
	for server.oracleIndex() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	msgs, err := collectMessages(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range msgs {
		var n Notification
		if err := json.Unmarshal(msg, &n); err == nil && n.Method == "$/progress" {
			t.Fatalf("unexpected progress notification: %s", msg)
		}
	}
}

func TestInitializeCanDisableWorkspaceIndex(t *testing.T) {
	called := make(chan struct{}, 1)
	fake := staticWorkspaceIndexer{called: called, idx: workspaceIndexFixture()}
	disabled := false
	opts, _ := json.Marshal(InitOptions{IndexOnInitialize: &disabled})

	server := NewServer(nil, &bytes.Buffer{})
	server.SetWorkspaceIndexer(fake)
	server.handleInitialize(initializeRequest(t, pathToURI(t.TempDir()), opts))

	select {
	case <-called:
		t.Fatal("indexer should not run when indexOnInitialize=false")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWaitForOracleIndexReturnsWhenInFlightIndexCompletes(t *testing.T) {
	server := NewServer(nil, &bytes.Buffer{})
	ready := make(chan struct{})
	server.indexMu.Lock()
	server.indexReady = ready
	server.indexMu.Unlock()
	go func() {
		time.Sleep(10 * time.Millisecond)
		server.SetOracleIndex(workspaceIndexFixture())
		close(ready)
	}()
	if idx := server.waitForOracleIndex(time.Second); idx == nil {
		t.Fatal("expected index after ready signal")
	}
}

type blockingWorkspaceIndexer struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingWorkspaceIndexer) BuildWorkspaceIndex(ctx context.Context, _ string, _ WorkspaceIndexProgress) (*oracle.Index, error) {
	close(b.started)
	select {
	case <-b.release:
		return workspaceIndexFixture(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type staticWorkspaceIndexer struct {
	called chan struct{}
	idx    *oracle.Index
}

func (s staticWorkspaceIndexer) BuildWorkspaceIndex(context.Context, string, WorkspaceIndexProgress) (*oracle.Index, error) {
	if s.called != nil {
		s.called <- struct{}{}
	}
	return s.idx, nil
}

func workspaceIndexFixture() *oracle.Index {
	return oracle.BuildIndex(&oracle.Data{Version: 1, Files: map[string]*oracle.File{
		"/tmp/A.kt": {Declarations: []*oracle.Class{{FQN: "test.A", Kind: "class", Line: 1, Column: 1}}},
	}})
}

func initializeRequest(t *testing.T, rootURI string, initOptions json.RawMessage) *Request {
	t.Helper()
	params := InitializeParams{RootURI: rootURI, InitializationOptions: initOptions}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return &Request{JSONRPC: "2.0", ID: float64(1), Method: "initialize", Params: raw}
}

func writeLSPTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
