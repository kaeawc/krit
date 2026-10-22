package scanner

import "testing"

func TestFindingsBundleKey_XMLInputs(t *testing.T) {
	base := RunFingerprint{Version: "v1", Rules: "r", Config: "c", SourceSet: "s", CrossFile: "x"}
	withXML := base
	withXML.XMLInputs = "xml-a"
	otherXML := base
	otherXML.XMLInputs = "xml-b"
	if FindingsBundleKey(base) == FindingsBundleKey(withXML) {
		t.Error("XMLInputs must participate in the bundle key")
	}
	if FindingsBundleKey(withXML) == FindingsBundleKey(otherXML) {
		t.Error("different XMLInputs must give different bundle keys")
	}
	// Projects without XML keep the key they had before the field existed:
	// the empty value is omitted rather than hashed as an empty field.
	empty := base
	empty.XMLInputs = ""
	if FindingsBundleKey(empty) != FindingsBundleKey(base) {
		t.Error("empty XMLInputs must not change the bundle key")
	}
}

func TestConservativeDeltaPlanner_XMLInputsChangeDisablesDelta(t *testing.T) {
	prev := RunFingerprint{Version: "v1", Rules: "r", Config: "c", SourceSet: "old", CrossFile: "x", XMLInputs: "xml-a"}
	cur := prev
	cur.SourceSet = "new"
	if plan := (ConservativeDeltaPlanner{}).Plan(prev, cur, []string{"/repo/A.kt"}); !plan.ReusePrevious {
		t.Fatal("control: stable XMLInputs should allow delta reuse")
	}
	cur.XMLInputs = "xml-b"
	if plan := (ConservativeDeltaPlanner{}).Plan(prev, cur, []string{"/repo/A.kt"}); plan.ReusePrevious {
		t.Error("an XML edit must disable delta reuse of the prior bundle")
	}
}
