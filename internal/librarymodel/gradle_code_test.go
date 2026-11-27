package librarymodel

import (
	"slices"
	"testing"
)

// In each source every '#' must be code and every '@' must not be (gradleCode).
func TestGradleCode(t *testing.T) {
	for _, test := range []struct {
		name   string
		groovy bool
		src    string
	}{
		{"line string", false, `# "@@{@" #`},
		{"escaped quote", false, `# "@\"@{" #`},
		{"template code", false, `# "@${#}@" #`},
		{"template with nested quotes and braces", false, `# "@${ # "@{@" # }@" #{#}`},
		{"nested templates", false, `"@${ "@${ # }@" }@" #`},
		{"identifier template", false, `"@$name@{" #`},
		{"raw string", false, "# \"\"\"@\n@\"@{@\"\"\"\" #"},
		{"raw string backslash", false, `# """@\""" # "@" #`},
		{"raw string template", false, `"""@${ # "@" # }@""" #`},
		{"char literal", false, `# '"' # '{' #`},
		{"escaped char literal", false, `# '\'' #`},
		{"backtick name", false, "# `@{@` #"},
		{"line comment", false, "# // @{@\n#"},
		{"block comment", false, "# /* @\n@{ */ #"},
		// Kotlin block comments nest; Groovy's end at the first */ (#1910 review round 3).
		{"nested block comment", false, "# /** @ /* @ */ @{ */ #"},
		{"unterminated nested block comment", false, "# /* @ /* @ */ @"},
		{"groovy block comments don't nest", true, "# /* @ /* @ */ #{"},
		{"groovy single-quoted string", true, `# '@{@"@' #`},
		{"groovy single-quoted string has no template", true, `# '@${@' #`},
		{"groovy triple-single-quoted string", true, "# '''@\n@'@{''' #"},
		{"groovy triple-double escape", true, `# """@\"""@""" #`},
		{"groovy GString template", true, `# "@${ # '@' }@" #`},
		{"unterminated line string ends at the newline", false, "# \"@{\n#"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code := gradleCode(test.src, test.groovy)
			for i := 0; i < len(test.src); i++ {
				switch test.src[i] {
				case '#':
					if !code[i] {
						t.Errorf("byte %d of %q: want code", i, test.src)
					}
				case '@':
					if code[i] {
						t.Errorf("byte %d of %q: want not code", i, test.src)
					}
				}
			}
		})
	}
}

// stripGradleComments drops comments, keeping their newlines, under the same lexical rules as gradleCode, and its
// code mask lines up with what it returns (#1910 review round 3).
func TestStripGradleComments(t *testing.T) {
	for _, test := range []struct {
		name      string
		groovy    bool
		src, want string
	}{
		{"line and block comments", false, "a // b\nc /* d\ne */ f", "a \nc \n f"},
		{"raw string with a quote and a comment marker", false, `val n = """a " // b""" // c` + "\nx", `val n = """a " // b""" ` + "\nx"},
		{"comment markers in line strings", false, `a "//" '/' "/*" b // c`, `a "//" '/' "/*" b `},
		{"nested block comment", false, "a /** b /* c */ d */ e", "a  e"},
		{"groovy block comments don't nest", true, "a /* b /* c */ d */ e", "a  d */ e"},
		{"groovy triple-single-quoted string", true, `def n = '''a ' // b''' // c` + "\nx", `def n = '''a ' // b''' ` + "\nx"},
		{"groovy GString", true, `def n = """a \" /* b""" /* c */ x`, `def n = """a \" /* b"""  x`},
		{"template code comment", false, `"${a /* "}" */ }" // c`, `"${a  }" `},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, code := stripGradleComments(test.src, test.groovy)
			if got != test.want {
				t.Fatalf("stripGradleComments(%q) = %q, want %q", test.src, got, test.want)
			}
			if want := gradleCode(got, test.groovy); !slices.Equal(code, want) {
				t.Fatalf("code mask of %q = %v, want gradleCode's %v", got, code, want)
			}
		})
	}
}

// A binary settings plugin's Kotlin source is Kotlin, not Groovy: only .gradle scripts and .groovy sources are.
func TestIsGroovyScript(t *testing.T) {
	for path, want := range map[string]bool{
		"settings.gradle": true, "build.gradle": true, "src/main/groovy/Catalogs.groovy": true,
		"settings.gradle.kts": false, "my.settings.gradle.kts": false, "src/main/kotlin/Catalogs.kt": false, "": false,
	} {
		if got := isGroovyScript(path); got != want {
			t.Errorf("isGroovyScript(%q) = %v, want %v", path, got, want)
		}
	}
}
