package rules

import "testing"

func TestFixtureByReferencePrefix(t *testing.T) {
	fixtures := map[string]testFixtureDeclaration{
		"a.b.Fake":  {qualifiedName: "a.b.Fake"},
		"a.b.Other": {qualifiedName: "a.b.Other"},
	}
	cases := []struct {
		text string
		want string
	}{
		{"a.b.Fake", "a.b.Fake"},
		{"a.b.Fake()", "a.b.Fake"},
		{"a.b.Fake<String>", "a.b.Fake"},
		{"a.b.Fake.create()", "a.b.Fake"},
		{"a.b.Other.INSTANCE.name", "a.b.Other"},
		{"a.b.FakeExtra", ""},
		{"a.b", ""},
		{"x.a.b.Fake", ""},
		{"Foo<a.b.Fake>", ""},
		{"", ""},
	}
	for _, tc := range cases {
		decl, ok := fixtureByReferencePrefix(tc.text, fixtures)
		if ok != (tc.want != "") || decl.qualifiedName != tc.want {
			t.Errorf("%q: got (%q, %v), want %q", tc.text, decl.qualifiedName, ok, tc.want)
		}
	}
}

func TestMentionsIdentifier(t *testing.T) {
	names := map[string]struct{}{"FakeThing": {}}
	cases := []struct {
		src  string
		want bool
	}{
		{"val x = FakeThing()", true},
		{"import a.b.FakeThing", true},
		{"FakeThing", true},
		{"val x = NotFakeThing()", false},
		{"val x = FakeThingy()", false},
		{"val FakeThing_ = 1", false},
		{"", false},
		{"a.FakeThing.b", true},
	}
	for _, tc := range cases {
		if got := mentionsIdentifier([]byte(tc.src), names); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.src, got, tc.want)
		}
	}
	if mentionsIdentifier([]byte("FakeThing"), nil) {
		t.Error("no names should never match")
	}
}
