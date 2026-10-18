package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const emptyOracleResult = `{"version":1,"kotlinVersion":"2.1.0","files":{},"dependencies":{}}`

// checkDaemon is a scripted krit-fir daemon: it records every request and
// answers each method from respond.
type checkDaemon struct {
	mu      sync.Mutex
	methods []string
	params  []map[string]json.RawMessage
}

func (c *checkDaemon) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.methods...)
}

func newCheckDaemon(t *testing.T, respond func(method string, id int) string) (*Daemon, *checkDaemon) {
	t.Helper()
	rec := &checkDaemon{}
	d, listener := newTCPMockDaemon(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
		for sc.Scan() {
			var req struct {
				ID     int                        `json:"id"`
				Method string                     `json:"method"`
				Params map[string]json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal([]byte(sc.Text()), &req); err != nil {
				return
			}
			rec.mu.Lock()
			rec.methods = append(rec.methods, req.Method)
			rec.params = append(rec.params, req.Params)
			rec.mu.Unlock()
			_, _ = conn.Write([]byte(respond(req.Method, req.ID) + "\n"))
		}
	})
	t.Cleanup(func() { listener.Close(); d.conn.Close() })
	return d, rec
}

// rider is a CheckRider that records what it delivered.
func testRider(request string, delivered *[]string) *CheckRider {
	return &CheckRider{Prepare: func() ([]byte, func([]byte)) {
		return []byte(request), func(response []byte) { *delivered = append(*delivered, string(response)) }
	}}
}

const checkRequestBody = `{"id":1,"command":"check","files":[{"path":"/p/A.kt"}],"rules":["R"]}`
const checkResponseBody = `{"id":1,"succeeded":1,"skipped":0,"findings":[],"rules":["R"],"crashed":{},"errorFiles":{},"ruleErrors":{}}`

// One request carries both: the oracle's analyzeAll and the pass's check, and
// the check's response comes back in the same reply.
func TestAnalyzeAllWithCheckIsOneRequest(t *testing.T) {
	d, rec := newCheckDaemon(t, func(method string, id int) string {
		if method != analyzeWithCheckMethod {
			return fmt.Sprintf(`{"id":%d,"error":"unexpected %s"}`, id, method)
		}
		return fmt.Sprintf(`{"id":%d,"result":%s,"check":%s}`, id, emptyOracleResult, checkResponseBody)
	})
	var delivered []string
	data, err := d.AnalyzeAllWithCheck(nil, testRider(checkRequestBody, &delivered))
	if err != nil || data == nil {
		t.Fatalf("data=%v err=%v", data, err)
	}
	if got := rec.seen(); len(got) != 1 || got[0] != analyzeWithCheckMethod {
		t.Fatalf("requests = %v, want one %s", got, analyzeWithCheckMethod)
	}
	if string(rec.params[0][sharedCheckKey]) == "" || strings.Contains(string(rec.params[0][sharedCheckKey]), `"command"`) {
		t.Fatalf("%s = %s", sharedCheckKey, rec.params[0][sharedCheckKey])
	}
	if len(delivered) != 1 || delivered[0] != checkResponseBody {
		t.Fatalf("delivered = %v", delivered)
	}
}

func TestAnalyzeAllWithCheckNotSharedDeliversNothing(t *testing.T) {
	d, rec := newCheckDaemon(t, func(_ string, id int) string {
		return fmt.Sprintf(`{"id":%d,"result":%s,"check":null}`, id, emptyOracleResult)
	})
	var delivered []string
	if _, err := d.AnalyzeAllWithCheck(nil, testRider(checkRequestBody, &delivered)); err != nil {
		t.Fatal(err)
	}
	if len(delivered) != 0 || len(rec.seen()) != 1 {
		t.Fatalf("delivered=%v requests=%v", delivered, rec.seen())
	}
}

