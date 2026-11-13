package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// relativeOverlapFindings scans one layout file and returns what RelativeOverlapResource reports.
func relativeOverlapFindings(t *testing.T, layout string) []scanner.Finding {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "res/layout"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "res/layout/a.xml"), []byte(layout), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := android.ScanResourceDir(filepath.Join(dir, "res"))
	if err != nil {
		t.Fatal(err)
	}
	return runResourceRule(findResourceRule(t, "RelativeOverlapResource"), idx)
}

const relOverlapHead = `<RelativeLayout xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools"
    android:layout_width="match_parent" android:layout_height="match_parent">
`

func TestRelativeOverlapReportsLeftAndRightTextThatCanGrow(t *testing.T) {
	f := relativeOverlapFindings(t, relOverlapHead+`
    <TextView android:id="@+id/title" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentLeft="true" android:text="@string/title" />
    <TextView android:id="@+id/count" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentRight="true" android:text="@string/count" />
</RelativeLayout>`)
	if len(f) != 1 {
		t.Fatalf("want one finding, got %v", findingMessages(f))
	}
	want := "`@id/count` can overlap `@id/title` if @string/title, @string/count grow due to localized text expansion"
	if f[0].Message != want || f[0].Line != 7 {
		t.Errorf("got line %d %q, want line 7 %q", f[0].Line, f[0].Message, want)
	}
}

func TestRelativeOverlapIgnoresTwoViewsOnTheSameSide(t *testing.T) {
	// The previous heuristic reported two left-aligned views; Lint only pairs a left view with a right view.
	f := relativeOverlapFindings(t, relOverlapHead+`
    <TextView android:id="@+id/a" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentLeft="true" android:text="@string/a" />
    <TextView android:id="@+id/b" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentStart="true" android:text="@string/b" />
</RelativeLayout>`)
	if len(f) != 0 {
		t.Fatalf("want no findings, got %v", findingMessages(f))
	}
}

func TestRelativeOverlapNeedsGrowableText(t *testing.T) {
	cases := map[string]string{
		"literal text keeps its width": `android:text="Hi"`,
		"fixed width":                  `android:text="@string/a" android:layout_width="40dp"`,
	}
	for name, attrs := range cases {
		t.Run(name, func(t *testing.T) {
			f := relativeOverlapFindings(t, relOverlapHead+`
    <TextView android:id="@+id/a" android:layout_height="wrap_content" android:layout_width="wrap_content"
        android:layout_alignParentLeft="true" `+attrs+` />
    <ImageView android:id="@+id/b" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentRight="true" />
</RelativeLayout>`)
			if len(f) != 0 {
				t.Fatalf("want no findings, got %v", findingMessages(f))
			}
		})
	}
}

func TestRelativeOverlapSkipsDifferentVerticalBuckets(t *testing.T) {
	f := relativeOverlapFindings(t, relOverlapHead+`
    <TextView android:id="@+id/a" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentLeft="true" android:layout_alignParentTop="true" android:text="@string/a" />
    <TextView android:id="@+id/b" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentRight="true" android:layout_alignParentBottom="true" android:text="@string/b" />
    <TextView android:id="@+id/c" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentRight="true" android:layout_below="@id/a" android:text="@string/c" />
    <TextView android:id="@+id/d" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentRight="true" android:visibility="gone" android:text="@string/d" />
</RelativeLayout>`)
	if len(f) != 0 {
		t.Fatalf("want no findings, got %v", findingMessages(f))
	}
}

func TestRelativeOverlapFollowsToEndOfChains(t *testing.T) {
	// The chain a -> b grows to the right, so a right-aligned c can reach it.
	f := relativeOverlapFindings(t, relOverlapHead+`
    <TextView android:id="@+id/a" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentStart="true" android:text="@string/a" />
    <TextView android:id="@+id/b" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_toEndOf="@id/a" android:text="@string/b" />
    <TextView android:id="@+id/c" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentEnd="true" android:text="@string/c" />
</RelativeLayout>`)
	want := "`@id/c` can overlap `@id/b` if @string/b, @string/c grow due to localized text expansion"
	if len(f) != 1 || f[0].Message != want {
		t.Fatalf("want [%q], got %v", want, findingMessages(f))
	}
}

func TestRelativeOverlapNamesViewsWithoutIdsByDomPosition(t *testing.T) {
	// Lint numbers nodes among all DOM children, so whitespace text and comments take indexes.
	f := relativeOverlapFindings(t, relOverlapHead+`
    <!-- start -->
    <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentLeft="true" android:text="@string/a" />
    <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentRight="true" android:text="@string/b" />
</RelativeLayout>`)
	if len(f) != 1 {
		t.Fatalf("want one finding, got %v", findingMessages(f))
	}
	want := "`TextView-5` can overlap `TextView-3` if @string/a, @string/b grow due to localized text expansion"
	if f[0].Message != want {
		t.Errorf("message = %q, want %q", f[0].Message, want)
	}
}

func TestRelativeOverlapHonorsToolsIgnoreOnTheReportedView(t *testing.T) {
	f := relativeOverlapFindings(t, relOverlapHead+`
    <TextView android:id="@+id/a" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentLeft="true" android:text="@string/a" tools:ignore="RelativeOverlap" />
    <TextView android:id="@+id/b" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_alignParentRight="true" android:text="@string/b" tools:ignore="RelativeOverlap" />
</RelativeLayout>`)
	if len(f) != 0 {
		t.Fatalf("want no findings, got %v", findingMessages(f))
	}
}

func TestRelativeOverlapSurvivesReferenceCycles(t *testing.T) {
	f := relativeOverlapFindings(t, relOverlapHead+`
    <TextView android:id="@+id/a" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_toEndOf="@id/b" android:text="@string/a" />
    <TextView android:id="@+id/b" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_toEndOf="@id/c" android:text="@string/b" />
    <TextView android:id="@+id/c" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_toEndOf="@id/a" android:text="@string/c" />
    <TextView android:id="@+id/d" android:layout_width="wrap_content" android:layout_height="wrap_content"
        android:layout_toEndOf="@id/d" android:layout_alignParentRight="true" android:text="@string/d" />
</RelativeLayout>`)
	_ = f // the check is that the rule terminates
}
