package rules

import "testing"

func TestSdkSetAlgebra(t *testing.T) {
	ge23, lt25 := sdkFrom(23), sdkSet{{0, 24}}
	if got := ge23.and(lt25).describe(); got != "≥ 23 and < 25" {
		t.Errorf("[23,25) described as %q", got)
	}
	if got := sdkFrom(21).and(sdkSet{{23, 23}}.not()).describe(); got != "≥ 21 and < 23 or ≥ 24" {
		t.Errorf("not 23 above 21 described as %q", got)
	}
	if got := (sdkSet{{23, 23}}).describe(); got != "= 23" {
		t.Errorf("exact level described as %q", got)
	}
	multi := sdkSet{{21, 22}, {24, sdkInf}}
	if !sdkFrom(21).openEnded() || lt25.openEnded() || !multi.openEnded() {
		t.Error("openEnded must look at the upper bound of the last range")
	}
	if !sdkAll.not().isEmpty() || !sdkSet(nil).not().equal(sdkAll) {
		t.Error("negating everything is nothing, and the reverse")
	}
	if got := ge23.or(sdkSet{{0, 20}}); !got.equal(sdkSet{{0, 20}, {23, sdkInf}}) {
		t.Errorf("union = %v", got)
	}
}

func TestSdkMaxEmptyConstraintIsNone(t *testing.T) {
	empty := &sdkCons{neg: true}
	got := sdkMax(&sdkCons{set: sdkFrom(21), neg: true}, empty)
	if got == nil || !got.none {
		t.Errorf("intersecting with an empty constraint = %+v, want NONE", got)
	}
	if got := sdkMax(nil, empty); got != empty {
		t.Errorf("a missing operand must leave the other as is")
	}
	if got := (&sdkCons{set: sdkFrom(21)}).not(); got.neg || !got.set.isEmpty() {
		t.Errorf("a constraint that cannot be negated negates to an empty one, got %+v", got)
	}
}

func TestSdkFolderVersion(t *testing.T) {
	cases := []struct {
		folder  string
		base    string
		version int
		ok      bool
	}{
		{"layout-v19", "layout", 19, true},
		{"layout-land-v19", "layout-land", 19, true},
		{"values-en-rUS-v21", "values-en-rUS", 21, true},
		{"values-b+sr+Latn-v21", "values-b+sr+Latn", 21, true},
		{"values-mcc310-mnc004-sw600dp-night-xhdpi-v26", "values-mcc310-mnc004-sw600dp-night-xhdpi", 26, true},
		{"drawable-anydpi-v21", "drawable-anydpi", 21, true},
		{"values-ldrtl-v17", "values-ldrtl", 17, true},
		{"values-car-v21", "values-car", 21, true},
		{"layout-v21-land", "", 0, false},
		{"values-v19-land", "", 0, false},
		{"layout-land-sw600dp-v19", "", 0, false},
		{"values-rUS-v19", "", 0, false},
		{"layout-land", "", 0, false},
		{"raw-v19", "", 0, false},
		{"other-v19", "", 0, false},
		{"layout-v", "", 0, false},
		{"layout", "", 0, false},
	}
	for _, c := range cases {
		base, version, ok := sdkFolderVersion(c.folder)
		if base != c.base || version != c.version || ok != c.ok {
			t.Errorf("sdkFolderVersion(%q) = %q, %d, %v; want %q, %d, %v", c.folder, base, version, ok, c.base, c.version, c.ok)
		}
	}
}

func TestSdkTargetApiLevel(t *testing.T) {
	cases := map[string]int{"19": 19, "1": 1, "KITKAT": 19, "kitkat": 19, "L": 21, "lollipop": 21, "Jelly_Bean": 16, "M": 23}
	for value, want := range cases {
		if got, ok := sdkTargetAPILevel(value); !ok || got != want {
			t.Errorf("sdkTargetAPILevel(%q) = %d, %v; want %d", value, got, ok, want)
		}
	}
	for _, value := range []string{"", "0", "019", " 19 ", "19L", "0x13", "-1", "abc", "JELLYBEAN", "K"} {
		if got, ok := sdkTargetAPILevel(value); ok {
			t.Errorf("sdkTargetAPILevel(%q) = %d, want no level", value, got)
		}
	}
}
