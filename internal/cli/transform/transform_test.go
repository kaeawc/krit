package transform

import "testing"

func TestParseArgsAllowsFlagsAfterRecipe(t *testing.T) {
	opts, err := parseArgs([]string{"replace-legacy-timber", "--apply", "--root", "/repo"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.recipe != "replace-legacy-timber" || !opts.apply || opts.root != "/repo" {
		t.Fatalf("parseArgs() = %+v", opts)
	}
}

func TestParseArgsDryRunOverridesApply(t *testing.T) {
	opts, err := parseArgs([]string{"--apply", "replace-legacy-timber", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.apply {
		t.Fatalf("parseArgs() = %+v, want dry run", opts)
	}
}

func TestParseArgsRejectsMissingRecipe(t *testing.T) {
	if _, err := parseArgs([]string{"--apply"}); err == nil {
		t.Fatal("expected missing recipe error")
	}
}
