package snapshot

import (
	"sort"
	"time"

	"github.com/kaeawc/krit/internal/breakage"
)

// SynthesizeFindingRegressions diffs the new findings against the
// most recent earlier capture and records a synthetic
// krit-finding-regression event for every (rule, file) newly reporting
// a finding at this commit. Returns (0, nil) when there is no earlier
// capture, when rule-set hashes diverge, or when redaction states
// differ — comparing across any of those would be noise.
func SynthesizeFindingRegressions(root string, current *Findings, now time.Time) (int, error) {
	if current == nil || current.CommitSHA == "" {
		return 0, nil
	}
	prev, err := loadPreviousFindings(root, current.CommitSHA)
	if err != nil {
		return 0, err
	}
	if prev == nil {
		return 0, nil
	}
	if current.RuleSetHash != "" && prev.RuleSetHash != "" && current.RuleSetHash != prev.RuleSetHash {
		return 0, nil
	}
	if current.Redacted != prev.Redacted {
		return 0, nil
	}
	events := buildRegressionEvents(prev, current, now)
	if len(events) == 0 {
		return 0, nil
	}
	return breakage.Record(root, events...)
}

// loadPreviousFindings walks captured manifests newest-first and
// returns the first findings sidecar strictly older than currentSHA.
func loadPreviousFindings(root, currentSHA string) (*Findings, error) {
	manifests, err := LoadManifests(root)
	if err != nil {
		return nil, err
	}
	sort.Slice(manifests, func(i, j int) bool {
		if manifests[i].CapturedAt != manifests[j].CapturedAt {
			return manifests[i].CapturedAt < manifests[j].CapturedAt
		}
		return manifests[i].CommitSHA < manifests[j].CommitSHA
	})
	for i := len(manifests) - 1; i >= 0; i-- {
		m := manifests[i]
		if m.CommitSHA == currentSHA {
			continue
		}
		f, err := LoadFindings(root, m.CommitSHA)
		if err != nil || f == nil {
			continue
		}
		return f, nil
	}
	return nil, nil
}

func buildRegressionEvents(prev, cur *Findings, now time.Time) []breakage.Event {
	if now.IsZero() {
		now = time.Now()
	}
	occurred := now.UnixMilli()
	var events []breakage.Event
	for rule, perFileCur := range cur.ByRuleFile {
		perFilePrev := prev.ByRuleFile[rule]
		for file, count := range perFileCur {
			if count <= perFilePrev[file] {
				continue
			}
			ev := breakage.Event{
				OccurredAt:  occurred,
				CommitSHA:   cur.CommitSHA,
				FailureKind: breakage.KindKritFindingRegression,
				Signature:   breakage.Normalize("rule " + rule + " file " + file),
				File:        file,
				Symbol:      rule,
				Source:      breakage.SourceKritFinding,
				Message:     "rule " + rule + " newly fires on " + file,
			}
			ev.ID = breakage.HashID(ev.FailureKind, ev.Signature, ev.CommitSHA, ev.Source)
			events = append(events, ev)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	return events
}
