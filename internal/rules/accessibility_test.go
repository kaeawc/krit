package rules_test

import (
	"testing"

	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

func TestAnimatorDurationIgnoresScale_Positive(t *testing.T) {
	findings := runRuleByName(t, "AnimatorDurationIgnoresScale", `
package test

import android.animation.ValueAnimator

fun example() {
    ValueAnimator.ofFloat(0f, 1f).apply {
        duration = 300
    }.start()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestAnimatorDurationIgnoresScale_Negative(t *testing.T) {
	findings := runRuleByName(t, "AnimatorDurationIgnoresScale", `
package test

import android.animation.ValueAnimator
import android.content.ContentResolver
import android.provider.Settings

fun example(contentResolver: ContentResolver) {
    val scale = Settings.Global.getFloat(
        contentResolver,
        Settings.Global.ANIMATOR_DURATION_SCALE,
        1f,
    )

    ValueAnimator.ofFloat(0f, 1f).apply {
        duration = (300 * scale).toLong()
    }.start()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeClickableWithoutMinTouchTarget_PositiveWidth(t *testing.T) {
	findings := runRuleByName(t, "ComposeClickableWithoutMinTouchTarget", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example(onTap: () -> Unit) {
    Box(Modifier.width(36.dp).clickable { onTap() })
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeClickableWithoutMinTouchTarget_NegativeNoExplicitSize(t *testing.T) {
	findings := runRuleByName(t, "ComposeClickableWithoutMinTouchTarget", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Example(onTap: () -> Unit) {
    Box(Modifier.clickable { onTap() })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeClickableWithoutMinTouchTarget_NegativeMinimumInteractiveComponentSize(t *testing.T) {
	findings := runRuleByName(t, "ComposeClickableWithoutMinTouchTarget", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example(onTap: () -> Unit) {
    Box(
        Modifier
            .minimumInteractiveComponentSize()
            .width(36.dp)
            .clickable { onTap() },
    )
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeClickableWithoutMinTouchTarget_NegativeExplicitMinimumSize(t *testing.T) {
	findings := runRuleByName(t, "ComposeClickableWithoutMinTouchTarget", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Example(onTap: () -> Unit) {
    Box(Modifier.size(48.dp).clickable { onTap() })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeClickableWithoutMinTouchTarget_NegativeLocalModifierLookalike(t *testing.T) {
	findings := runRuleByName(t, "ComposeClickableWithoutMinTouchTarget", `
package test

object Modifier {
    fun size(value: Dp): Modifier = this
    fun clickable(onClick: () -> Unit): Modifier = this
}
class Dp
val Int.dp: Dp get() = Dp()

fun Example(onTap: () -> Unit) {
    Modifier.size(24.dp).clickable { onTap() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local Modifier lookalike, got %d: %v", len(findings), findings)
	}
}

func TestComposeClickableWithoutMinTouchTarget_DoesNotRequireTypeContext(t *testing.T) {
	rule := buildRuleIndex()["ComposeClickableWithoutMinTouchTarget"]
	if rule == nil {
		t.Fatal("ComposeClickableWithoutMinTouchTarget rule not found")
	}
	if rule.Needs.Has(v2rules.NeedsResolver) || rule.Needs.Has(v2rules.NeedsOracle) ||
		rule.Needs.Has(v2rules.NeedsParsedFiles) || rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatalf("ComposeClickableWithoutMinTouchTarget should stay AST/import-only; got Needs=%b", rule.Needs)
	}
	if rule.TypeInfo != (v2rules.TypeInfoHint{}) {
		t.Fatalf("ComposeClickableWithoutMinTouchTarget TypeInfo=%+v, want zero value", rule.TypeInfo)
	}
}

// --- ComposeDecorativeImageContentDescription ---

func TestComposeDecorativeImageContentDescription_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeDecorativeImageContentDescription", `
package test

import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource

@Composable
fun Example() {
    Image(
        painter = painterResource(R.drawable.decoration),
        contentDescription = null,
        modifier = Modifier.clickable { },
    )
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeDecorativeImageContentDescription_NegativeClearAndSetSemantics(t *testing.T) {
	findings := runRuleByName(t, "ComposeDecorativeImageContentDescription", `
package test

import androidx.compose.foundation.Image
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.clearAndSetSemantics

@Composable
fun Example() {
    Image(
        painter = painterResource(R.drawable.decoration),
        contentDescription = null,
        modifier = Modifier.clearAndSetSemantics { },
    )
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- ComposeIconButtonMissingContentDescription ---

func TestComposeIconButtonMissingContentDescription_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    IconButton(onClick = { }) {
        Icon(Icons.Filled.ArrowBack)
    }
}
`)
	if len(findings) < 1 {
		t.Fatalf("expected at least 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeIconButtonMissingContentDescription_Negative(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    IconButton(onClick = { }) {
        Icon(
            imageVector = Icons.Filled.ArrowBack,
            contentDescription = "Back",
        )
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeIconButtonMissingContentDescription_NegativeNullIsPresentDescription(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    IconButton(onClick = { }) {
        Icon(
            imageVector = Icons.Filled.ArrowBack,
            contentDescription = null,
        )
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeIconButtonMissingContentDescription_NegativeLocalLookalikes(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

fun Icon(value: Any? = null) = Unit
fun Image(value: Any? = null) = Unit
fun AsyncImage(value: Any? = null) = Unit
fun IconButton(onClick: () -> Unit, content: () -> Unit) = content()

fun Example() {
    IconButton(onClick = { }) {
        Icon()
    }
    Image()
    AsyncImage()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeIconButtonMissingContentDescription_PositiveFullyQualifiedCall(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.runtime.Composable

@Composable
fun Example() {
    androidx.compose.material3.IconButton(onClick = { }) {
        androidx.compose.material3.Icon(Icons.Filled.ArrowBack)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

// --- ComposeRawTextLiteral ---

func TestComposeRawTextLiteral_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    Text("Welcome")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_NegativeStringResource(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Header() {
    Text(stringResource(R.string.welcome))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_NegativePreview(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview

@Preview
@Composable
fun HeaderPreview() {
    Text("Welcome")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- ComposeSemanticsMissingRole ---

func TestComposeSemanticsMissingRole_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Example(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_NegativeExplicitRole(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role

@Composable
fun Example(onClick: () -> Unit) {
    Row(Modifier.clickable(role = Role.Button) { onClick() }) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_NegativeSemanticsBlock(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics

@Composable
fun Example(onClick: () -> Unit) {
    Row(
        Modifier
            .clickable { onClick() }
            .semantics { role = Role.Button },
    ) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_NegativePreview(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

annotation class SignalPreview

@SignalPreview
@Composable
fun RadioRowPreview(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings in preview code, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_NegativePrivatePreviewNestedCall(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

annotation class SignalPreview
interface RowScope

object Rows {
    @Composable
    fun RadioRow(
        content: @Composable RowScope.() -> Unit,
        selected: Boolean,
        modifier: Modifier = Modifier,
        enabled: Boolean = true,
    ) {}

    @Composable
    fun RadioRow(selected: Boolean, text: String, modifier: Modifier = Modifier) {}
}

object Previews {
    @Composable
    fun Preview(content: @Composable () -> Unit) {}
}

@SignalPreview
@Composable
private fun RadioRowPreview() {
    Previews.Preview {
        Rows.RadioRow(
            true,
            "RadioRow",
            modifier = Modifier.clickable {},
        )
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings in nested private preview code, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_NegativeDisabled(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Example(onClick: () -> Unit) {
    Row(Modifier.clickable(enabled = false) { onClick() }) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for disabled clickable, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_NegativeLocalModifierLookalike(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

object Modifier {
    fun clickable(onClick: () -> Unit): Modifier = this
}

fun Example(onTap: () -> Unit) {
    Modifier.clickable { onTap() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local Modifier lookalike, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_DoesNotRequireTypeContext(t *testing.T) {
	rule := buildRuleIndex()["ComposeSemanticsMissingRole"]
	if rule == nil {
		t.Fatal("ComposeSemanticsMissingRole rule not found")
	}
	if rule.Needs.Has(v2rules.NeedsResolver) || rule.Needs.Has(v2rules.NeedsOracle) ||
		rule.Needs.Has(v2rules.NeedsParsedFiles) || rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatalf("ComposeSemanticsMissingRole should stay AST/import-only; got Needs=%b", rule.Needs)
	}
	if rule.TypeInfo != (v2rules.TypeInfoHint{}) {
		t.Fatalf("ComposeSemanticsMissingRole TypeInfo=%+v, want zero value", rule.TypeInfo)
	}
}

// --- ComposeTextFieldMissingLabel ---

func TestComposeTextFieldMissingLabel_Positive(t *testing.T) {
	findings := runRuleByName(t, "ComposeTextFieldMissingLabel", `
package test

import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    TextField(
        value = "",
        onValueChange = {},
    )
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeTextFieldMissingLabel_NegativeWithLabel(t *testing.T) {
	findings := runRuleByName(t, "ComposeTextFieldMissingLabel", `
package test

import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    TextField(
        value = "",
        onValueChange = {},
        label = { Text("Email") },
    )
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- ToastForAccessibilityAnnouncement ---

func TestToastForAccessibilityAnnouncement_Positive(t *testing.T) {
	findings := runRuleByName(t, "ToastForAccessibilityAnnouncement", `
package test

import android.widget.Toast

fun announceAccessibilityChange(context: android.content.Context) {
    Toast.makeText(context, "Updated", Toast.LENGTH_SHORT).show()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestToastForAccessibilityAnnouncement_NegativeNonA11yContext(t *testing.T) {
	findings := runRuleByName(t, "ToastForAccessibilityAnnouncement", `
package test

import android.widget.Toast

fun showMessage(context: android.content.Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).show()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}
