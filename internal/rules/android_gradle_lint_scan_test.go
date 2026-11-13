package rules

import (
	"fmt"
	"testing"
)

func describeLintGradleStatements(stmts []lintGradleStmt) []string {
	var out []string
	for _, st := range stmts {
		out = append(out, fmt.Sprintf("%d:%s/%s>%s=%s", st.Line, st.GrandParent, st.Parent, st.Name, st.Value))
	}
	return out
}

func TestScanLintGradleStatements(t *testing.T) {
	src := `android {
    defaultConfig { minSdkVersion 21; targetSdkVersion = 34 } // trailing comment
    /* block
       minSdkVersion 99 */
    productFlavors {
        dev { applicationIdSuffix("x") }
        create("prod") {
            minSdk = 21
        }
    }
    if (flag) {
        compileSdk = 33
    } else {
        compileSdk = 34
    }
}
dependencies {
    implementation 'a:b:1' // 'quote
    implementation("c:d:${v}") { exclude(group = "g") }
    api platform("e:f:1")
    def local = 1
    "compile"("g:h:2")
    implementation(
        "multi:line:1"
    )
}
`
	got := describeLintGradleStatements(scanLintGradleStatements(src))
	want := []string{
		"2:android/defaultConfig>minSdkVersion=21",
		"2:android/defaultConfig>targetSdkVersion=34",
		"6:productFlavors/dev>applicationIdSuffix=\"x\"",
		"8:productFlavors/create>minSdk=21",
		"12:/android>compileSdk=33",
		"14:/android>compileSdk=34",
		"18:/dependencies>implementation='a:b:1'",
		"19:dependencies/implementation>exclude=group = \"g\"",
		"20:/dependencies>api=platform(\"e:f:1\")",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("statements:\n got %q\nwant %q", got, want)
	}
}

func TestScanLintGradleStatementsKeepsColumns(t *testing.T) {
	stmts := scanLintGradleStatements("dependencies {\n    implementation 'a:b:1'\n}\n")
	if len(stmts) != 1 || stmts[0].Line != 2 || stmts[0].Col != 5 || stmts[0].ValueCol != 20 {
		t.Fatalf("unexpected statement %+v", stmts)
	}
}
