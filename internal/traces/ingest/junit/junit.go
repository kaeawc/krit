// Package junit parses JUnit step-boundary records into the
// canonical traces.Event stream.
//
// Standard JUnit XML reports record test pass/fail outcomes but not
// the in-test step boundaries krit cares about. We accept a separate
// JSON file emitted by a test-side listener — the format is a flat
// array of step records:
//
//	[
//	  {
//	    "timestamp_ns": 173...,
//	    "test_class": "com.acme.OrderTest",
//	    "test_method": "createOrder",
//	    "step": "given_empty_cart",
//	    "stack": ["com.acme.Order.create", "com.acme.OrderTest.createOrder"]
//	  }
//	]
//
// Steps without a `stack` are still useful: the (test_class,
// test_method) pair becomes a two-frame stack so test-vs-prod
// divergence queries get a coarse but consistent test signal.
package junit

import (
	"encoding/json"
	"fmt"

	"github.com/kaeawc/krit/internal/traces"
)

// Step is one row of the JUnit step-boundary file.
type Step struct {
	TimestampNS int64    `json:"timestamp_ns"`
	TestClass   string   `json:"test_class"`
	TestMethod  string   `json:"test_method"`
	StepName    string   `json:"step,omitempty"`
	Stack       []string `json:"stack,omitempty"`
}

// Parse decodes a JUnit step-boundary JSON array into events.
func Parse(data []byte) ([]traces.Event, error) {
	var steps []Step
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, fmt.Errorf("junit: parse: %w", err)
	}
	out := make([]traces.Event, 0, len(steps))
	for _, st := range steps {
		stack := st.Stack
		if len(stack) == 0 {
			if st.TestClass != "" && st.TestMethod != "" {
				stack = []string{st.TestClass + "." + st.TestMethod}
			}
		}
		if len(stack) == 0 {
			continue
		}
		out = append(out, traces.Event{
			TimestampNS: st.TimestampNS,
			FrameStack:  stack,
			Kind:        traces.KindCall,
			Role:        traces.RoleTest,
		})
	}
	return out, nil
}
