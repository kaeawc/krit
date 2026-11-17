package rules_test

import "testing"

// Receivers of printStackTrace() inside a catch: only the caught exception itself counts.
func TestPrintStackTraceReceiverMustBeTheCaughtException(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"caught variable", `catch (e: Exception) { e.printStackTrace() }`, 1},
		{"safe call", `catch (e: Exception) { e?.printStackTrace() }`, 1},
		{"outer catch variable in a nested catch", `catch (outer: Exception) { try { g() } catch (inner: Exception) { outer.printStackTrace() } }`, 1},
		{"local shadows the caught variable", `catch (e: Exception) { val e = Printer(); e.printStackTrace() }`, 0},
		{"lambda parameter shadows the caught variable", `catch (e: Exception) { printers.forEach { e -> e.printStackTrace() } }`, 0},
		{"loop variable shadows the caught variable", `catch (e: Exception) { for (e in printers) { e.printStackTrace() } }`, 0},
		{"property of this", `catch (e: Exception) { this.e.printStackTrace() }`, 0},
		{"property of another value", `catch (e: Exception) { holder.e.printStackTrace() }`, 0},
		{"local declared after the call", `catch (e: Exception) { e.printStackTrace(); val p = Printer() }`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nclass Printer { fun printStackTrace() {} }\nfun f() {\n    try { g() } " + tc.body + "\n}\n"
			if got := runRuleByName(t, "PrintStackTrace", src); len(got) != tc.want {
				t.Errorf("want %d findings, got %v", tc.want, got)
			}
		})
	}
}
