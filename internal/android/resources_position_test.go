package android

import "testing"

func TestParseLayoutRecordsElementAndAttributePositions(t *testing.T) {
	src := "<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\">\n" +
		"    <TextView\n" +
		"        android:text=\"hi\" />\n" +
		"</LinearLayout>\n"
	layout, err := parseLayoutBytes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	tv := layout.RootView.Children[0]
	if tv.Line != 2 || tv.Col != 5 {
		t.Fatalf("element at %d:%d, want 2:5", tv.Line, tv.Col)
	}
	if line, col := tv.PosOf("android:text"); line != 3 || col != 9 {
		t.Fatalf("attribute at %d:%d, want 3:9", line, col)
	}
	if line, col := tv.PosOf("android:missing"); line != 2 || col != 5 {
		t.Fatalf("missing attribute falls back to %d:%d, want 2:5", line, col)
	}
}

// Version 9 added View.AttrPos; an older cache entry decodes without it and
// would anchor attribute findings at the element start.
func TestResourceCacheVersionCoversAttributePositions(t *testing.T) {
	if resourceCacheVersion < 9 {
		t.Fatalf("resourceCacheVersion = %d, want >= 9 so entries without View.AttrPos are discarded", resourceCacheVersion)
	}
}

// A DOM parser numbers every child node, so whitespace text runs and comments take indexes.
func TestParseLayoutNumbersChildrenLikeADOM(t *testing.T) {
	src := "<LinearLayout>\n" +
		"    <!-- note -->\n" +
		"    <TextView/>\n" +
		"    <Button/><View/>text &amp; more<Space/>\n" +
		"</LinearLayout>\n"
	layout, err := parseLayoutBytes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"TextView": 3, "Button": 5, "View": 6, "Space": 8}
	for _, child := range layout.RootView.Children {
		if got := child.DOMIndex; got != want[child.Type] {
			t.Errorf("%s DOMIndex = %d, want %d", child.Type, got, want[child.Type])
		}
	}
}
