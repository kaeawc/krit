package pipeline

import (
	"sync/atomic"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	xmlHelperKt     = "app/src/main/java/com/example/Helper.kt"
	xmlHelperLayout = "app/src/main/res/layout/main.xml"
	xmlOtherLayout  = "app/src/main/res/layout/other.xml"

	xmlWithHelper    = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<fragment xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    android:name=\"com.example.Helper\" />\n"
	xmlWithoutHelper = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<FrameLayout xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n"
)

// newWarmCrossXMLHarness builds a one-class project whose cross rule reports
// "Helper" when nothing but its own file references it, the way the DeadCode
// rules do. The only possible reference is a class name inside an XML layout,
// which feeds the cross-file index through classRefPatterns.
func newWarmCrossXMLHarness(t *testing.T, files map[string]string) *warmCrossHarness {
	t.Helper()
	files[xmlHelperKt] = "package com.example\n\nclass Helper\n"
	h := newWarmCrossHarness(t, "CrossRule", files)
	calls := h.crossCalls
	h.rule = api.FakeRule("CrossRule", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(calls, 1)
		for _, sym := range ctx.CodeIndex.SymbolsNamed("Helper") {
			if !ctx.CodeIndex.IsReferencedOutsideFile("Helper", sym.File) {
				ctx.Emit(scanner.Finding{File: sym.File, Line: 1, Col: 1, Message: "unused"})
			}
		}
	}))
	return h
}

// Warm cross findings must not outlive an XML edit that changes the indexed
// class references (issue #841): every Kotlin file stays a per-file cache
// hit, so only the XML content tells the cached findings are stale.
func TestRunProject_WarmCrossFindingsInvalidatedByXMLEdit(t *testing.T) {
	h := newWarmCrossXMLHarness(t, map[string]string{xmlHelperLayout: xmlWithHelper})
	h.wantFindings("cold", 0, false)
	h.wantFindings("warm", 0, true)
	if h.calls() != 1 {
		t.Fatalf("warm run must reuse cross findings; calls=%d want 1", h.calls())
	}
	h.write(xmlHelperLayout, xmlWithoutHelper)
	h.wantFindings("after reference removed", 1, false)
	h.wantFindings("warm again", 1, true)
	h.write(xmlHelperLayout, xmlWithHelper)
	h.wantFindings("after reference restored", 0, false)
	h.wantFindings("warm after restore", 0, true)
}

func TestRunProject_WarmCrossFindingsInvalidatedByXMLDelete(t *testing.T) {
	h := newWarmCrossXMLHarness(t, map[string]string{xmlHelperLayout: xmlWithHelper})
	h.wantFindings("cold", 0, false)
	h.wantFindings("warm", 0, true)
	h.remove(xmlHelperLayout)
	h.wantFindings("after layout deleted", 1, false)
	h.wantFindings("warm again", 1, true)
}

func TestRunProject_WarmCrossFindingsInvalidatedByXMLAdd(t *testing.T) {
	h := newWarmCrossXMLHarness(t, map[string]string{xmlOtherLayout: xmlWithoutHelper})
	h.wantFindings("cold", 1, false)
	h.wantFindings("warm", 1, true)
	h.write(xmlHelperLayout, xmlWithHelper)
	h.wantFindings("after layout added", 0, false)
	h.wantFindings("warm again", 0, true)
}
