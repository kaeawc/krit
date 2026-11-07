package rules_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func propertyFindingPositions(root string, findings []scanner.Finding) string {
	var out string
	for _, f := range findings {
		rel, _ := filepath.Rel(root, f.File)
		out += fmt.Sprintf("%s:%d:%d;", filepath.ToSlash(rel), f.Line, f.Col)
	}
	return out
}

// Expected positions come from running Android Lint 32.4.1 on the same files.
func TestUsingHttpAndProxyPasswordLintParity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "gradle", "wrapper", "gradle-wrapper.properties"), `distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
distributionUrl=http\://services.gradle.org/distributions/gradle-8.0-bin.zip
zipStoreBase=GRADLE_USER_HOME
`)
	writeFile(t, filepath.Join(root, "gradle.properties"), `org.gradle.jvmargs=-Xmx2g
systemProp.http.proxyPassword=secret
systemProp.https.proxyPassword=secret2
distributionUrl=http\://example.com/x.zip
`)
	writeFile(t, filepath.Join(root, "local.properties"), `distributionUrl=http\://local.example.com/x.zip
systemProp.http.proxyPassword=x
`)
	writeFile(t, filepath.Join(root, "foo.properties"), `distributionUrl=http\://foo.example.com/x.zip
`)

	http := buildRuleIndex()["UsingHttp"]
	got := propertyFindingPositions(root, runSupplyModuleRule(t, http, root))
	want := "gradle.properties:4:17;gradle/wrapper/gradle-wrapper.properties:3:17;local.properties:1:17;"
	if got != want {
		t.Errorf("UsingHttp = %s, want %s", got, want)
	}
	password := buildRuleIndex()["ProxyPassword"]
	got = propertyFindingPositions(root, runSupplyModuleRule(t, password, root))
	want = "gradle.properties:2:31;gradle.properties:3:32;local.properties:2:31;"
	if got != want {
		t.Errorf("ProxyPassword = %s, want %s", got, want)
	}
}

func TestProxyPasswordSkipsGitIgnoredFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".gitignore"), "local.properties\n")
	writeFile(t, filepath.Join(root, "local.properties"), "systemProp.http.proxyPassword=x\n")
	writeFile(t, filepath.Join(root, "gradle.properties"), "# noinspection ProxyPassword\nsystemProp.https.proxyPassword=y\n")
	password := buildRuleIndex()["ProxyPassword"]
	if got := propertyFindingPositions(root, runSupplyModuleRule(t, password, root)); got != "" {
		t.Errorf("ProxyPassword = %s, want none", got)
	}
}
