package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
	if rule.Needs.Has(api.NeedsResolver) || rule.Needs.Has(api.NeedsOracle) ||
		rule.Needs.Has(api.NeedsParsedFiles) || rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatalf("ComposeClickableWithoutMinTouchTarget should stay AST/import-only; got Needs=%b", rule.Needs)
	}
	if rule.TypeInfo != (api.TypeInfoHint{}) {
		t.Fatalf("ComposeClickableWithoutMinTouchTarget TypeInfo=%+v, want zero value", rule.TypeInfo)
	}
}

func TestComposeClickableWithoutMinTouchTarget_UsesLexicalCalleeDispatch(t *testing.T) {
	rule := buildRuleIndex()["ComposeClickableWithoutMinTouchTarget"]
	if rule == nil {
		t.Fatal("ComposeClickableWithoutMinTouchTarget rule not found")
	}
	if len(rule.LexicalCalleeNames) != 1 || rule.LexicalCalleeNames[0] != "clickable" {
		t.Fatalf("ComposeClickableWithoutMinTouchTarget LexicalCalleeNames = %v, want [clickable]", rule.LexicalCalleeNames)
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

func TestComposeRawTextLiteral_PureTemplatesAndPreviewHelpers(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview

@Composable
fun Pure(x: Int, items: List<String>) {
    Text(text = "$x")
    Text("${items.size}")
    Text("$x/${items.size}")
}

@Composable
fun Mixed(x: Int) {
    Text("Total: $x")
}

@Preview
@Composable
fun SamplePreview() {
    Text("Preview only")
    Helper()
}

@Composable
private fun Helper() {
    Text("Helper text")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the mixed template to be reported, got %d", len(findings))
	}
}

func TestComposeRawTextLiteral_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["ComposeRawTextLiteral"]
	if rule == nil {
		t.Fatal("ComposeRawTextLiteral rule is not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("ComposeRawTextLiteral should remain AST-only, got needs %v", rule.Needs)
	}
	if rule.Oracle != nil {
		t.Fatal("ComposeRawTextLiteral should not declare oracle metadata")
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

func TestComposeRawTextLiteral_NegativeEmptyText(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Placeholder() {
    Text("")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for empty Text placeholder, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_NegativeTestSource(t *testing.T) {
	file := parseInline(t, `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun TestContent() {
    Text("Fixture label")
}
`)
	file.Path = "/repo/module/src/androidUnitTest/kotlin/test/Fixture.kt"
	findings := runRuleByNameOnFile(t, "ComposeRawTextLiteral", file)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for test source, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_NegativeDemoSource(t *testing.T) {
	file := parseInline(t, `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun DemoContent() {
    Text("Try dragging the box")
}
`)
	file.Path = "/repo/compose/animation/animation/integration-tests/animation-demos/src/main/kotlin/test/Demo.kt"
	findings := runRuleByNameOnFile(t, "ComposeRawTextLiteral", file)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for demo source, got %d: %v", len(findings), findings)
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

func TestComposeSemanticsMissingRole_NegativeScrim(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.hideFromAccessibility
import androidx.compose.ui.semantics.semantics

@Composable
fun Example(source: Source, onDismiss: () -> Unit) {
    Box(Modifier.fillMaxSize().clickable(interactionSource = source, indication = null) { onDismiss() })
    Box(Modifier.fillMaxSize().semantics { hideFromAccessibility() }.clickable { onDismiss() })
    Box(Modifier.fillMaxSize().clearAndSetSemantics {}.clickable { onDismiss() })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
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

annotation class CustomPreview

@CustomPreview
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

annotation class CustomPreview
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

@CustomPreview
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

func TestComposeSemanticsMissingRole_NegativePreviewPrefixedMultipreview(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

annotation class PreviewDarkLight

@PreviewDarkLight
@Composable
fun Gallery(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for Preview-prefixed annotation, got %d: %v", len(findings), findings)
	}
}

// A class header followed by an annotated `constructor` on its own line makes the grammar emit the later
// declarations as sibling annotation and prefix_expression nodes instead of function_declaration nodes.
func TestComposeSemanticsMissingRole_NegativePreviewAfterMisparsedConstructor(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.PreviewLightDark

data class DialogState
@OptIn(ExperimentalApi::class)
constructor(
  val title: String,
  val onDismiss: () -> Unit,
)

@PreviewLightDark
@Composable
internal fun OverlayCase_WithClear() {
  Surface {
    Box(modifier = Modifier.fillMaxSize()) {
      Text(
        text = "x",
        modifier = Modifier.align(Alignment.Center).clickable { visible = true },
      )
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a preview function after a misparsed constructor, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_PrivateHelperOnlyCalledFromPreviews(t *testing.T) {
	const helper = `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview

@Composable
private fun Demo() {
    Row(Modifier.clickable {}) {}
}

@Preview
@Composable
private fun DemoShot() {
    Demo()
}
`
	if got := runRuleByName(t, "ComposeSemanticsMissingRole", helper); len(got) != 0 {
		t.Fatalf("expected no findings for preview-only helper, got %d: %v", len(got), got)
	}
	withProdCaller := helper + `
@Composable
fun Screen() {
    Demo()
}
`
	if got := runRuleByName(t, "ComposeSemanticsMissingRole", withProdCaller); len(got) != 1 {
		t.Fatalf("expected 1 finding when a non-preview caller exists, got %d: %v", len(got), got)
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
	if rule.Needs.Has(api.NeedsResolver) || rule.Needs.Has(api.NeedsOracle) ||
		rule.Needs.Has(api.NeedsParsedFiles) || rule.Needs.Has(api.NeedsCrossFile) {
		t.Fatalf("ComposeSemanticsMissingRole should stay AST/import-only; got Needs=%b", rule.Needs)
	}
	if rule.TypeInfo != (api.TypeInfoHint{}) {
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

func TestComposeTextFieldMissingLabel_Negative_NotMaterial(t *testing.T) {
	const call = `
@Composable
fun Example() {
    TextField(
        value = "",
        onValueChange = {},
    )
}
`
	for name, header := range map[string]string{
		"other package":     "import com.example.widgets.TextField\nimport androidx.compose.runtime.Composable\n",
		"no import":         "import androidx.compose.runtime.Composable\n",
		"foundation import": "import androidx.compose.foundation.text.TextField\nimport androidx.compose.runtime.Composable\n",
	} {
		if findings := runRuleByName(t, "ComposeTextFieldMissingLabel", "package test\n"+header+call); len(findings) != 0 {
			t.Errorf("%s: expected no findings, got %v", name, findings)
		}
	}
}

func TestComposeTextFieldMissingLabel_Negative_QualifiedAndLocalDeclarations(t *testing.T) {
	findings := runRuleByName(t, "ComposeTextFieldMissingLabel", `
package test

import androidx.compose.material3.*
import androidx.compose.runtime.Composable

data class OutlinedTextField(val value: String, val hint: String)

@Composable
fun Example() {
    Foo.TextField(value = "", onValueChange = {})
    OutlinedTextField(value = "", hint = "")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestComposeTextFieldMissingLabel_Negative_PreviewAndNonComposable(t *testing.T) {
	findings := runRuleByName(t, "ComposeTextFieldMissingLabel", `
package test

import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview

@Preview
@Composable
fun ExamplePreview() {
    TextField(value = "", onValueChange = {})
}

fun build() {
    TextField(value = "", onValueChange = {})
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestComposeTextFieldMissingLabel_Negative_TestSource(t *testing.T) {
	file := parseInline(t, `
package test

import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    TextField(value = "", onValueChange = {})
}
`)
	file.Path = "/repo/app/src/test/kotlin/com/example/Example.kt"
	if findings := runRuleByNameOnFile(t, "ComposeTextFieldMissingLabel", file); len(findings) != 0 {
		t.Fatalf("expected no findings in test sources, got %v", findings)
	}
}

func TestComposeSemanticsMissingRole_Negative_TestSources(t *testing.T) {
	const src = `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Example(onClick: () -> Unit) {
    Row(Modifier.clickable { onClick() }) {}
}
`
	file := parseInline(t, src)
	file.Path = "/repo/app/src/androidTest/kotlin/com/example/Example.kt"
	if findings := runRuleByNameOnFile(t, "ComposeSemanticsMissingRole", file); len(findings) != 0 {
		t.Fatalf("expected no findings in androidTest sources, got %v", findings)
	}
	inTestClass := `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

class ScreenTest {
    @Composable
    fun Example(onClick: () -> Unit) {
        Row(Modifier.clickable { onClick() }) {}
    }
}
`
	if findings := runRuleByName(t, "ComposeSemanticsMissingRole", inTestClass); len(findings) != 0 {
		t.Fatalf("expected no findings in a Test class, got %v", findings)
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

func TestComposeIconButtonMissingContentDescription_NegativePositionalDescription(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    IconButton(onClick = { }) {
        Icon(Icons.Filled.ArrowBack, "Back")
    }
    Icon(Icons.Filled.Add, "Add")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeIconButtonMissingContentDescription_NegativeSameFileHelperForwardsDescription(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable

@Composable
private fun ActionIcon(contentDescription: String) {
    Icon(Icons.Filled.Add, contentDescription)
}

@Composable
fun Example() {
    IconButton(onClick = { }) {
        ActionIcon("Add")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeIconButtonMissingContentDescription_PositiveHelperWithoutDescription(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable

@Composable
private fun ActionIcon(tint: Int) {
    Icon(Icons.Filled.Add)
}

@Composable
fun Example() {
    IconButton(onClick = { }) {
        ActionIcon(1)
    }
}
`)
	if len(findings) < 1 {
		t.Fatalf("expected at least 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_SourceSets(t *testing.T) {
	const code = `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    Text("Welcome")
}
`
	run := func(t *testing.T, rel string) int {
		t.Helper()
		path := filepath.Join(t.TempDir(), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		return len(runRuleByNameOnFile(t, "ComposeRawTextLiteral", file))
	}
	if n := run(t, "app/src/main/kotlin/Header.kt"); n != 1 {
		t.Fatalf("production source should be reported, got %d", n)
	}
	for _, rel := range []string{
		"tools/playground/src/main/kotlin/Header.kt",
		"tools/sandbox/src/main/kotlin/Header.kt",
		"features/debug-tools/src/main/kotlin/Header.kt",
		"app/src/debug/kotlin/Header.kt",
	} {
		if n := run(t, rel); n != 0 {
			t.Errorf("%s should be skipped, got %d findings", rel, n)
		}
	}
}

func TestComposeRawTextLiteral_ConfiguredNonProductionPathMarker(t *testing.T) {
	var impl *rules.ComposeRawTextLiteralRule
	for _, r := range api.Registry {
		if r.ID == "ComposeRawTextLiteral" {
			impl = r.Implementation.(*rules.ComposeRawTextLiteralRule)
		}
	}
	if impl == nil {
		t.Fatal("ComposeRawTextLiteral not registered")
	}
	const code = `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    Text("Welcome")
}
`
	const path = "app/src/staging/kotlin/Header.kt"
	if got := runRuleByNameOnPath(t, "ComposeRawTextLiteral", path, code); len(got) != 1 {
		t.Fatalf("a path without a marker is reported by default, got %d", len(got))
	}
	saved := impl.NonProductionPathMarkers
	t.Cleanup(func() { impl.NonProductionPathMarkers = saved })
	impl.NonProductionPathMarkers = []string{"/src/staging/"}
	if got := runRuleByNameOnPath(t, "ComposeRawTextLiteral", path, code); len(got) != 0 {
		t.Fatalf("a configured marker should skip the file, got %d", len(got))
	}
}

func TestComposeRawTextLiteral_NegativePreviewFunction(t *testing.T) {
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

func TestComposeTextFieldMissingLabel_ReadOnlyAndDebugPathIgnored(t *testing.T) {
	code := `
package test

import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    TextField(value = "", onValueChange = {}, readOnly = true)
}
`
	if got := runRuleByName(t, "ComposeTextFieldMissingLabel", code); len(got) != 0 {
		t.Errorf("expected no findings for a read-only field, got %d", len(got))
	}
	writable := `
package test

import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable

@Composable
fun Example() {
    TextField(value = "", onValueChange = {})
}
`
	debug := runRuleByNameOnPath(t, "ComposeTextFieldMissingLabel", "feature/debug-tools/src/main/kotlin/Screen.kt", writable)
	if len(debug) != 0 {
		t.Errorf("expected no findings in a debug tooling module, got %d", len(debug))
	}
}

func TestComposeRawTextLiteral_InspectionModeBranchIgnored(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalInspectionMode

@Composable
fun Example() {
    val inspecting = LocalInspectionMode.current
    if (LocalInspectionMode.current) {
        Text("Preview text")
    } else {
        Text("Real text")
    }
    if (inspecting) Text("Other preview")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the non-preview branch to be flagged, got %d: %v", len(findings), findings)
	}
}

func TestComposeIconButtonMissingContentDescription_SemanticsAndSlotAccepted(t *testing.T) {
	findings := runRuleByName(t, "ComposeIconButtonMissingContentDescription", `
package test

import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics

@Composable
fun Example(onClick: () -> Unit, icon: @Composable () -> Unit) {
    IconButton(onClick = onClick) { icon() }
    IconButton(onClick = onClick, modifier = Modifier.semantics { contentDescription = "add" }) {
        Icon(imageVector = Add)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_NamedTextArgument(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    Text(text = "Welcome")
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_NegativeNoLettersOrCustomText(t *testing.T) {
	findings := runRuleByName(t, "ComposeRawTextLiteral", `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    Text(" ")
    Text("•")
    Text("1/2")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for letterless text, got %d: %v", len(findings), findings)
	}
	findings = runRuleByName(t, "ComposeRawTextLiteral", `
package test

import com.example.design.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    Text("Welcome")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a non-AndroidX Text, got %d: %v", len(findings), findings)
	}
}

func TestComposeSemanticsMissingRole_PositionalRole(t *testing.T) {
	findings := runRuleByName(t, "ComposeSemanticsMissingRole", `
package test

import androidx.compose.foundation.clickable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role

@Composable
fun Item() {
    Box(Modifier.clickable(true, null, Role.Button) { })
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestToastForAccessibilityAnnouncement_QualifiedToast(t *testing.T) {
	findings := runRuleByName(t, "ToastForAccessibilityAnnouncement", `
package test

fun announceDone(ctx: Context) {
    android.widget.Toast.makeText(ctx, "x", 0).show()
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestComposeRawTextLiteral_PreviewContexts(t *testing.T) {
	const header = `
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.tooling.preview.PreviewFontScale
import androidx.compose.ui.tooling.preview.PreviewLightDark
import androidx.compose.ui.tooling.preview.PreviewScreenSizes

@Composable fun DemoTheme(content: @Composable () -> Unit) = content()
`
	cases := []struct {
		name string
		body string
		want int
	}{
		{"PreviewLightDark", "@PreviewLightDark @Composable fun A() { DemoTheme { Text(\"Menu\") } }", 0},
		{"PreviewScreenSizes", "@PreviewScreenSizes @Composable fun A() { DemoTheme { Text(\"Menu\") } }", 0},
		{"PreviewFontScale", "@PreviewFontScale @Composable fun A() { DemoTheme { Text(\"Menu\") } }", 0},
		{"MultipreviewSameFile", `
@Preview(name = "light")
@Preview(name = "dark")
annotation class Themes

@Themes @Composable fun A() { DemoTheme { Text("Menu") } }`, 0},
		{"MultipreviewNestedFQN", `
@androidx.compose.ui.tooling.preview.Preview
annotation class Variants

@Variants
@Composable
fun A() { DemoTheme { Text("Menu") } }`, 0},
		{"NonPreviewAnnotationClass", `
annotation class Marker

@Marker @Composable fun A() { DemoTheme { Text("Menu") } }`, 1},
		{"InternalCalledOnlyFromPreview", `
@Composable internal fun MenuContent() { DemoTheme { Text("Menu") } }

@Preview @Composable fun MenuScreen() { MenuContent() }`, 0},
		{"InternalAlsoCalledFromProduction", `
@Composable internal fun MenuContent() { DemoTheme { Text("Menu") } }

@Preview @Composable fun MenuScreen() { MenuContent() }
@Composable fun Real() { MenuContent() }`, 1},
		{"PublicCalledOnlyFromPreview", `
@Composable fun MenuContent() { DemoTheme { Text("Menu") } }

@Preview @Composable fun MenuScreen() { MenuContent() }`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "ComposeRawTextLiteral", header+tc.body+"\n")
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d: %v", tc.want, len(findings), findings)
			}
		})
	}
}

func TestComposeRawTextLiteral_PreviewOnlyHelperShapes(t *testing.T) {
	const header = `
package test

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.PreviewFontScale
import androidx.compose.ui.tooling.preview.PreviewLightDark

enum class Direction { Down, Up }

@Composable fun PreviewWidgetTheme(content: @Composable () -> Unit) = content()
`
	const helper = `
@Composable
private fun WidgetPreviewBody(direction: Direction) {
  PreviewWidgetTheme { Surface { Column { Box { Text(text = "Menu") } } } }
}
`
	cases := []struct {
		name string
		body string
		want int
	}{
		{"TwoLightDarkFontScalePreviews", helper + `
@PreviewLightDark @PreviewFontScale @Composable internal fun WidgetPreview() { WidgetPreviewBody(Direction.Down) }
@PreviewLightDark @PreviewFontScale @Composable internal fun WidgetPreviewUp() { WidgetPreviewBody(Direction.Up) }
`, 0},
		{"UnannotatedNamedPreviewCallers", helper + `
@Composable internal fun WidgetPreviewUp() { WidgetPreviewBody(Direction.Up) }
@Composable internal fun WidgetPreviewDown() { WidgetPreviewBody(Direction.Down) }
`, 1},
		{"AlsoCalledFromProduction", helper + `
@PreviewLightDark @Composable internal fun WidgetPreview() { WidgetPreviewBody(Direction.Down) }
@Composable fun Widget() { WidgetPreviewBody(Direction.Up) }
`, 1},
		{"NoPreviewCaller", helper + `
@Composable internal fun WidgetScreen() { WidgetPreviewBody(Direction.Up) }
`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "ComposeRawTextLiteral", header+tc.body)
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d: %v", tc.want, len(findings), findings)
			}
		})
	}
}

func TestPotentialBugsBangBang_BuiltinMultipreviewAnnotations(t *testing.T) {
	const header = `
package test

import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.PreviewLightDark

`
	for body, want := range map[string]int{
		"@PreviewLightDark @Composable fun A(s: String?) { val n = s!!.length }": 0,
		"@Composable fun A(s: String?) { val n = s!!.length }":                   1,
	} {
		findings := runRuleByName(t, "UnsafeCallOnNullableType", header+body+"\n")
		if len(findings) != want {
			t.Errorf("%q: expected %d findings, got %d", body, want, len(findings))
		}
	}
}

const composeMissingRoleHeader = `
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics

`

func TestComposeSemanticsMissingRole_RoleDetectionMatrix(t *testing.T) {
	cases := []struct {
		name string
		code string
		want int
	}{
		{"then if-else clickable, role in later semantics", `
private fun Modifier.field(on: Boolean, f: () -> Unit): Modifier =
    this
        .then(if (on) Modifier.clickable(onClick = f) else Modifier)
        .semantics(mergeDescendants = true) { role = Role.DropdownList }`, 0},
		{"then if-else clickable, role in earlier semantics", `
fun Modifier.field(on: Boolean, f: () -> Unit): Modifier =
    this.semantics { role = Role.Button }.then(if (on) Modifier.clickable(onClick = f) else Modifier)`, 0},
		{"then plain clickable, role later", `
fun Modifier.field(f: () -> Unit): Modifier =
    this.then(Modifier.clickable(onClick = f)).semantics { role = Role.Button }`, 0},
		{"then when branch, role later", `
fun Modifier.field(k: Int, f: () -> Unit): Modifier =
    this.then(when (k) { 0 -> Modifier.toggleable(value = true, onValueChange = {}); else -> Modifier })
        .semantics { role = Role.Switch }`, 0},
		{"then parenthesized, role later", `
fun Modifier.field(f: () -> Unit): Modifier =
    this.then((Modifier.selectable(selected = true, onClick = f))).semantics { role = Role.Tab }`, 0},
		{"then if-else clickable, no role", `
fun Modifier.field(on: Boolean, f: () -> Unit): Modifier =
    this.then(if (on) Modifier.clickable(onClick = f) else Modifier).semantics { stateDescription = "x" }`, 1},
		{"then clickable alone, no role", `
fun Modifier.field(f: () -> Unit): Modifier =
    this.then(Modifier.clickable(onClick = f))`, 1},
		{"role argument inside then", `
fun Modifier.field(f: () -> Unit): Modifier =
    this.then(Modifier.clickable(role = Role.Button, onClick = f))`, 0},
		{"modifier parameter chain with semantics role", `
@Composable
fun Item(modifier: Modifier, f: () -> Unit) {
    Row(modifier.then(Modifier.clickable(onClick = f)).semantics { role = Role.Button }) {}
}`, 0},
		{"role in a separate statement is out of syntactic reach", `
fun Modifier.field(f: () -> Unit): Modifier {
    val base = Modifier.clickable(onClick = f)
    return base.semantics { role = Role.Button }
}`, 1},
		{"selectable positional role", `
@Composable
fun Item(f: () -> Unit) { Row(Modifier.selectable(true, true, Role.RadioButton, f)) {} }`, 0},
		{"toggleable named role", `
@Composable
fun Item() { Row(Modifier.toggleable(value = true, role = Role.Checkbox, onValueChange = {})) {} }`, 0},
		{"semantics without role does not count", `
@Composable
fun Item(f: () -> Unit) { Row(Modifier.semantics { contentDescription = "x" }.clickable(onClick = f)) {} }`, 1},
		{"then argument unrelated to the clickable chain keeps reporting", `
@Composable
fun Item(f: () -> Unit) { Row(Modifier.semantics { role = Role.Button }.then(Modifier.padding(1)).clickable(onClick = f)) {} }`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "ComposeSemanticsMissingRole", composeMissingRoleHeader+tc.code+"\n")
			if len(findings) != tc.want {
				t.Fatalf("want %d findings, got %d: %v", tc.want, len(findings), findings)
			}
		})
	}
}
