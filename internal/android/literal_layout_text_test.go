package android

import "testing"

func TestIsLiteralLayoutText(t *testing.T) {
	cases := map[string]bool{
		"Hello":              true,
		"":                   false,
		"@string/ok":         false,
		"@android:string/ok": false,
		"@null":              false,
		"?attr/title":        false,
		"@{vm.title}":        false,
		"@strings/typo":      false,
	}
	for in, want := range cases {
		if got := IsLiteralLayoutText(in); got != want {
			t.Errorf("IsLiteralLayoutText(%q) = %v, want %v", in, got, want)
		}
	}
}
