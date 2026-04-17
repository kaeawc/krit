package pipeline

import (
	"testing"
)

func TestDefaultActiveRules_NonEmpty(t *testing.T) {
	active := DefaultActiveRules()
	if len(active) == 0 {
		t.Fatal("DefaultActiveRules returned 0 rules; expected the usual registered set")
	}
}

func TestDefaultActiveRules_NoNilEntries(t *testing.T) {
	for i, r := range DefaultActiveRules() {
		if r == nil {
			t.Errorf("DefaultActiveRules()[%d] is nil", i)
		}
	}
}

func TestBuildDispatcher_AcceptsNilResolver(t *testing.T) {
	d := BuildDispatcher(DefaultActiveRules(), nil)
	if d == nil {
		t.Fatal("BuildDispatcher returned nil")
	}
}

func TestBuildDispatcher_AcceptsEmptyRules(t *testing.T) {
	d := BuildDispatcher(nil, nil)
	if d == nil {
		t.Fatal("BuildDispatcher(nil,nil) returned nil")
	}
}
