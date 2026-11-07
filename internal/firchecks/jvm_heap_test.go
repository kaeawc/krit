package firchecks

import "testing"

func TestFirHeapFlag(t *testing.T) {
	cases := map[string]string{
		"":         "-XX:MaxRAMPercentage=25",
		"8g":       "-Xmx8g",
		" 512M ":   "-Xmx512M",
		"lots":     "-XX:MaxRAMPercentage=25",
		"8g -foo":  "-XX:MaxRAMPercentage=25",
		"-Xmx8g":   "-XX:MaxRAMPercentage=25",
		"24G":      "-Xmx24G",
		"1024k":    "-Xmx1024k",
		"8gb":      "-XX:MaxRAMPercentage=25",
		"0x10":     "-XX:MaxRAMPercentage=25",
		"\t4g\n":   "-Xmx4g",
		"16 g":     "-XX:MaxRAMPercentage=25",
		"g":        "-XX:MaxRAMPercentage=25",
		"2147483g": "-Xmx2147483g",
	}
	for in, want := range cases {
		getenv := func(key string) string {
			if key == "KRIT_FIR_XMX" {
				return in
			}
			return ""
		}
		if got := firHeapFlag(getenv); got != want {
			t.Errorf("KRIT_FIR_XMX=%q: got %q, want %q", in, got, want)
		}
	}
}

func TestFirExtraJVMArgs(t *testing.T) {
	getenv := func(string) string { return "-XX:+HeapDumpOnOutOfMemoryError  -Dx=y" }
	got := firExtraJVMArgs(getenv)
	if len(got) != 2 || got[0] != "-XX:+HeapDumpOnOutOfMemoryError" || got[1] != "-Dx=y" {
		t.Fatalf("got %v", got)
	}
	if extra := firExtraJVMArgs(func(string) string { return "" }); len(extra) != 0 {
		t.Fatalf("unset env should add nothing, got %v", extra)
	}
}
