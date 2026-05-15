package breakage

import (
	"strings"
	"testing"
)

func TestRecordDedupes(t *testing.T) {
	root := t.TempDir()
	ev := Event{
		CommitSHA:   "abc123",
		FailureKind: KindTestFailure,
		Signature:   "failed: x",
		Source:      SourceCI,
		OccurredAt:  100,
	}

	added, err := Record(root, ev)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if added != 1 {
		t.Fatalf("first Record: added=%d, want 1", added)
	}
	added, err = Record(root, ev)
	if err != nil {
		t.Fatalf("Record (dup): %v", err)
	}
	if added != 0 {
		t.Fatalf("dup Record: added=%d, want 0", added)
	}

	events, err := LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("LoadAll: got %d events, want 1", len(events))
	}
	if events[0].ID == "" {
		t.Fatalf("LoadAll: ID was not populated")
	}
}

func TestRecordRequiresCommitAndKind(t *testing.T) {
	root := t.TempDir()
	if _, err := Record(root, Event{FailureKind: KindTestFailure}); err == nil {
		t.Errorf("Record without commit_sha should error")
	}
	if _, err := Record(root, Event{CommitSHA: "abc"}); err == nil {
		t.Errorf("Record without failure_kind should error")
	}
}

func TestIngestJUnit(t *testing.T) {
	xml := `<testsuites>
		<testsuite name="s">
			<testcase classname="com.acme.OrderTest" name="placesOrder" file="OrderTest.kt"/>
			<testcase classname="com.acme.OrderTest" name="rejectsBadInput" file="OrderTest.kt">
				<failure message="expected 5 but got 7" type="AssertionError">stack body</failure>
			</testcase>
			<testcase classname="com.acme.PaymentTest" name="charges">
				<error message="NPE">at com.acme.Payment.charge(Payment.kt:42)</error>
			</testcase>
		</testsuite>
	</testsuites>`
	events, err := IngestJUnit(strings.NewReader(xml), IngestOptions{CommitSHA: "deadbeef"})
	if err != nil {
		t.Fatalf("IngestJUnit: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (only failures + errors)", len(events))
	}
	for _, e := range events {
		if e.CommitSHA != "deadbeef" {
			t.Errorf("event commit_sha=%q", e.CommitSHA)
		}
		if e.FailureKind != KindTestFailure {
			t.Errorf("event kind=%q", e.FailureKind)
		}
		if e.Source != SourceCI {
			t.Errorf("event source=%q, want %q", e.Source, SourceCI)
		}
		if e.ID == "" {
			t.Errorf("event ID empty")
		}
		if e.Symbol == "" {
			t.Errorf("event symbol empty")
		}
	}
}

func TestIngestGoTest(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"run","Package":"x","Test":"TestA"}`,
		`{"Action":"output","Package":"x","Test":"TestA","Output":"--- FAIL: TestA (0.01s)\n"}`,
		`{"Action":"output","Package":"x","Test":"TestA","Output":"    x.go:42: expected 1 got 2\n"}`,
		`{"Action":"fail","Package":"x","Test":"TestA"}`,
		`{"Action":"run","Package":"x","Test":"TestB"}`,
		`{"Action":"pass","Package":"x","Test":"TestB"}`,
		``,
	}, "\n")
	events, err := IngestGoTest(strings.NewReader(stream), IngestOptions{CommitSHA: "abc"})
	if err != nil {
		t.Fatalf("IngestGoTest: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Symbol != "x.TestA" {
		t.Errorf("symbol=%q, want x.TestA", events[0].Symbol)
	}
	if !strings.Contains(events[0].Message, "expected 1 got 2") {
		t.Errorf("message lost: %q", events[0].Message)
	}
}

func TestIngestGenericArrayAndSingle(t *testing.T) {
	single := `{"failure_kind":"runtime-failure","message":"OOM","module":":app"}`
	events, err := IngestGeneric(strings.NewReader(single), IngestOptions{CommitSHA: "abc"})
	if err != nil {
		t.Fatalf("single: %v", err)
	}
	if len(events) != 1 || events[0].Module != ":app" {
		t.Fatalf("single: %+v", events)
	}

	arr := `[{"failure_kind":"runtime-failure","message":"OOM"},{"failure_kind":"build-failure","message":"link"}]`
	events, err = IngestGeneric(strings.NewReader(arr), IngestOptions{CommitSHA: "abc"})
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("array: got %d events, want 2", len(events))
	}
}