// A jar that predates the request answers "Unknown command": the oracle still
// gets its facts from a plain analyzeAll, the pass is released before that
// compile starts (it must not queue behind it) and left to compile on its own,
// and the daemon is not asked again.
func TestAnalyzeAllWithCheckFallsBackOnAnOlderJar(t *testing.T) {
	var abandoned atomic.Int32
	releasedBeforePlainCompile := true
	d, rec := newCheckDaemon(t, func(method string, id int) string {
		if method == analyzeWithCheckMethod {
			return fmt.Sprintf(`{"id":%d,"error":"Unknown command: %s"}`, id, method)
		}
		if abandoned.Load() == 0 {
			releasedBeforePlainCompile = false
		}
		return fmt.Sprintf(`{"id":%d,"result":%s}`, id, emptyOracleResult)
	})
	var delivered []string
	rider := testRider(checkRequestBody, &delivered)
	rider.Decline = func(string) { abandoned.Add(1) }
	for i := 0; i < 2; i++ {
		data, err := d.AnalyzeAllWithCheck(nil, rider)
		if err != nil || data == nil {
			t.Fatalf("call %d: data=%v err=%v", i, data, err)
		}
	}
	want := []string{analyzeWithCheckMethod, "analyzeAll", "analyzeAll"}
	if got := rec.seen(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	if len(delivered) != 0 || !d.noSharedCheck.Load() {
		t.Fatalf("delivered=%v noSharedCheck=%v", delivered, d.noSharedCheck.Load())
	}
	if abandoned.Load() != 2 || !releasedBeforePlainCompile {
		t.Fatalf("abandoned=%d releasedBeforePlainCompile=%v", abandoned.Load(), releasedBeforePlainCompile)
	}
}

func TestAnalyzeAllWithCheckWithoutARiderIsPlainAnalyzeAll(t *testing.T) {
	d, rec := newCheckDaemon(t, func(_ string, id int) string {
		return fmt.Sprintf(`{"id":%d,"result":%s}`, id, emptyOracleResult)
	})
	if _, err := d.AnalyzeAllWithCheck(nil, nil); err != nil {
		t.Fatal(err)
	}
	empty := &CheckRider{Prepare: func() ([]byte, func([]byte)) { return nil, nil }}
	if _, err := d.AnalyzeAllWithCheck(nil, empty); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rec.seen(), ","); got != "analyzeAll,analyzeAll" {
		t.Fatalf("requests = %s", got)
	}
}

// A real failure is the caller's to handle; it is not retried as analyzeAll.
func TestAnalyzeAllWithCheckSurfacesRealErrors(t *testing.T) {
	d, rec := newCheckDaemon(t, func(_ string, id int) string {
		return fmt.Sprintf(`{"id":%d,"error":"Analysis API exception"}`, id)
	})
	var delivered []string
	if _, err := d.AnalyzeAllWithCheck(nil, testRider(checkRequestBody, &delivered)); err == nil {
		t.Fatal("want the daemon's error")
	}
	if len(rec.seen()) != 1 || len(delivered) != 0 || d.noSharedCheck.Load() {
		t.Fatalf("requests=%v delivered=%v noSharedCheck=%v", rec.seen(), delivered, d.noSharedCheck.Load())
	}
}

// oldJarParser mimics the pre-#810 krit-fir request parser exactly: fields are
// read without regard to nesting (first occurrence), and `command` is
// preferred over `method`. A `check` command runs a check and answers
// check-shaped, with no `result`; `analyzeAll` answers the oracle's facts;
// anything else is "Unknown command".
func oldJarParser(line string) string {
	first := func(key string) string {
		m := regexp.MustCompile(`"` + key + `"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`).FindStringSubmatch(line)
		if m == nil {
			return ""
		}
		return m[1]
	}
	id := regexp.MustCompile(`"id"\s*:\s*(\d+)`).FindStringSubmatch(line)[1]
	command := first("command")
	if command == "" {
		command = first("method")
	}
	switch command {
	case "check":
		return fmt.Sprintf(`{"id":%s,"succeeded":1,"skipped":0,"findings":[],"rules":["R"],"crashed":{},"errorFiles":{},"ruleErrors":{}}`, id)
	case "analyzeAll":
		return fmt.Sprintf(`{"id":%s,"result":%s}`, id, emptyOracleResult)
	}
	return fmt.Sprintf(`{"id":%s,"error":"Unknown command: %s"}`, id, command)
}

