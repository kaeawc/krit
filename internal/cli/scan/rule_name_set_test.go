package scan

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseRuleNameSet(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty input", "", nil},
		{"single name", "Foo", []string{"Foo"}},
		{"multiple names", "Foo,Bar,Baz", []string{"Bar", "Baz", "Foo"}},
		{"whitespace trimmed", "  Foo , Bar ", []string{"Bar", "Foo"}},
		{"duplicate names dedupe", "Foo,Foo,Bar", []string{"Bar", "Foo"}},
		{"trailing comma yields empty entry", "Foo,", []string{"", "Foo"}},
		{"all-whitespace token yields empty entry", "Foo,   ,Bar", []string{"", "Bar", "Foo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRuleNameSet(tc.in)
			gotKeys := mapKeysSorted(got)
			if !reflect.DeepEqual(gotKeys, tc.want) {
				t.Fatalf("parseRuleNameSet(%q) keys = %v; want %v", tc.in, gotKeys, tc.want)
			}
		})
	}
}

func TestParseRuleNameSetReturnsNonNilMap(t *testing.T) {
	got := parseRuleNameSet("")
	if got == nil {
		t.Fatal("got nil map; want non-nil empty")
	}
	if len(got) != 0 {
		t.Fatalf("got %d entries; want 0", len(got))
	}
}

func mapKeysSorted(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
