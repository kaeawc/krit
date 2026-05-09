package rules

import "testing"

func TestMatchTypeBytes(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		names []string
		want  bool
	}{
		{"empty text", "", []string{"Foo"}, false},
		{"trim only whitespace", "   \t  ", []string{"Foo"}, false},
		{"contains substring", "androidx.recyclerview.widget.ListAdapter<X, Y>", []string{"ListAdapter"}, true},
		{"trims surrounding whitespace", "  ListAdapter  ", []string{"ListAdapter"}, true},
		{"no match", "kotlin.String", []string{"Foo"}, false},
		{"dollar in text matches dotted name", "foo$Bar", []string{"foo.Bar"}, true},
		{"dollar in name matches dotted text", "foo.Bar", []string{"foo$Bar"}, true},
		{"multiple names, second matches", "kotlin.collections.List", []string{"Foo", "List"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchTypeBytes([]byte(tc.text), tc.names)
			if got != tc.want {
				t.Fatalf("matchTypeBytes(%q, %v) = %v; want %v", tc.text, tc.names, got, tc.want)
			}
		})
	}
}
