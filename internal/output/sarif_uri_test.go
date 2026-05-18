package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestSARIFArtifactLocationURI verifies SARIF artifactLocation.uri is a valid
// URI reference per RFC 3986: reserved characters (space, '#', '?'), unicode
// scalars, and Windows backslashes are encoded/normalised. Strict consumers
// such as GitHub code scanning reject runs that embed raw paths.
func TestSARIFArtifactLocationURI(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantURI string
	}{
		{
			name:    "posix-absolute-with-spaces",
			path:    "/tmp/My Project/Foo.kt",
			wantURI: "file:///tmp/My%20Project/Foo.kt",
		},
		{
			name:    "posix-absolute-with-hash",
			path:    "/tmp/proj/Bug#123.kt",
			wantURI: "file:///tmp/proj/Bug%23123.kt",
		},
		{
			name:    "posix-absolute-with-question",
			path:    "/tmp/proj/Maybe?.kt",
			wantURI: "file:///tmp/proj/Maybe%3F.kt",
		},
		{
			name:    "posix-absolute-with-unicode",
			path:    "/tmp/проект/Файл.kt",
			wantURI: "file:///tmp/%D0%BF%D1%80%D0%BE%D0%B5%D0%BA%D1%82/%D0%A4%D0%B0%D0%B9%D0%BB.kt",
		},
		{
			name:    "windows-drive-with-backslashes",
			path:    `C:\Users\Jane\My Project\Foo.kt`,
			wantURI: "file:///C:/Users/Jane/My%20Project/Foo.kt",
		},
		{
			name:    "relative-with-spaces",
			path:    "app/My Module/Foo.kt",
			wantURI: "app/My%20Module/Foo.kt",
		},
		{
			name:    "relative-with-backslashes",
			path:    `app\sub dir\Foo.kt`,
			wantURI: "app/sub%20dir/Foo.kt",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := []scanner.Finding{
				{
					File:     tc.path,
					Line:     1,
					Col:      1,
					Severity: "warning",
					RuleSet:  "style",
					Rule:     "Demo",
					Message:  "demo",
				},
			}

			var buf bytes.Buffer
			if err := FormatSARIF(&buf, findings, "0.0.0"); err != nil {
				t.Fatalf("FormatSARIF: %v", err)
			}

			var log sarifLog
			if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
				t.Fatalf("invalid SARIF JSON: %v\n%s", err, buf.String())
			}
			if len(log.Runs) != 1 || len(log.Runs[0].Results) != 1 {
				t.Fatalf("expected 1 run with 1 result; got %s", buf.String())
			}
			got := log.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI
			if got != tc.wantURI {
				t.Fatalf("artifactLocation.uri = %q, want %q", got, tc.wantURI)
			}

			// Belt-and-suspenders: no raw spaces, '#', '?', backslashes, or
			// non-ASCII bytes should leak into the URI string.
			if strings.ContainsAny(got, " \\") {
				t.Errorf("URI contains forbidden char (space/backslash): %q", got)
			}
			for i := 0; i < len(got); i++ {
				if got[i] > 0x7E || got[i] < 0x20 {
					t.Errorf("URI contains non-printable/non-ASCII byte 0x%02x: %q", got[i], got)
					break
				}
			}
		})
	}
}

// TestPathToSARIFURI exercises the helper directly so the URI convention is
// pinned independent of the formatter wiring.
func TestPathToSARIFURI(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"/tmp/foo.kt", "file:///tmp/foo.kt"},
		{"/tmp/My Project/Foo.kt", "file:///tmp/My%20Project/Foo.kt"},
		{"/tmp/проект/Файл.kt", "file:///tmp/%D0%BF%D1%80%D0%BE%D0%B5%D0%BA%D1%82/%D0%A4%D0%B0%D0%B9%D0%BB.kt"},
		{"/tmp/has?and#.kt", "file:///tmp/has%3Fand%23.kt"},
		{"app/Foo.kt", "app/Foo.kt"},
		{"app/My Module/Foo.kt", "app/My%20Module/Foo.kt"},
		{`C:\Users\Jane\Foo.kt`, "file:///C:/Users/Jane/Foo.kt"},
		{`C:\Users\Jane\My Project\Foo.kt`, "file:///C:/Users/Jane/My%20Project/Foo.kt"},
		{`app\sub\Foo.kt`, "app/sub/Foo.kt"},
		// UNC: \\server\share\Foo.kt → file://server/share/Foo.kt per the
		// MS-recommended file URI form for UNC paths.
		{`\\server\share\Foo.kt`, "file://server/share/Foo.kt"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := pathToSARIFURI(tc.in); got != tc.want {
				t.Fatalf("pathToSARIFURI(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
