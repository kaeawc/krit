package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestAnalyzeProjectArgsKotlinSettingsWire(t *testing.T) {
	empty, err := json.Marshal(AnalyzeProjectArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(empty), "oracle_kotlin") {
		t.Fatalf("absent settings must not be sent: %s", empty)
	}
	in := AnalyzeProjectArgs{OracleKotlin: gradlemodel.KotlinSettings{
		LanguageVersion: "1.9", APIVersion: "1.8", OptIns: []string{"a.A"}, Progressive: true,
		Flags: []string{"-Xcontext-receivers"}, Notes: []string{"note"}, Rejected: []string{"-Werror (x)"},
	}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out AnalyzeProjectArgs
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.OracleKotlin.Identity() != in.OracleKotlin.Identity() || out.OracleKotlin.Identity() == "" {
		t.Fatalf("settings did not survive the daemon protocol: %s", data)
	}
	if len(out.OracleKotlin.Notes) != 1 || len(out.OracleKotlin.Rejected) != 1 {
		t.Fatalf("diagnostics were dropped: %+v", out.OracleKotlin)
	}
}
