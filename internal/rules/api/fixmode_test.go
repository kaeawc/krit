package api

import (
	"errors"
	"strings"
	"testing"
)

func TestFixMode_String(t *testing.T) {
	tests := []struct {
		mode FixMode
		want string
	}{
		{FixModeNone, "none"},
		{FixModeAutofix, "autofix"},
		{FixModeSuggested, "suggested"},
		{FixMode(99), "none"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("FixMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestRule_FixMode(t *testing.T) {
	tests := []struct {
		name string
		rule *Rule
		want FixMode
	}{
		{"nil rule", nil, FixModeNone},
		{"no fix", &Rule{ID: "x"}, FixModeNone},
		{"autofix cosmetic", &Rule{ID: "x", Fix: FixCosmetic}, FixModeAutofix},
		{"autofix idiomatic", &Rule{ID: "x", Fix: FixIdiomatic}, FixModeAutofix},
		{"autofix semantic", &Rule{ID: "x", Fix: FixSemantic}, FixModeAutofix},
		{
			"suggested",
			&Rule{ID: "x", SuggestedFixes: []SuggestedFix{{ID: "a", Title: "Apply A", Level: FixIdiomatic}}},
			FixModeSuggested,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.FixMode(); got != tt.want {
				t.Errorf("FixMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRule_ValidateFixMode_Accepts(t *testing.T) {
	tests := []struct {
		name string
		rule *Rule
	}{
		{"nil rule", nil},
		{"no fix", &Rule{ID: "x"}},
		{"autofix only", &Rule{ID: "x", Fix: FixIdiomatic}},
		{
			"suggested only",
			&Rule{
				ID: "x",
				SuggestedFixes: []SuggestedFix{
					{ID: "a", Title: "Apply A", Level: FixIdiomatic},
					{ID: "b", Title: "Apply B", Level: FixCosmetic},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.rule.ValidateFixMode(); err != nil {
				t.Errorf("ValidateFixMode() = %v, want nil", err)
			}
		})
	}
}

func TestRule_ValidateFixMode_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		rule     *Rule
		wantKind FixModeErrorKind
	}{
		{
			"mixed declaration",
			&Rule{
				ID:  "MixedRule",
				Fix: FixSemantic,
				SuggestedFixes: []SuggestedFix{
					{ID: "a", Title: "Apply A", Level: FixIdiomatic},
				},
			},
			FixModeErrorMixedDeclaration,
		},
		{
			"empty suggestion ID",
			&Rule{
				ID:             "x",
				SuggestedFixes: []SuggestedFix{{ID: "", Title: "Apply A", Level: FixIdiomatic}},
			},
			FixModeErrorEmptyID,
		},
		{
			"empty suggestion title",
			&Rule{
				ID:             "x",
				SuggestedFixes: []SuggestedFix{{ID: "a", Title: "", Level: FixIdiomatic}},
			},
			FixModeErrorEmptyTitle,
		},
		{
			"suggestion level none",
			&Rule{
				ID:             "x",
				SuggestedFixes: []SuggestedFix{{ID: "a", Title: "Apply A", Level: FixNone}},
			},
			FixModeErrorLevelNone,
		},
		{
			"duplicate suggestion ID",
			&Rule{
				ID: "x",
				SuggestedFixes: []SuggestedFix{
					{ID: "a", Title: "First", Level: FixCosmetic},
					{ID: "a", Title: "Second", Level: FixIdiomatic},
				},
			},
			FixModeErrorDuplicateID,
		},
		{
			"mixed interface",
			&Rule{ID: "MixedImpl", Implementation: fakeMixedImpl{}},
			FixModeErrorMixedInterface,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.ValidateFixMode()
			if err == nil {
				t.Fatalf("ValidateFixMode() = nil, want error of kind %v", tt.wantKind)
			}
			var fme *FixModeError
			if !errors.As(err, &fme) {
				t.Fatalf("ValidateFixMode() error type = %T, want *FixModeError", err)
			}
			if fme.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v (msg: %s)", fme.Kind, tt.wantKind, err)
			}
			if fme.Rule != tt.rule.ID {
				t.Errorf("Rule = %q, want %q", fme.Rule, tt.rule.ID)
			}
		})
	}
}

type fakeAutofixImpl struct{}

func (fakeAutofixImpl) AutofixLevel() FixLevel { return FixIdiomatic }

type fakeSuggestedImpl struct{}

func (fakeSuggestedImpl) SuggestedFixes() []SuggestedFix {
	return []SuggestedFix{{ID: "a", Title: "Apply A", Level: FixIdiomatic}}
}

type fakeMixedImpl struct{}

func (fakeMixedImpl) AutofixLevel() FixLevel { return FixIdiomatic }
func (fakeMixedImpl) SuggestedFixes() []SuggestedFix {
	return []SuggestedFix{{ID: "a", Title: "Apply A", Level: FixIdiomatic}}
}

func TestRule_FixMode_InterfaceSatisfaction(t *testing.T) {
	if _, ok := any(fakeAutofixImpl{}).(AutofixRule); !ok {
		t.Error("fakeAutofixImpl should satisfy AutofixRule")
	}
	if _, ok := any(fakeSuggestedImpl{}).(SuggestedFixRule); !ok {
		t.Error("fakeSuggestedImpl should satisfy SuggestedFixRule")
	}
}

func TestRegister_PanicsOnMixedFixDeclaration(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Register did not panic on mixed Fix + SuggestedFixes")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value type = %T, want string", r)
		}
		if !strings.Contains(msg, "exactly one fix mode") {
			t.Errorf("panic message %q should explain the constraint", msg)
		}
	}()
	Register(&Rule{
		ID:          "MixedRegister",
		Description: "tries to declare both fix modes",
		Check:       func(*Context) {},
		Fix:         FixCosmetic,
		SuggestedFixes: []SuggestedFix{
			{ID: "a", Title: "Apply A", Level: FixIdiomatic},
		},
	})
}

func TestRegister_AcceptsValidModes(t *testing.T) {
	tests := []struct {
		name string
		rule *Rule
		want FixMode
	}{
		{
			"no fix",
			&Rule{
				ID:          "DiagnosticOnly",
				Description: "no-fix test rule",
				Check:       func(*Context) {},
			},
			FixModeNone,
		},
		{
			"autofix only",
			&Rule{
				ID:          "AutofixOnlyRegister",
				Description: "autofix-only test rule",
				Check:       func(*Context) {},
				Fix:         FixIdiomatic,
			},
			FixModeAutofix,
		},
		{
			"suggested only",
			&Rule{
				ID:          "SuggestedOnlyRegister",
				Description: "suggested-only test rule",
				Check:       func(*Context) {},
				SuggestedFixes: []SuggestedFix{
					{ID: "primary", Title: "Primary suggestion", Level: FixIdiomatic},
					{ID: "fallback", Title: "Fallback suggestion", Level: FixCosmetic},
				},
			},
			FixModeSuggested,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(Registry)
			defer func() { Registry = Registry[:before] }()
			Register(tt.rule)
			if len(Registry) != before+1 {
				t.Fatalf("Registry len = %d, want %d", len(Registry), before+1)
			}
			if got := Registry[before].FixMode(); got != tt.want {
				t.Errorf("FixMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegister_PreservesSuggestedFixOrdering(t *testing.T) {
	before := len(Registry)
	defer func() { Registry = Registry[:before] }()
	Register(&Rule{
		ID:          "OrderedSuggestions",
		Description: "checks ordering is rule-owned",
		Check:       func(*Context) {},
		SuggestedFixes: []SuggestedFix{
			{ID: "primary", Title: "Primary", Level: FixIdiomatic},
			{ID: "fallback", Title: "Fallback", Level: FixCosmetic},
		},
	})
	got := Registry[before].SuggestedFixes
	if got[0].ID != "primary" || got[1].ID != "fallback" {
		t.Errorf("ordering not preserved: %+v", got)
	}
}

func TestFakeRule_WithSuggestedFixes(t *testing.T) {
	r := FakeRule("x", WithSuggestedFixes(
		SuggestedFix{ID: "a", Title: "Apply A", Level: FixIdiomatic},
		SuggestedFix{ID: "b", Title: "Apply B", Level: FixCosmetic},
	))
	if r.FixMode() != FixModeSuggested {
		t.Errorf("FixMode() = %v, want FixModeSuggested", r.FixMode())
	}
	if len(r.SuggestedFixes) != 2 || r.SuggestedFixes[1].ID != "b" {
		t.Errorf("SuggestedFixes ordering wrong: %+v", r.SuggestedFixes)
	}
}
