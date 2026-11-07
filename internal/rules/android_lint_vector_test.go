package rules_test

import (
	"reflect"
	"strings"
	"testing"
)

func vectorFile(width, height, pathData string) string {
	return `<vector xmlns:android="http://schemas.android.com/apk/res/android" android:width="` + width + `" android:height="` + height +
		`" android:viewportWidth="24" android:viewportHeight="24">
  <path android:fillColor="#000" android:pathData="` + pathData + `"/>
</vector>
`
}

// Expected positions come from running Android Lint 32.4.1 on the same files.
func TestVectorRasterLintParity(t *testing.T) {
	files := map[string]string{
		"drawable/small.xml": vectorFile("24dp", "24dp", "M0,0L10,10Z"),
		"drawable/wide.xml":  vectorFile("300dp", "24dp", "M0,0L10,10Z"),
		"drawable/tall.xml":  vectorFile("24dp", "250dip", "M0,0L10,10Z"),
		"drawable/both.xml":  vectorFile("300dp", "300dp", "M0,0L10,10Z"),
		"drawable/px.xml":    vectorFile("300px", "300px", "M0,0L10,10Z"),
	}
	want := []string{"drawable/both.xml:1:83", "drawable/tall.xml:1:105", "drawable/wide.xml:1:83"}
	if got := resourcePositions(t, "VectorRaster", files); !reflect.DeepEqual(got, want) {
		t.Fatalf("positions:\n got  %v\n want %v", got, want)
	}
}

func TestInvalidVectorPathLintParity(t *testing.T) {
	files := map[string]string{"drawable/floats.xml": vectorFile("24dp", "24dp", "M1e3,2.5E-2 L.5,-.5 L1.5.5 L3-.25 L4,5.5 .25 6 z m1,1 l-.1-.2.3 .4")}
	want := []string{
		"drawable/floats.xml:2:53", "drawable/floats.xml:2:57", "drawable/floats.xml:2:65", "drawable/floats.xml:2:68",
		"drawable/floats.xml:2:76", "drawable/floats.xml:2:81", "drawable/floats.xml:2:93", "drawable/floats.xml:2:107",
		"drawable/floats.xml:2:110", "drawable/floats.xml:2:113", "drawable/floats.xml:2:116",
	}
	if got := resourcePositions(t, "InvalidVectorPath", files); !reflect.DeepEqual(got, want) {
		t.Fatalf("positions:\n got  %v\n want %v", got, want)
	}
}

func TestVectorPathThreshold(t *testing.T) {
	long := "M0,0 " + strings.Repeat("L1,2 ", 800) + "Z"
	short := "M0,0 " + strings.Repeat("L1,2 ", 400) + "Z"
	files := map[string]string{
		"drawable/long.xml":  vectorFile("24dp", "24dp", long),
		"drawable/short.xml": vectorFile("24dp", "24dp", short),
		"values/strings.xml": `<resources><string name="big">` + long + `</string></resources>`,
		"drawable/ref.xml":   vectorFile("24dp", "24dp", "@string/big"),
	}
	want := []string{"drawable/long.xml:2:52", "drawable/ref.xml:2:52"}
	if got := resourcePositions(t, "VectorPath", files); !reflect.DeepEqual(got, want) {
		t.Fatalf("positions:\n got  %v\n want %v", got, want)
	}
}
