package rules_test

import (
	"strings"
	"testing"
)

func TestLocaleGetDefaultForFormatting_LocalizedBy(t *testing.T) {
	findings := runRuleByName(t, "LocaleGetDefaultForFormatting", `
package test
import java.time.format.DateTimeFormatter
import java.util.Locale
val iso = DateTimeFormatter.ISO_INSTANT.localizedBy(Locale.getDefault())
val root = DateTimeFormatter.ISO_INSTANT.localizedBy(Locale.ROOT)
val pattern = DateTimeFormatter.ofPattern("yyyy").localizedBy(Locale.getDefault())
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for localizedBy(Locale.getDefault()) on an ISO formatter, got %d", len(findings))
	}
	if !strings.HasPrefix(findings[0].Message, "'localizedBy(Locale.getDefault())'") {
		t.Fatalf("message should name localizedBy, got %q", findings[0].Message)
	}
}

func TestLocaleGetDefaultForFormatting_WithLocaleMessage(t *testing.T) {
	findings := runRuleByName(t, "LocaleGetDefaultForFormatting", `
package test
import java.time.format.DateTimeFormatter
import java.util.Locale
val iso = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())
`)
	if len(findings) != 1 || !strings.HasPrefix(findings[0].Message, "'withLocale(Locale.getDefault())'") {
		t.Fatalf("expected 1 withLocale finding, got %v", findings)
	}
}
