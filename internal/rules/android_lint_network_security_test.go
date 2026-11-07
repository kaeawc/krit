package rules_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

const networkSecurityConfigSample = `<?xml version="1.0" encoding="utf-8"?>
<network-security-config>
  <base-config cleartextTrafficPermitted="true">
    <trust-anchors>
      <certificates src="system"/>
      <certificates src="user"/>
      <certificates src="bogus"/>
      <certificates/>
      <certs src="system"/>
    </trust-anchors>
    <trust-anchors/>
    <domain>x.com</domain>
  </base-config>
  <base-config/>
  <debug-overrides>
    <trust-anchors><certificates src="user"/></trust-anchors>
  </debug-overrides>
  <debug-overrides/>
  <domain-config cleartextTrafficPermitted="false">
    <domain includeSubdomains="true">example.com</domain>
    <domain includeSubdomains="false">Example.com</domain>
    <domain>noattr.com</domain>
    <domain includeSubdomain="true">typo.com</domain>
    <pin-set expiration="2010-01-01">
      <pin digest="SHA-256">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
    </pin-set>
    <pin-set expiration="2026-11-08">
      <pin digest="SHA-256">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
      <pin digest="SHA-1">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
      <pin digest="SHA-256">AAAA</pin>
      <pin digest="SHA-256">not base64!</pin>
      <pin digest="SHA-256"/>
      <pin>AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
      <pin digst="SHA-256">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
    </pin-set>
    <domain-config>
      <domain>nested.com</domain>
    </domain-config>
    <trust-anchors><certificates src="system"/></trust-anchors>
    <trust-anchors><certificates src="system"/></trust-anchors>
    <pin-sat expiration="2099-01-01"/>
  </domain-config>
  <domain-config>
  </domain-config>
  <domain-config cleartextTrafficPermited="true">
    <domain includeSubdomains="true">a.com</domain>
    <pin-set expiration="2099-13-45"/>
  </domain-config>
  <domain-config>
    <domain includeSubdomains="true">b.com</domain>
    <pin-set expiration="2099-01-01">
      <pin digest="SHA-256">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
    </pin-set>
    <pin-set>
      <pin digest="SHA-256">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
    </pin-set>
    <pin-set/>
  </domain-config>
  <domain-confg/>
  <other/>
</network-security-config>
`

// Expected positions come from running Android Lint 32.4.1 on the same file.
func TestNetworkSecurityConfigLintParity(t *testing.T) {
	files := map[string]string{
		"AndroidManifest.xml": manifestHead + "  <application android:label=\"x\"/>\n</manifest>\n",
		"res/xml/a.xml":       networkSecurityConfigSample,
	}
	want := map[string][]string{
		"MissingBackupPin":        {"res/xml/a.xml:24:6", "res/xml/a.xml:51:6"},
		"PinSetExpiry":            {"res/xml/a.xml:24:26"},
		"AcceptsUserCertificates": {"res/xml/a.xml:6:7"},
		"NetworkSecurityConfig": {
			"res/xml/a.xml:7:26", "res/xml/a.xml:8:8", "res/xml/a.xml:11:6", "res/xml/a.xml:12:6", "res/xml/a.xml:14:4",
			"res/xml/a.xml:18:4", "res/xml/a.xml:21:39", "res/xml/a.xml:22:6", "res/xml/a.xml:23:13", "res/xml/a.xml:27:6",
			"res/xml/a.xml:37:8", "res/xml/a.xml:40:6", "res/xml/a.xml:41:6", "res/xml/a.xml:45:18", "res/xml/a.xml:47:6",
			"res/xml/a.xml:47:26", "res/xml/a.xml:54:6", "res/xml/a.xml:57:6", "res/xml/a.xml:59:4", "res/xml/a.xml:60:4",
		},
	}
	for rule, positions := range want {
		t.Run(rule, func(t *testing.T) {
			if got := projectManifestFindings(t, rule, files); !reflect.DeepEqual(got, positions) {
				t.Fatalf("positions:\n got  %v\n want %v", got, positions)
			}
		})
	}
}

func TestPinSetExpiryWindow(t *testing.T) {
	config := func(date string) map[string]string {
		return map[string]string{
			"AndroidManifest.xml": manifestHead + "  <application android:label=\"x\"/>\n</manifest>\n",
			"res/xml/n.xml": fmt.Sprintf(`<network-security-config>
  <domain-config>
    <domain includeSubdomains="true">example.com</domain>
    <pin-set expiration="%s">
      <pin digest="SHA-256">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
      <pin digest="SHA-256">AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=</pin>
    </pin-set>
  </domain-config>
</network-security-config>
`, date),
		}
	}
	today := time.Now()
	cases := []struct {
		name string
		date string
		want int
	}{
		{"expired", today.AddDate(0, 0, -1).Format("2006-01-02"), 1},
		{"expiring soon", today.AddDate(0, 0, 3).Format("2006-01-02"), 1},
		{"far away", today.AddDate(0, 0, 30).Format("2006-01-02"), 0},
		{"today is not expired but is expiring soon", today.Format("2006-01-02"), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := len(projectManifestFindings(t, "PinSetExpiry", config(c.date))); got != c.want {
				t.Fatalf("findings for %s = %d, want %d", c.date, got, c.want)
			}
		})
	}
}
