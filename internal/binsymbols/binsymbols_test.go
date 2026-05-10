package binsymbols

import (
	"reflect"
	"testing"
)

func TestEmpty_AlwaysReturnsNil(t *testing.T) {
	if got := Empty.LookupClass("anything"); got != nil {
		t.Errorf("Empty.LookupClass returned %+v, want nil", got)
	}
}

func TestStatic_HitAndMiss(t *testing.T) {
	s := Static{Classes: map[string]*Class{
		"androidx.fragment.app.Fragment": {
			Name:       "Fragment",
			FQN:        "androidx.fragment.app.Fragment",
			Kind:       "class",
			Supertypes: []string{"java.lang.Object"},
		},
	}}

	if c := s.LookupClass("androidx.fragment.app.Fragment"); c == nil {
		t.Fatal("expected hit on known FQN")
	} else if c.Name != "Fragment" {
		t.Errorf("Name = %q, want Fragment", c.Name)
	}
	if c := s.LookupClass("not.in.the.map"); c != nil {
		t.Errorf("expected miss to be nil, got %+v", c)
	}
}

func TestStatic_NilMapSafe(t *testing.T) {
	var s Static
	if got := s.LookupClass("foo.Bar"); got != nil {
		t.Errorf("nil-map Static.LookupClass = %+v, want nil", got)
	}
	if got := s.FQNs(); len(got) != 0 {
		t.Errorf("nil-map FQNs = %v, want empty", got)
	}
}

func TestMulti_ReturnsFirstHit(t *testing.T) {
	a := Static{Classes: map[string]*Class{"a.A": {FQN: "a.A", Name: "A_first"}}}
	b := Static{Classes: map[string]*Class{"a.A": {FQN: "a.A", Name: "A_second"}, "b.B": {FQN: "b.B", Name: "B"}}}
	m := Multi{Readers: []Reader{a, b}}

	if c := m.LookupClass("a.A"); c == nil || c.Name != "A_first" {
		t.Errorf("Multi.LookupClass(a.A) returned %+v, want first reader's hit", c)
	}
	if c := m.LookupClass("b.B"); c == nil || c.Name != "B" {
		t.Errorf("Multi.LookupClass(b.B) returned %+v, want second reader's hit", c)
	}
	if c := m.LookupClass("c.C"); c != nil {
		t.Errorf("Multi.LookupClass(c.C) = %+v, want nil", c)
	}
}

func TestMulti_SkipsNilReaders(t *testing.T) {
	a := Static{Classes: map[string]*Class{"a.A": {FQN: "a.A"}}}
	m := Multi{Readers: []Reader{nil, a, nil}}
	if got := m.LookupClass("a.A"); got == nil {
		t.Error("nil entries should be skipped, not panic")
	}
}

func TestStatic_FQNsSortedAndComplete(t *testing.T) {
	s := Static{Classes: map[string]*Class{
		"z.Z": {FQN: "z.Z"},
		"a.A": {FQN: "a.A"},
		"m.M": {FQN: "m.M"},
	}}
	want := []string{"a.A", "m.M", "z.Z"}
	if got := s.FQNs(); !reflect.DeepEqual(got, want) {
		t.Errorf("FQNs() = %v, want %v", got, want)
	}
}
