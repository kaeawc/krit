package oracle

import (
	"bytes"
	"testing"
)

func TestDropWroteLineKeepsOtherOutput(t *testing.T) {
	var out bytes.Buffer
	w := &dropWroteLine{w: &out}
	for _, chunk := range []string{"warn: a\nWro", "te /tmp/x.json\nwarn: b\n"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := out.String(), "warn: a\nwarn: b\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
