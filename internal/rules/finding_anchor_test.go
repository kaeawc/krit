package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// wantAnchor fails unless exactly one finding sits at line:col and its byte span covers a non-empty range.
func wantAnchor(t *testing.T, findings []scanner.Finding, line, col int) scanner.Finding {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.Line != line || f.Col != col {
		t.Fatalf("got %d:%d, want %d:%d", f.Line, f.Col, line, col)
	}
	if f.EndByte <= f.StartByte {
		t.Fatalf("expected a byte span, got %d-%d", f.StartByte, f.EndByte)
	}
	return f
}

func TestSafeCastAnchorsOnTheCastNotTheIsCheck(t *testing.T) {
	findings := runRedundantSafeCast(t, `
package test

fun f(holder: Any) {
    if (holder is String) {
        (holder as String).length
    }
}
`)
	f := wantAnchor(t, findings, 6, 10)
	if got := f.EndByte - f.StartByte; got != len("holder as String") {
		t.Fatalf("span length %d, want the as-expression", got)
	}
}

func TestComposeClickableAnchorsOnCalleeNameInSingleLineChain(t *testing.T) {
	findings := runRuleByName(t, "ComposeClickableWithoutMinTouchTarget", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.width
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

fun Example(onTap: () -> Unit) {
    Box(Modifier.width(36.dp).clickable { onTap() })
}
`)
	wantAnchor(t, findings, 11, 31)
}

func TestComposeSemanticsMissingRoleAnchorsOnCalleeNameInMultiLineChain(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.ui.Modifier

fun Example(onTap: () -> Unit) {
    val m =
        Modifier.fillMaxWidth()
            .clickable { onTap() }
}
`)
	wantAnchor(t, findings, 10, 14)
}

func TestRedundantSuspendModifierAnchorsOnSuspendKeyword(t *testing.T) {
	findings := runRuleByName(t, "RedundantSuspendModifier", `
package test

private suspend fun plain(x: Int): Int = x + 1
`)
	f := wantAnchor(t, findings, 4, 9)
	if got := f.EndByte - f.StartByte; got != len("suspend") {
		t.Fatalf("span length %d, want the suspend keyword", got)
	}
}

func TestOkHttpCallExecuteSyncAnchorsOnExecuteName(t *testing.T) {
	findings := runRuleByName(t, "OkHttpCallExecuteSync", `
package test

import okhttp3.OkHttpClient
import okhttp3.Request

suspend fun load(client: OkHttpClient, request: Request) {
    client.newCall(request).execute()
}
`)
	wantAnchor(t, findings, 8, 29)
}

func TestTooGenericExceptionThrownAnchorsOnTheThrowKeyword(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "TooGenericExceptionThrown", `
package test

fun fail() {
    throw
        RuntimeException("boom")
}
`)
	wantAnchor(t, findings, 5, 5)
}
