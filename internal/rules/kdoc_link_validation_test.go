package rules

import "testing"

func TestIterateKdocLinks(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "simple and qualified links",
			text: "/** See [User] and [com.example.UserRepository]. */",
			want: []string{"User", "com.example.UserRepository"},
		},
		{
			name: "markdown link skipped",
			text: "/** See [external](https://example.com) and [User]. */",
			want: []string{"User"},
		},
		{
			name: "escaped bracket skipped",
			text: `/** Literal \[User] and real [Account]. */`,
			want: []string{"Account"},
		},
		{
			name: "inline code skipped",
			text: "/** Use `[User]` in examples and [Account] in docs. */",
			want: []string{"Account"},
		},
		{
			name: "nested brackets keep inner target",
			text: "/** Nested [[User]] reference. */",
			want: []string{"User"},
		},
		{
			name: "raw string-like contents skipped by target shape",
			text: "/** Example [\"not a link\"] but [User] is. */",
			want: []string{"User"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			iterateKdocLinks(tt.text, func(link kdocLinkToken) {
				got = append(got, link.Target)
			})
			if len(got) != len(tt.want) {
				t.Fatalf("targets=%v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("targets=%v, want %v", got, tt.want)
				}
			}
		})
	}
}
