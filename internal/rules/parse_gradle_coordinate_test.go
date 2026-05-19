package rules

import "testing"

func TestParseGradleCoordinate(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantGroup   string
		wantName    string
		wantVersion string
		wantOK      bool
	}{
		{
			name:        "three-part-canonical",
			input:       "com.example:lib:1.2.3",
			wantGroup:   "com.example",
			wantName:    "lib",
			wantVersion: "1.2.3",
			wantOK:      true,
		},
		{
			name:        "four-part-classifier",
			input:       "com.example:lib:1.2.3:sources",
			wantGroup:   "com.example",
			wantName:    "lib",
			wantVersion: "1.2.3",
			wantOK:      true,
		},
		{
			name:        "four-part-aar-extension-after-split",
			input:       "com.example:lib:1.2.3:aar",
			wantGroup:   "com.example",
			wantName:    "lib",
			wantVersion: "1.2.3",
			wantOK:      true,
		},
		{
			name:        "trims-whitespace",
			input:       " com.example : lib : 1.2.3 ",
			wantGroup:   "com.example",
			wantName:    "lib",
			wantVersion: "1.2.3",
			wantOK:      true,
		},
		{
			name:   "too-few-parts-is-not-ok",
			input:  "com.example:lib",
			wantOK: false,
		},
		{
			name:   "empty-group-fails",
			input:  ":lib:1.2.3",
			wantOK: false,
		},
		{
			name:   "empty-name-fails",
			input:  "com.example::1.2.3",
			wantOK: false,
		},
		{
			name:   "empty-version-fails",
			input:  "com.example:lib:",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, n, v, ok := parseGradleCoordinate(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if g != tc.wantGroup || n != tc.wantName || v != tc.wantVersion {
				t.Errorf("parseGradleCoordinate(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.input, g, n, v, tc.wantGroup, tc.wantName, tc.wantVersion)
			}
		})
	}
}
