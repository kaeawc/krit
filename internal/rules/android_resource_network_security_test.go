package rules_test

import (
	"strings"
	"testing"
)

// Expected findings were produced by running Android Lint 32.4.1 on the same files.
func TestInsecureBaseConfigurationResourceLintParity(t *testing.T) {
	files := map[string]string{
		"layout/main.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`,
		"xml/nsc.xml": `<network-security-config>
  <base-config cleartextTrafficPermitted="true">
    <trust-anchors><certificates src="user"/><certificates src="system"/></trust-anchors>
  </base-config>
  <domain-config><domain>example.com</domain><trust-anchors><certificates src="user"/></trust-anchors></domain-config>
  <debug-overrides><trust-anchors><certificates src="user"/></trust-anchors></debug-overrides>
</network-security-config>`,
		"xml/other.xml": `<paths><base-config cleartextTrafficPermitted="true"/></paths>`,
	}
	got := strings.Join(resourceFindings(t, "InsecureBaseConfigurationResource", files), "\n")
	if want := "xml/nsc.xml:02 Insecure Base Configuration"; got != want {
		t.Errorf("InsecureBaseConfiguration:\ngot  %s\nwant %s", got, want)
	}
	// Lint reports at the first character of the attribute value.
	if got, want := resourceFindingColumns(t, "InsecureBaseConfigurationResource", files), "xml/nsc.xml:2:43"; got != want {
		t.Errorf("InsecureBaseConfiguration position:\ngot  %s\nwant %s", got, want)
	}
}
