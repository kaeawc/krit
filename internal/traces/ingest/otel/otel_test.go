package otel

import (
	"testing"

	"github.com/kaeawc/krit/internal/traces"
)

const sampleOTLP = `{
  "resourceSpans": [
    {
      "resource": {
        "attributes": [
          {"key": "service.name", "value": {"stringValue": "orderservice"}},
          {"key": "deployment.environment", "value": {"stringValue": "production"}}
        ]
      },
      "scopeSpans": [
        {
          "spans": [
            {
              "traceId": "t1",
              "spanId": "s1",
              "name": "POST /orders",
              "startTimeUnixNano": "1700000000000000000",
              "attributes": [
                {"key": "code.namespace", "value": {"stringValue": "com.acme.api"}},
                {"key": "code.function", "value": {"stringValue": "OrdersHandler.create"}}
              ]
            },
            {
              "traceId": "t1",
              "spanId": "s2",
              "parentSpanId": "s1",
              "name": "Order.create",
              "startTimeUnixNano": "1700000000000001000",
              "attributes": [
                {"key": "code.namespace", "value": {"stringValue": "com.acme.domain"}},
                {"key": "code.function", "value": {"stringValue": "Order.create"}}
              ]
            }
          ]
        }
      ]
    }
  ]
}`

func TestParseExtractsCallStackAndRole(t *testing.T) {
	events, err := Parse([]byte(sampleOTLP))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	// Top span (parent) has 1-frame stack.
	if got := events[0].FrameStack; len(got) != 1 || got[0] != "com.acme.api.OrdersHandler.create" {
		t.Fatalf("parent stack: %v", got)
	}
	// Child span has 2-frame stack with parent on top of its own.
	if got := events[1].FrameStack; len(got) != 2 || got[0] != "com.acme.domain.Order.create" || got[1] != "com.acme.api.OrdersHandler.create" {
		t.Fatalf("child stack: %v", got)
	}
	if events[0].Role != traces.RoleRequest {
		t.Fatalf("role: want %s, got %s", traces.RoleRequest, events[0].Role)
	}
}
