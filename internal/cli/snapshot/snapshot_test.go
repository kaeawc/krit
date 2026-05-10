package snapshot

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestFormatInfoError_FriendlyOnMissing(t *testing.T) {
	got := formatInfoError("abc1234", fs.ErrNotExist)
	if !strings.Contains(got, "abc1234") {
		t.Errorf("missing arg in message: %q", got)
	}
	if !strings.Contains(got, "is not a captured snapshot") {
		t.Errorf("missing friendly hint: %q", got)
	}
	if !strings.Contains(got, "krit snapshot status") {
		t.Errorf("missing status hint: %q", got)
	}
}

func TestFormatInfoError_PassthroughOnOtherError(t *testing.T) {
	custom := errors.New("boom")
	got := formatInfoError("abc1234", custom)
	if !strings.Contains(got, "boom") {
		t.Errorf("expected non-ENOENT error to pass through: %q", got)
	}
	if strings.Contains(got, "is not a captured snapshot") {
		t.Errorf("non-ENOENT error should not get the friendly hint: %q", got)
	}
}
