package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
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
	if string(rec.params[0]["sharedCheck"]) != checkRequestBody {
		t.Fatalf("sharedCheck = %s", rec.params[0]["sharedCheck"])
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
	rider.Abandon = func() { abandoned.Add(1) }
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
