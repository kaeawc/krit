package firchecks

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestActiveFirRulesPassesEveryID(t *testing.T) {
	got := ActiveFirRules([]string{"Unknown", "InjectDispatcher", "Unknown"}, false)
	if !reflect.DeepEqual(got.Names, []string{"InjectDispatcher", "Unknown"}) {
		t.Fatalf("names = %v", got.Names)
	}
}
func TestCollectFirCheckFilesAllFiles(t *testing.T) {
	got := CollectFirCheckFiles([]*scanner.File{{Path: "A.kt"}, {Path: "B.kt"}})
	if !got.AllFiles || got.MarkedFiles != 2 {
		t.Fatalf("summary = %+v", got)
	}
}