func newOldJarDaemon(t *testing.T) (*Daemon, *checkDaemon) {
	t.Helper()
	rec := &checkDaemon{}
	d, listener := newTCPMockDaemon(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			var req struct {
				Method string `json:"method"`
			}
			_ = json.Unmarshal([]byte(line), &req)
			rec.mu.Lock()
			rec.methods = append(rec.methods, req.Method)
			rec.mu.Unlock()
			_, _ = conn.Write([]byte(oldJarParser(line) + "\n"))
		}
	})
	t.Cleanup(func() { listener.Close(); d.conn.Close() })
	return d, rec
}

// Against a parser that reads the embedded check's fields as the request's
// own, the combined request must still be refused as an unknown method: the
// oracle keeps its facts, the daemon is remembered as unable, and the waiter
// is released. (The embedded check request itself says "command":"check".)
func TestAnalyzeAllWithCheckAgainstTheOldNestBlindParser(t *testing.T) {
	d, rec := newOldJarDaemon(t)
	var declined []string
	var delivered []string
	rider := testRider(checkRequestBody, &delivered)
	rider.Decline = func(reason string) { declined = append(declined, reason) }
	data, err := d.AnalyzeAllWithCheck(nil, rider)
	if err != nil || data == nil {
		t.Fatalf("the oracle lost its facts: data=%v err=%v", data, err)
	}
	if got := strings.Join(rec.seen(), ","); got != analyzeWithCheckMethod+",analyzeAll" {
		t.Fatalf("requests = %s", got)
	}
	if !d.noSharedCheck.Load() || len(declined) != 1 || len(delivered) != 0 {
		t.Fatalf("noSharedCheck=%v declined=%v delivered=%v", d.noSharedCheck.Load(), declined, delivered)
	}
}

// Defense in depth: even a jar that answers the carrier without an oracle
// result is treated as unable to carry a check, never as a failed analysis.
func TestAnalyzeAllWithCheckTreatsAResultlessReplyAsUnsupported(t *testing.T) {
	for name, reply := range map[string]string{
		"check shaped":   `{"id":%d,"succeeded":1,"findings":[],"rules":["R"],"crashed":{},"errorFiles":{}}`,
		"check attached": `{"id":%d,"check":` + checkResponseBody + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			d, rec := newCheckDaemon(t, func(method string, id int) string {
				if method == analyzeWithCheckMethod {
					return fmt.Sprintf(reply, id)
				}
				return fmt.Sprintf(`{"id":%d,"result":%s}`, id, emptyOracleResult)
			})
			var delivered []string
			declined := 0
			rider := testRider(checkRequestBody, &delivered)
			rider.Decline = func(string) { declined++ }
			data, err := d.AnalyzeAllWithCheck(nil, rider)
			if err != nil || data == nil {
				t.Fatalf("data=%v err=%v", data, err)
			}
			if !d.noSharedCheck.Load() || declined != 1 || len(delivered) != 0 || strings.Join(rec.seen(), ",") != analyzeWithCheckMethod+",analyzeAll" {
				t.Fatalf("noSharedCheck=%v declined=%d delivered=%v requests=%v", d.noSharedCheck.Load(), declined, delivered, rec.seen())
			}
		})
	}
}

// Another method's "Unknown command" is a real failure, not a missing carrier.
func TestIsUnknownCommandMatchesOnlyTheCarrier(t *testing.T) {
	if !isUnknownCommand(fmt.Errorf("daemon error: Unknown command: analyzeWithCheck")) {
		t.Fatal("carrier refusal not recognized")
	}
	if isUnknownCommand(fmt.Errorf("daemon error: Unknown command: check")) || isUnknownCommand(fmt.Errorf("Unknown command")) {
		t.Fatal("unrelated refusal taken for a missing carrier")
	}
}

// The embedded request is sent without its own "command".
func TestAnalyzeAllWithCheckSendsTheCheckWithoutACommand(t *testing.T) {
	d, rec := newCheckDaemon(t, func(_ string, id int) string {
		return fmt.Sprintf(`{"id":%d,"result":%s,"check":null}`, id, emptyOracleResult)
	})
	var delivered []string
	if _, err := d.AnalyzeAllWithCheck(nil, testRider(checkRequestBody, &delivered)); err != nil {
		t.Fatal(err)
	}
	sent := string(rec.params[0][sharedCheckKey])
	if strings.Contains(sent, `"command"`) || !strings.Contains(sent, `"rules":["R"]`) {
		t.Fatalf("embedded check = %s", sent)
	}
}
