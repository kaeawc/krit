package rules_test

import (
	"fmt"
	"testing"
)

// The firebaseAppDistribution half of WrongGradleMethod cannot be recorded with the Lint CLI (a KTS script that uses
// the extension without its plugin does not compile), so these cases follow the detector's checkCli source.
func TestWrongGradleMethodFirebaseAppDistribution(t *testing.T) {
	block := `android {
    buildTypes {
        getByName("release") {
            firebaseAppDistribution {
                artifactType = "APK"
            }
        }
    }
}
`
	cases := []struct {
		name, path, content string
		want                []string
	}{
		{"kts without import", "app/build.gradle.kts", block, []string{"4:13"}},
		{"kts with import", "app/build.gradle.kts",
			"import com.google.firebase.appdistribution.gradle.firebaseAppDistribution\n\n" + block, nil},
		{"groovy is not checked", "app/build.gradle", block, nil},
		{"commented out", "app/build.gradle.kts", "// firebaseAppDistribution {\n// }\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, f := range runLintGradleRule(t, "WrongGradleMethod", c.path, c.content) {
				got = append(got, fmt.Sprintf("%d:%d", f.Line, f.Col))
			}
			if fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

// A dependencies block in a commented-out or string context, or under an unrelated parent, is not reported.
func TestWrongGradleMethodDependenciesNegatives(t *testing.T) {
	content := `android {
    buildTypes {
        /* debug { dependencies { implementation("x:y:1") } } */
        debug {
            val note = "dependencies { }"
        }
    }
    defaultConfig {
        dependencies { }
    }
}
dependencies { }
`
	if got := runLintGradleRule(t, "WrongGradleMethod", "app/build.gradle.kts", content); len(got) != 0 {
		t.Fatalf("expected no findings, got %v", got)
	}
}
