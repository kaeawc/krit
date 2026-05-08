package lsp

import "testing"

func TestPositionToByteOffsetUTF16(t *testing.T) {
	content := []byte("val s = \"😀\"\nval next = 1\n")
	if got := PositionToByteOffset(content, Position{Line: 0, Character: 9}); got != len(`val s = "`) {
		t.Fatalf("emoji start offset = %d, want %d", got, len(`val s = "`))
	}
	if got := PositionToByteOffset(content, Position{Line: 0, Character: 11}); got != len(`val s = "😀`) {
		t.Fatalf("after emoji offset = %d, want %d", got, len(`val s = "😀`))
	}
}

func TestPositionToByteOffsetCRLF(t *testing.T) {
	content := []byte("abc\r\ndef\r\n")
	if got := PositionToByteOffset(content, Position{Line: 0, Character: 99}); got != 3 {
		t.Fatalf("line 0 clamp = %d, want 3", got)
	}
	if got := PositionToByteOffset(content, Position{Line: 1, Character: 1}); got != len("abc\r\nd") {
		t.Fatalf("line 1 char 1 = %d, want %d", got, len("abc\r\nd"))
	}
}

func TestPositionToByteOffsetOutOfRange(t *testing.T) {
	content := []byte("abc")
	if got := PositionToByteOffset(content, Position{Line: 10, Character: 0}); got != len(content) {
		t.Fatalf("out-of-range line = %d, want %d", got, len(content))
	}
}
