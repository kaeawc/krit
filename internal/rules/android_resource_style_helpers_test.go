package rules

// Direct unit tests for the package-private helpers introduced for AOSP
// PxUsageDetector parity. The end-to-end behavior is covered in the
// rules_test integration tests; these checks pin the helpers themselves
// so a future regression cannot silently change the parsing semantics
// (e.g. "0.5px" being treated as exempt because it starts with '0').

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestParseLeadingNumber(t *testing.T) {
	cases := []struct {
		in    string
		ok    bool
		value float64
	}{
		{"0", true, 0},
		{"0.0", true, 0},
		{"0.5", true, 0.5},
		{"12", true, 12},
		{"24.75", true, 24.75},
		{"", false, 0},
		{"abc", false, 0},
	}
	for _, c := range cases {
		got, ok := parseLeadingNumber(c.in)
		if ok != c.ok || (ok && got != c.value) {
			t.Errorf("parseLeadingNumber(%q) = (%v, %v); want (%v, %v)", c.in, got, ok, c.value, c.ok)
		}
	}
}

func TestPxValueExempt(t *testing.T) {
	cases := map[string]bool{
		"0px":   true,
		"0.0px": true,
		"1px":   true,
		// Only the exact literal "1px" is exempt — see AOSP issue 55722.
		"1.0px": false,
		"2px":   false,
		"24px":  false,
		"":      false,
		// Numeric prefix must be parseable.
		"abcpx": false,
		// Non-px units never qualify.
		"0dp": false,
	}
	for in, want := range cases {
		if got := pxValueExempt(in); got != want {
			t.Errorf("pxValueExempt(%q) = %v; want %v", in, got, want)
		}
	}
}

func TestInOrMmValueExempt(t *testing.T) {
	cases := []struct {
		val  string
		unit string
		want bool
	}{
		{"0mm", "mm", true},
		{"0in", "in", true},
		{"0.0mm", "mm", true},
		{"5mm", "mm", false},
		{"5in", "in", false},
		// Wrong unit yields false.
		{"0mm", "in", false},
		{"", "mm", false},
	}
	for _, c := range cases {
		if got := inOrMmValueExempt(c.val, c.unit); got != c.want {
			t.Errorf("inOrMmValueExempt(%q, %q) = %v; want %v", c.val, c.unit, got, c.want)
		}
	}
}

func TestResolveDimenReference(t *testing.T) {
	idx := &android.ResourceIndex{
		Dimensions: map[string]string{
			"title_size": "14dp",
			"body_size":  "16sp",
		},
		DimensionsLocation: map[string]android.StringLocation{
			"title_size": {FilePath: "res/values/dimens.xml", Line: 12},
		},
	}

	t.Run("plain @dimen ref resolves", func(t *testing.T) {
		val, loc, ok := resolveDimenReference(idx, "@dimen/title_size")
		if !ok || val != "14dp" {
			t.Fatalf("got (%q, %v); want (14dp, true)", val, ok)
		}
		if loc.FilePath != "res/values/dimens.xml" || loc.Line != 12 {
			t.Fatalf("loc = %+v; want dimens.xml:12", loc)
		}
	})

	t.Run("namespaced @android:dimen/x resolves", func(t *testing.T) {
		val, _, ok := resolveDimenReference(idx, "@android:dimen/body_size")
		if !ok || val != "16sp" {
			t.Fatalf("got (%q, %v); want (16sp, true)", val, ok)
		}
	})

	t.Run("unknown ref does not resolve", func(t *testing.T) {
		if _, _, ok := resolveDimenReference(idx, "@dimen/missing"); ok {
			t.Fatal("expected unknown ref to not resolve")
		}
	})

	t.Run("non-dimen ref does not resolve", func(t *testing.T) {
		if _, _, ok := resolveDimenReference(idx, "@string/foo"); ok {
			t.Fatal("expected non-dimen ref to not resolve")
		}
	})

	t.Run("literal value does not resolve", func(t *testing.T) {
		if _, _, ok := resolveDimenReference(idx, "14dp"); ok {
			t.Fatal("expected literal value to not resolve")
		}
	})
}

func TestIsTextSizeAttrAndLayoutHeightAttr(t *testing.T) {
	if !isTextSizeAttr("android:textSize") || !isTextSizeAttr("textSize") {
		t.Error("expected isTextSizeAttr to match both prefixed and unprefixed names")
	}
	if isTextSizeAttr("android:layout_width") {
		t.Error("isTextSizeAttr should not match layout_width")
	}
	if !isLayoutHeightAttr("android:layout_height") || !isLayoutHeightAttr("layout_height") {
		t.Error("expected isLayoutHeightAttr to match both prefixed and unprefixed names")
	}
}
