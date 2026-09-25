package firchecks

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

// fakeFirDaemon wires a FirDaemon to an in-process peer that captures the
// request line and replies with response (a single wire line).
func fakeFirDaemon(t *testing.T, response string) (*FirDaemon, <-chan []byte) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	requests := make(chan []byte, 1)
	go func() {
		in := bufio.NewScanner(server)
		if !in.Scan() {
			return
		}
		requests <- append([]byte(nil), in.Bytes()...)
		_, _ = server.Write([]byte(response + "\n"))
	}()
	return &FirDaemon{conn: client, reader: bufio.NewScanner(client), nextID: 1, started: true}, requests
}

func TestCheckSendsConfiguredRuleOptionsAsRuleConfigs(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Set("coroutines", "InjectDispatcher", "active", true)
	cfg.Set("coroutines", "InjectDispatcher", "excludes", []interface{}{"**/gen/**"})
	cfg.Set("coroutines", "InjectDispatcher", "dispatcherNames", []interface{}{"IO", "Default"})
	cfg.Set("coroutines", "InjectDispatcher", "nested", map[interface{}]interface{}{"k": 1})
	ruleConfigs := RuleConfigs{"InjectDispatcher": cfg.RuleOptions("coroutines", "InjectDispatcher")}

	d, requests := fakeFirDaemon(t, `{"id":1,"succeeded":1,"skipped":0,"findings":[],"rules":[],"crashed":{}}`)
	if _, err := d.Check([]fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, ruleConfigs, FileFacts{}); err != nil {
		t.Fatalf("Check: %v", err)
	}
	var sent struct {
		RuleConfigs map[string]map[string]any `json:"ruleConfigs"`
	}
	raw := <-requests
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatalf("request is not JSON: %v\n%s", err, raw)
	}
	opts := sent.RuleConfigs["InjectDispatcher"]
	names, _ := opts["dispatcherNames"].([]any)
	if len(opts) != 2 || len(names) != 2 || names[0] != "IO" {
		t.Fatalf("ruleConfigs = %#v; want dispatcherNames and nested only\n%s", sent.RuleConfigs, raw)
	}
	if nested, _ := opts["nested"].(map[string]any); nested["k"] != float64(1) {
		t.Fatalf("nested option not encoded: %#v", opts["nested"])
	}
}

func TestCheckOmitsRuleConfigsWhenNoneConfigured(t *testing.T) {
	d, requests := fakeFirDaemon(t, `{"id":1,"succeeded":0,"skipped":0,"findings":[],"rules":[],"crashed":{}}`)
	if _, err := d.Check(nil, nil, nil, []string{"A"}, RuleConfigs{"A": {}}, FileFacts{}); err != nil {
		t.Fatalf("Check: %v", err)
	}
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(<-requests, &sent); err != nil {
		t.Fatal(err)
	}
	if _, ok := sent["ruleConfigs"]; ok {
		t.Fatalf("expected ruleConfigs to be omitted, got %s", sent["ruleConfigs"])
	}
}

// The daemon reply is one line; control characters in a message arrive
// escaped (as krit-fir's escJson now writes them) and decode intact.
func TestCheckDecodesEscapedControlCharactersInMessages(t *testing.T) {
	response := `{"id":1,"succeeded":1,"skipped":0,"findings":[{"path":"/src/A.kt","line":1,"col":1,` +
		`"rule":"InjectDispatcher","severity":"warning","message":"a\nb\r\tc\u0001\"q\\","confidence":1.0}],` +
		`"rules":["InjectDispatcher"],"crashed":{"/src/B.kt":"boom\nat B.kt:1"}}`
	d, _ := fakeFirDaemon(t, response)
	resp, err := d.Check(nil, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got := resp.Findings[0].Message; got != "a\nb\r\tc\x01\"q\\" {
		t.Fatalf("message = %q", got)
	}
	if got := resp.Crashed["/src/B.kt"]; got != "boom\nat B.kt:1" {
		t.Fatalf("crash = %q", got)
	}
}

func TestFirFingerprintChangesWithRuleOptions(t *testing.T) {
	dir := t.TempDir()
	cacheDir, _ := CacheDir(dir)
	source := dir + "/A.kt"
	jar := dir + "/krit-fir.jar"
	if err := os.WriteFile(source, []byte("class A"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	rules := []string{"A", "B"}
	optsA := func(limit int) RuleConfigs {
		return RuleConfigs{"A": {"limit": limit, "names": []any{"x"}}, "B": {"flag": true}}
	}
	first := FirInvocationFingerprint(nil, jar, rules, optsA(1), FileFacts{})
	for i := 0; i < 20; i++ { // map iteration order must not matter
		if again := FirInvocationFingerprint(nil, jar, rules, optsA(1), FileFacts{}); again != first {
			t.Fatal("fingerprint is not deterministic for identical options")
		}
	}
	WriteFreshEntriesForFingerprint(cacheDir, []string{source}, &CheckResponse{}, first)
	if hits, _ := ClassifyFilesForFingerprint(cacheDir, []string{source}, first); len(hits) != 1 {
		t.Fatal("expected initial hit")
	}
	second := FirInvocationFingerprint(nil, jar, rules, optsA(2), FileFacts{})
	if first == second {
		t.Fatal("option change did not change the fingerprint")
	}
	if hits, misses := ClassifyFilesForFingerprint(cacheDir, []string{source}, second); len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("hits=%d misses=%d", len(hits), len(misses))
	}
	if FirInvocationFingerprint(nil, jar, rules, nil, FileFacts{}) != FirInvocationFingerprint(nil, jar, rules, RuleConfigs{"A": {}}, FileFacts{}) {
		t.Fatal("an empty options map must fingerprint like no options")
	}
}
