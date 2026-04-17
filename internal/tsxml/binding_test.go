package tsxml

import "testing"

func TestGetLanguage(t *testing.T) {
	lang := GetLanguage()
	if lang == nil {
		t.Fatal("expected GetLanguage() to return non-nil language")
	}
}
