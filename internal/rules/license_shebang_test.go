package rules_test

import "testing"

func TestLicenseHeaderFollowsShebang(t *testing.T) {
	const withHeader = "#!/usr/bin/env kotlin\n/* Copyright 2024 Example */\nprintln(1)\n"
	if got := lintAt(t, "AbsentOrWrongFileLicense", "tool.main.kts", withHeader); len(got) != 0 {
		t.Fatalf("license after the shebang line is the header: %v", got)
	}
	const missing = "#!/usr/bin/env kotlin\nprintln(1)\n"
	got := lintAt(t, "AbsentOrWrongFileLicense", "tool.main.kts", missing)
	if len(got) != 1 || got[0].Fix == nil {
		t.Fatalf("want one fixable finding, got %v", got)
	}
	if want := len("#!/usr/bin/env kotlin\n"); got[0].Fix.StartByte != want {
		t.Fatalf("fix inserts at byte %d, want %d (after the shebang line)", got[0].Fix.StartByte, want)
	}
}
