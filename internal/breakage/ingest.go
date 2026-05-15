package breakage

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// IngestOptions tunes a single ingest invocation. CommitSHA is required
// because every event must be tied to a commit on the snapshot
// timeline.
type IngestOptions struct {
	CommitSHA string
	Source    string
	// OccurredAt overrides the event timestamp; zero means "now".
	OccurredAt time.Time
}

func (o IngestOptions) occurred() int64 {
	if o.OccurredAt.IsZero() {
		return time.Now().UnixMilli()
	}
	return o.OccurredAt.UnixMilli()
}

type junitDetail struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

type junitTestCase struct {
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	File      string        `xml:"file,attr"`
	Failures  []junitDetail `xml:"failure"`
	Errors    []junitDetail `xml:"error"`
}

// IngestJUnit parses a JUnit XML stream and emits one Event per failed
// or errored test case. Successful cases are skipped — bisect only
// needs the breakage signal.
func IngestJUnit(r io.Reader, opts IngestOptions) ([]Event, error) {
	if opts.CommitSHA == "" {
		return nil, fmt.Errorf("breakage: IngestJUnit: commit_sha required")
	}
	src := opts.Source
	if src == "" {
		src = SourceCI
	}
	occurred := opts.occurred()

	dec := xml.NewDecoder(r)
	var out []Event
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("breakage: parse junit: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "testcase" {
			continue
		}
		var c junitTestCase
		if err := dec.DecodeElement(&c, &start); err != nil {
			return nil, fmt.Errorf("breakage: decode testcase: %w", err)
		}
		if len(c.Failures) == 0 && len(c.Errors) == 0 {
			continue
		}
		fail := firstNonEmpty(junitMessages(c.Failures, c.Errors)...)
		symbol := strings.TrimSpace(c.Classname)
		if c.Name != "" {
			if symbol != "" {
				symbol += "." + c.Name
			} else {
				symbol = c.Name
			}
		}
		e := Event{
			OccurredAt:  occurred,
			CommitSHA:   opts.CommitSHA,
			FailureKind: KindTestFailure,
			Signature:   Normalize(fail),
			Symbol:      symbol,
			File:        c.File,
			Source:      src,
			Message:     fail,
		}
		e.ID = HashID(e.FailureKind, e.Signature, e.CommitSHA, e.Source)
		out = append(out, e)
	}
	return out, nil
}

func junitMessages(failures, errs []junitDetail) []string {
	out := make([]string, 0, len(failures)+len(errs))
	for _, f := range failures {
		out = append(out, strings.TrimSpace(f.Message+" "+f.Body))
	}
	for _, e := range errs {
		out = append(out, strings.TrimSpace(e.Message+" "+e.Body))
	}
	return out
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// IngestGoTest reads `go test -json` output and emits one event per
// failed test. Skipped/passed actions are ignored. Output records that
// can't be parsed as JSON are skipped silently so partial logs still
// produce signal.
func IngestGoTest(r io.Reader, opts IngestOptions) ([]Event, error) {
	if opts.CommitSHA == "" {
		return nil, fmt.Errorf("breakage: IngestGoTest: commit_sha required")
	}
	src := opts.Source
	if src == "" {
		src = SourceCI
	}
	occurred := opts.occurred()

	type record struct {
		Action  string `json:"Action"`
		Package string `json:"Package"`
		Test    string `json:"Test"`
		Output  string `json:"Output"`
	}

	type key struct{ pkg, test string }
	buffers := make(map[key]*strings.Builder)
	var out []Event
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var rec record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Test == "" {
			continue
		}
		k := key{rec.Package, rec.Test}
		switch rec.Action {
		case "output":
			b := buffers[k]
			if b == nil {
				b = &strings.Builder{}
				buffers[k] = b
			}
			b.WriteString(rec.Output)
		case "fail":
			b := buffers[k]
			var msg string
			if b != nil {
				msg = b.String()
			}
			symbol := rec.Test
			if rec.Package != "" {
				symbol = rec.Package + "." + rec.Test
			}
			e := Event{
				OccurredAt:  occurred,
				CommitSHA:   opts.CommitSHA,
				FailureKind: KindTestFailure,
				Signature:   Normalize(msg),
				Symbol:      symbol,
				Source:      src,
				Message:     strings.TrimSpace(msg),
			}
			e.ID = HashID(e.FailureKind, e.Signature, e.CommitSHA, e.Source)
			out = append(out, e)
			delete(buffers, k)
		case "pass", "skip":
			delete(buffers, k)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("breakage: scan go test json: %w", err)
	}
	return out, nil
}

// GenericEvent is the externally-facing JSON shape consumers can post
// with `krit breakage record --kind generic --from file.json`. It's
// deliberately a loose superset of Event so producers can omit
// optional fields.
type GenericEvent struct {
	FailureKind string   `json:"failure_kind"`
	Signature   string   `json:"signature"`
	Module      string   `json:"module,omitempty"`
	File        string   `json:"file,omitempty"`
	Symbol      string   `json:"symbol,omitempty"`
	Source      string   `json:"source,omitempty"`
	Frames      []string `json:"frames,omitempty"`
	Message     string   `json:"message,omitempty"`
}

// IngestGeneric reads a JSON document (single event or array of events)
// and emits the resulting Events. CommitSHA from opts is applied to all
// emitted events.
func IngestGeneric(r io.Reader, opts IngestOptions) ([]Event, error) {
	if opts.CommitSHA == "" {
		return nil, fmt.Errorf("breakage: IngestGeneric: commit_sha required")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("breakage: read generic: %w", err)
	}
	src := opts.Source
	if src == "" {
		src = SourceLocal
	}
	occurred := opts.occurred()

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	var arr []GenericEvent
	if trimmed[0] == '[' {
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, fmt.Errorf("breakage: parse generic array: %w", err)
		}
	} else {
		var one GenericEvent
		if err := json.Unmarshal(data, &one); err != nil {
			return nil, fmt.Errorf("breakage: parse generic event: %w", err)
		}
		arr = []GenericEvent{one}
	}

	out := make([]Event, 0, len(arr))
	for _, g := range arr {
		kind := g.FailureKind
		if kind == "" {
			kind = KindRuntimeFailure
		}
		sig := g.Signature
		if sig == "" {
			sig = Normalize(g.Message)
		}
		ev := Event{
			OccurredAt:  occurred,
			CommitSHA:   opts.CommitSHA,
			FailureKind: kind,
			Signature:   sig,
			Module:      g.Module,
			File:        g.File,
			Symbol:      g.Symbol,
			Source:      pickSource(g.Source, src),
			Frames:      g.Frames,
			Message:     g.Message,
		}
		ev.ID = HashID(ev.FailureKind, ev.Signature, ev.CommitSHA, ev.Source)
		out = append(out, ev)
	}
	return out, nil
}

func pickSource(explicit, fallback string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return fallback
}
