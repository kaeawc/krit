package librarymodel

import "testing"

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
