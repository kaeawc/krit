package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestTestWithoutAssertion_ConstructionAndNoOpTests(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import kotlin.test.assertFailsWith
import org.junit.Test

class WidgetIdTest {
    @Test
    fun invalidIdFails() {
        assertFailsWith<IllegalArgumentException> { WidgetId("") }
    }

    @Test
    fun createsValidInstance() {
        WidgetId("abc")
    }

    @Test
    fun createsFromFactory() {
        WidgetId.create("abc")
    }
}

class PlayerTest {
    @Test
    fun reset_is_no_op() {
        player.reset()
    }

    @Test
    fun resetIsNoop() {
        player.reset()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("construction and no-op tests are not missing assertions, got %d: %v", len(findings), findings)
	}
}

func TestTestWithoutAssertion_ConstructionWithoutThrowingSiblingIsReported(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class WidgetIdTest {
    @Test
    fun createsValidInstance() {
        WidgetId("abc")
    }

    @Test
    fun loadsTwice() {
        load()
        load()
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both tests reported, got %d", len(findings))
	}
}

// runIgnoredReturnValueWithRangedOracle gives the receiver and the call that starts at the same position
// different compiler types: the receiver owns the line:col key, the call is the shared-start fact.
func runIgnoredReturnValueWithRangedOracle(t *testing.T, code, callText, receiverText, receiverType, callType string) []scanner.Finding {
	t.Helper()
	file := parseInlineForRound6(t, code)
	var call, receiver uint32
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if file.FlatType(idx) == "call_expression" && file.FlatNodeText(idx) == callText {
			call = idx
		}
	})
	if call != 0 {
		file.FlatWalkAllNodes(call, func(idx uint32) {
			if receiver == 0 && file.FlatType(idx) == "simple_identifier" && file.FlatNodeText(idx) == receiverText {
				receiver = idx
			}
		})
	}
	if call == 0 || receiver == 0 {
		t.Fatalf("missing nodes: call=%d receiver=%d", call, receiver)
	}
	key := func(idx uint32) string { return fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1) }
	o, err := oracle.LoadFromData(&oracle.Data{
		Version:       1,
		KotlinVersion: "2.1.0",
		Files: map[string]*oracle.File{
			file.Path: {Expressions: map[string]*oracle.ExpressionType{
				key(receiver): {Type: receiverType, Nullable: true, CallTarget: "kotlinx.coroutines.flow.FlowKt.collect", StartByte: int(file.FlatStartByte(receiver)), EndByte: int(file.FlatEndByte(receiver))},
				fmt.Sprintf("%s@%d", key(call), file.FlatEndByte(call)): {Type: callType, StartByte: int(file.FlatStartByte(call)), EndByte: int(file.FlatEndByte(call))},
			}},
		},
		Dependencies: map[string]*oracle.Class{},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(o, resolver)
	for _, r := range api.Registry {
		if r.ID == "IgnoredReturnValue" {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("rule IgnoredReturnValue not found")
	return nil
}

func TestIgnoredReturnValue_TypeComesFromTheCallNotItsReceiver(t *testing.T) {
	code := `package test
import kotlinx.coroutines.flow.MutableStateFlow

class Holder(private val flows: MutableMap<String, MutableStateFlow<Int>>) {
    suspend fun reset(key: String) {
        flows[key]?.emit(0)
    }
}
`
	findings := runIgnoredReturnValueWithRangedOracle(t, code, "flows[key]?.emit(0)", "flows",
		"kotlinx.coroutines.flow.MutableStateFlow<kotlin.Int>", "kotlin.Unit")
	if len(findings) != 0 {
		t.Fatalf("a Unit call must not be reported through its receiver's type, got %v", findings)
	}
}

func TestIgnoredReturnValue_MapRemoveIsAMutator(t *testing.T) {
	code := `package test
import kotlinx.coroutines.flow.MutableStateFlow

class Holder {
    private val flows = mutableMapOf<String, MutableStateFlow<Int>>()
    fun clear(key: String) {
        flows.remove(key)
    }
}
`
	findings := runIgnoredReturnValueWithRangedOracle(t, code, "flows.remove(key)", "flows",
		"kotlin.collections.MutableMap<kotlin.String, kotlinx.coroutines.flow.MutableStateFlow<kotlin.Int>>",
		"kotlinx.coroutines.flow.MutableStateFlow<kotlin.Int>")
	if len(findings) != 0 {
		t.Fatalf("a collection mutator is not an ignored return value, got %v", findings)
	}
}

func parseInlineForRound6(t *testing.T, code string) *scanner.File {
	t.Helper()
	return parseInline(t, code)
}

const fieldImports = `package test
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
`

func TestComposeTextFieldMissingLabel_DescribedRepeatedCellIsSkipped(t *testing.T) {
	findings := runRuleByName(t, "ComposeTextFieldMissingLabel", fieldImports+`
@Composable
fun Code(values: List<String>) {
    repeat(6) { index ->
        OutlinedTextField(
            value = values[index],
            onValueChange = {},
            modifier = Modifier.semantics { contentDescription = "Digit " + index },
        )
    }
    (0 until 4).map { index ->
        OutlinedTextField(value = values[index], onValueChange = {}, modifier = Modifier.semantics { contentDescription = "Pin " + index })
    }
}

@Composable
fun Pin(value: String) {
    OutlinedTextField(
        value = value,
        onValueChange = {},
        singleLine = true,
        modifier = Modifier.size(48.dp).semantics { contentDescription = "Pin digit" },
    )
}
`)
	if len(findings) != 0 {
		t.Fatalf("described cells are labelled, got %d: %v", len(findings), findings)
	}
}

func TestComposeTextFieldMissingLabel_UndescribedRepeatedCellIsReported(t *testing.T) {
	findings := runRuleByName(t, "ComposeTextFieldMissingLabel", fieldImports+`
@Composable
fun Code(values: List<String>) {
    repeat(6) { index ->
        OutlinedTextField(value = values[index], onValueChange = {}, singleLine = true)
    }
}

@Composable
fun Wide(value: String) {
    OutlinedTextField(
        value = value,
        onValueChange = {},
        singleLine = true,
        modifier = Modifier.width(300.dp).semantics { contentDescription = "Name" },
    )
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both fields reported, got %d: %v", len(findings), findings)
	}
}

func TestComposeTextFieldMissingLabel_SingleCharBoxSuggestsSemantics(t *testing.T) {
	findings := runRuleByName(t, "ComposeTextFieldMissingLabel", fieldImports+`
@Composable
fun Box(value: String, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = value,
        onValueChange = { onChange(it.take(1)) },
    )
}

@Composable
fun DescribedBox(value: String, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = value,
        onValueChange = { if (it.length <= 1) onChange(it) },
        modifier = Modifier.semantics { contentDescription = "Digit" },
    )
}

@Composable
fun Name(value: String, onChange: (String) -> Unit) {
    OutlinedTextField(value = value, onValueChange = onChange)
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected the undescribed box and the plain field, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "contentDescription") || strings.Contains(findings[0].Message, "missing a `label`") {
		t.Errorf("single-character box should suggest semantics, got %q", findings[0].Message)
	}
	if !strings.Contains(findings[1].Message, "missing a `label`") {
		t.Errorf("a plain field should still ask for a label, got %q", findings[1].Message)
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_FakeWithReplaceablePrivateScopeIsSkipped(t *testing.T) {
	findings := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
class FakeLoader {
    private var loadScope = CoroutineScope(SupervisorJob())
    fun swapScope(replacement: CoroutineScope) {
        loadScope = replacement
    }
}
class StubRunner {
    private val scope = CoroutineScope(SupervisorJob())
}
`)
	if len(findings) != 0 {
		t.Fatalf("fakes and stubs leave scope cancellation to the test, got %v", findings)
	}
}
