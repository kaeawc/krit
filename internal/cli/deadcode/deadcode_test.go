package deadcode

import "testing"

func TestMultiStringFlag(t *testing.T) {
	var f multiStringFlag
	if err := f.Set("a"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("b"); err != nil {
		t.Fatal(err)
	}
	if got := f.String(); got != "[a b]" {
		t.Errorf("String() = %q, want %q", got, "[a b]")
	}
	if len(f) != 2 || f[0] != "a" || f[1] != "b" {
		t.Errorf("flag = %v, want [a b]", []string(f))
	}
}
