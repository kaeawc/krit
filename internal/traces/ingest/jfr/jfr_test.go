package jfr

import (
	"testing"

	"github.com/kaeawc/krit/internal/traces"
)

func TestParseFormatsStackAndRole(t *testing.T) {
	const in = `{
        "recording": {
            "events": [
                {
                    "type": "jdk.ExecutionSample",
                    "startTime": 100,
                    "thread": {"javaName": "main"},
                    "stackTrace": {
                        "frames": [
                            {"method": {"type": {"name": "com.acme.Foo"}, "name": "bar"}},
                            {"method": {"type": {"name": "com.acme.Main"}, "name": "main"}}
                        ]
                    }
                }
            ]
        }
    }`
	events, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if got := events[0].FrameStack; len(got) != 2 || got[0] != "com.acme.Foo.bar" || got[1] != "com.acme.Main.main" {
		t.Fatalf("stack: %v", got)
	}
	if events[0].Role != traces.RoleStartup {
		t.Fatalf("role: want startup, got %s", events[0].Role)
	}
}

func TestParseSkipsUnknownEventType(t *testing.T) {
	const in = `{"recording":{"events":[{"type":"jdk.OtherEvent","stackTrace":{"frames":[{"method":{"name":"x"}}]}}]}}`
	events, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("want 0 events, got %d", len(events))
	}
}
