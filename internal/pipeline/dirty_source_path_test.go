package pipeline

import "testing"

func TestHasDirtySourcePath(t *testing.T) {
	cases := []struct {
		dirty []string
		want  bool
	}{
		{nil, false},
		{[]string{}, false},
		{[]string{"/r/A.kt"}, true},
		{[]string{"/r/B.java"}, true},
		{[]string{"/r/res/layout/a.xml", "/r/krit.yml", "/r/gradle/libs.versions.toml"}, false},
		{[]string{"/r/build.gradle.kts"}, true},
		{[]string{"/r/scripts/Tool.kts"}, true},
		{[]string{"/r/res/layout/a.xml", "/r/src/A.kt"}, true},
	}
	for _, tc := range cases {
		if got := hasDirtySourcePath(tc.dirty); got != tc.want {
			t.Errorf("hasDirtySourcePath(%v) = %v, want %v", tc.dirty, got, tc.want)
		}
	}
}
