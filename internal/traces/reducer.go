package traces

import (
	"slices"
	"sort"
)

// Reduce collapses a stream of canonical events into runtime states
// and the transitions between them.
//
// Adjacent events that fingerprint to the same state are merged into
// the same RuntimeState (count incremented, LastSeen widened). A
// transition is emitted whenever consecutive events fingerprint to
// different states; its kind is taken from the second event's Kind
// field (the kind that caused the change), falling back to KindCall.
//
// Events are assumed pre-sorted within a single source (OTel spans
// are timestamp-ordered, JFR samples are wall-clock-ordered, JUnit
// step boundaries are emit-ordered). When ingesting from multiple
// sources the caller is expected to call Reduce per-source and merge
// the results via Store.Merge so cross-source transitions do not
// invent edges that never actually occurred.
//
// Empty input returns nil slices. Events whose FrameStack is empty
// are skipped — there is no top symbol to fingerprint.
func Reduce(events []Event) ([]RuntimeState, []RuntimeTransition) {
	if len(events) == 0 {
		return nil, nil
	}
	stateByFP := map[string]*RuntimeState{}
	transByKey := map[string]*RuntimeTransition{}
	sourceSeen := map[string]map[string]struct{}{}
	var prevFP string

	for _, ev := range events {
		if len(ev.FrameStack) == 0 {
			continue
		}
		fp := upsertState(ev, stateByFP, sourceSeen)
		if prevFP != "" && prevFP != fp {
			upsertTransition(ev, prevFP, fp, transByKey)
		}
		prevFP = fp
	}
	return collectAndSort(stateByFP, transByKey)
}

// upsertState inserts or updates the RuntimeState for ev and returns
// its fingerprint.
func upsertState(ev Event, stateByFP map[string]*RuntimeState, sourceSeen map[string]map[string]struct{}) string {
	top := ev.FrameStack[0]
	chain := HashCallerChain(ev.FrameStack)
	role := ev.Role
	if role == "" {
		role = RoleUnknown
	}
	fp := Fingerprint(top, chain, role)

	st, ok := stateByFP[fp]
	if !ok {
		st = &RuntimeState{
			Fingerprint:     fp,
			TopSymbol:       top,
			CallerChainHash: chain,
			Role:            role,
			FirstSeen:       ev.TimestampNS,
			LastSeen:        ev.TimestampNS,
			CallerFrames:    callerFrames(ev.FrameStack),
		}
		stateByFP[fp] = st
		sourceSeen[fp] = map[string]struct{}{}
	}
	st.Count++
	widen(&st.FirstSeen, &st.LastSeen, ev.TimestampNS)
	if ev.SourceID != "" {
		if _, dup := sourceSeen[fp][ev.SourceID]; !dup {
			sourceSeen[fp][ev.SourceID] = struct{}{}
			st.Sources = append(st.Sources, ev.SourceID)
		}
	}
	return fp
}

// upsertTransition inserts or updates the transition between prevFP
// and curFP. Kind defaults to KindCall.
func upsertTransition(ev Event, prevFP, curFP string, transByKey map[string]*RuntimeTransition) {
	kind := ev.Kind
	if kind == "" {
		kind = KindCall
	}
	key := prevFP + "|" + curFP + "|" + string(kind)
	tr, ok := transByKey[key]
	if !ok {
		tr = &RuntimeTransition{
			FromFP:    prevFP,
			ToFP:      curFP,
			Kind:      kind,
			FirstSeen: ev.TimestampNS,
			LastSeen:  ev.TimestampNS,
		}
		transByKey[key] = tr
	}
	tr.Count++
	widen(&tr.FirstSeen, &tr.LastSeen, ev.TimestampNS)
	if ev.SourceID != "" && !slices.Contains(tr.Sources, ev.SourceID) {
		tr.Sources = append(tr.Sources, ev.SourceID)
	}
}

// widen narrows first toward ts and widens last toward ts when ts is
// non-zero. Zero (missing) timestamps are ignored — typical for JUnit
// step boundaries that don't carry one.
func widen(first, last *int64, ts int64) {
	if ts <= 0 {
		return
	}
	if *first == 0 || ts < *first {
		*first = ts
	}
	if ts > *last {
		*last = ts
	}
}

func collectAndSort(stateByFP map[string]*RuntimeState, transByKey map[string]*RuntimeTransition) ([]RuntimeState, []RuntimeTransition) {
	states := make([]RuntimeState, 0, len(stateByFP))
	for _, st := range stateByFP {
		states = append(states, *st)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].Fingerprint < states[j].Fingerprint })

	trans := make([]RuntimeTransition, 0, len(transByKey))
	for _, t := range transByKey {
		trans = append(trans, *t)
	}
	sort.Slice(trans, func(i, j int) bool {
		if trans[i].FromFP != trans[j].FromFP {
			return trans[i].FromFP < trans[j].FromFP
		}
		if trans[i].ToFP != trans[j].ToFP {
			return trans[i].ToFP < trans[j].ToFP
		}
		return trans[i].Kind < trans[j].Kind
	})
	return states, trans
}

// callerFrames returns a copy of the bounded caller-frame window
// (frames[1:1+depth]) so the RuntimeState owns its slice
// independently of the caller's event buffer.
func callerFrames(frames []string) []string {
	w := callerWindow(frames)
	if len(w) == 0 {
		return nil
	}
	out := make([]string, len(w))
	copy(out, w)
	return out
}
