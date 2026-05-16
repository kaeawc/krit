package junit

import (
	"testing"

	"github.com/kaeawc/krit/internal/traces"
)

func TestParseAssignsTestRole(t *testing.T) {
	const in = `[
        {"timestamp_ns": 1, "test_class": "com.acme.OrderTest", "test_method": "createOrder", "step": "given"},
        {"timestamp_ns": 2, "test_class": "com.acme.OrderTest", "test_method": "createOrder", "stack": ["com.acme.Order.create", "com.acme.OrderTest.createOrder"]}
    ]`
	events, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	for _, ev := range events {
		if ev.Role != traces.RoleTest {
			t.Fatalf("role: want test, got %s", ev.Role)
		}
	}
	if got := events[0].FrameStack; len(got) != 1 || got[0] != "com.acme.OrderTest.createOrder" {
		t.Fatalf("synthesized stack: %v", got)
	}
	if got := events[1].FrameStack; len(got) != 2 {
		t.Fatalf("explicit stack: %v", got)
	}
}
