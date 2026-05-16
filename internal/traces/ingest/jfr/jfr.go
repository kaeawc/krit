// Package jfr parses JFR-derived stack samples into the canonical
// traces.Event stream.
//
// Krit does not parse the binary JFR format directly. The accepted
// input is the JSON dump produced by `jfr print --json
// --events=jdk.ExecutionSample <file.jfr>` (or any tool that emits
// the same shape). This keeps the ingest path narrow — production
// JFR files are converted by an external tool, krit reads the JSON.
//
// The reduced event stream uses the full Java method stack (with
// `Class.method` formatting) as the frame stack, top-of-stack first.
// Sample frequency is implicit: each sample is one event, and
// adjacent equal samples collapse into a single state with count = N.
package jfr

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/traces"
)

// Document is the relevant subset of `jfr print --json` output.
type Document struct {
	Recording Recording `json:"recording"`
}

type Recording struct {
	Events []SampleEvent `json:"events"`
}

type SampleEvent struct {
	Type      string `json:"type"`
	StartTime int64  `json:"startTime"`
	Thread    Thread `json:"thread"`
	Stack     Stack  `json:"stackTrace"`
}

type Thread struct {
	JavaName string `json:"javaName"`
	OSName   string `json:"osName"`
}

type Stack struct {
	Frames []Frame `json:"frames"`
}

type Frame struct {
	Method Method `json:"method"`
}

type Method struct {
	Type Type   `json:"type"`
	Name string `json:"name"`
}

type Type struct {
	Name string `json:"name"`
}

// Parse decodes JFR JSON bytes into events. SourceID is left for the
// caller to stamp.
func Parse(data []byte) ([]traces.Event, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("jfr: parse: %w", err)
	}
	out := make([]traces.Event, 0, len(doc.Recording.Events))
	for _, ev := range doc.Recording.Events {
		if ev.Type != "" && ev.Type != "jdk.ExecutionSample" {
			continue
		}
		if len(ev.Stack.Frames) == 0 {
			continue
		}
		stack := make([]string, 0, len(ev.Stack.Frames))
		for _, fr := range ev.Stack.Frames {
			if fr.Method.Name == "" {
				continue
			}
			cls := fr.Method.Type.Name
			if cls == "" {
				stack = append(stack, fr.Method.Name)
			} else {
				stack = append(stack, cls+"."+fr.Method.Name)
			}
		}
		if len(stack) == 0 {
			continue
		}
		out = append(out, traces.Event{
			TimestampNS: ev.StartTime,
			FrameStack:  stack,
			Kind:        traces.KindCall,
			Role:        roleFromThread(ev.Thread),
		})
	}
	return out, nil
}

func roleFromThread(t Thread) traces.RoleTag {
	name := t.JavaName
	if name == "" {
		name = t.OSName
	}
	switch {
	case name == "":
		return traces.RoleUnknown
	case containsAny(name, "Test", "JUnit", "Spek"):
		return traces.RoleTest
	case containsAny(name, "main", "Main"):
		return traces.RoleStartup
	case containsAny(name, "Worker", "Background", "Scheduler"):
		return traces.RoleBackground
	}
	return traces.RoleRequest
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
