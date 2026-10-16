package rules_test

import "testing"

// TestSwallowedExceptionWrapperJava pins Java parity for the wrapper
// exemption: consumed wrapper forms must never be flagged. (A discarded
// `new Analytics(e);` is a known Java false negative, not asserted here.)
func TestSwallowedExceptionWrapperJava(t *testing.T) {
	wrap := func(body string) string {
		return "class C {\n  void m() {\n    try { work(); } catch (Exception e) {\n      " + body + "\n    }\n  }\n  void work() {}\n  void log(Object o) {}\n}\n"
	}
	cases := []struct {
		name string
		body string
		want int
	}{
		{"thrown wrapper", "throw new Wrapper(e);", 0},
		{"assigned wrapper", "Object w = new Wrapper(e); log(w);", 0},
		{"wrapper passed as argument", "log(new Wrapper(e));", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runExceptionsRuleOnSource(t, "SwallowedException", ".java", wrap(tc.body))
			if len(findings) != tc.want {
				t.Errorf("got %d findings, want %d: %v", len(findings), tc.want, findings)
			}
		})
	}
}
