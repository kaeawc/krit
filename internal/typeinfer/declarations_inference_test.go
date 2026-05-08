package typeinfer

import (
	"testing"
)

func TestPropertyInference_LazyDelegate_StringLiteral(t *testing.T) {
	src := `
val label by lazy { "hello" }
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	label := fi.RootScope.Lookup("label")
	if label == nil {
		t.Fatal("expected delegated property to be declared")
	}
	if label.Name != "String" {
		t.Fatalf("expected delegated property type String, got %q", label.Name)
	}
}

func TestPropertyInference_RememberDelegate_IntLiteral(t *testing.T) {
	src := `
val count by remember { 42 }
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil {
		t.Fatal("expected remembered property to be declared")
	}
	if count.Name != "Int" {
		t.Fatalf("expected remembered property type Int, got %q", count.Name)
	}
}

func TestPropertyInference_RememberDelegate_LastExpression(t *testing.T) {
	src := `
val count by remember {
    val base = 40
    42
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil {
		t.Fatal("expected remembered property to be declared")
	}
	if count.Name != "Int" {
		t.Fatalf("expected remembered property type Int from last expression, got %q", count.Name)
	}
}

func TestPropertyInference_LazyDelegate_LastExpression(t *testing.T) {
	src := `
val answer by lazy {
    val base = 40
    "hello"
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	answer := fi.RootScope.Lookup("answer")
	if answer == nil {
		t.Fatal("expected property to be declared")
	}
	if answer.Name != "String" {
		t.Fatalf("expected property type String from last expression, got %q", answer.Name)
	}
}

func TestPropertyInference_LazyDelegate_LastExpressionLocalIdentifier(t *testing.T) {
	src := `
val answer by lazy {
    val local = "hello"
    local
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	answer := fi.RootScope.Lookup("answer")
	if answer == nil {
		t.Fatal("expected property to be declared")
	}
	if answer.Name != "String" {
		t.Fatalf("expected property type String from local identifier last expression, got %q", answer.Name)
	}
}

func TestPropertyInference_LazyDelegate_CallExpressionLastValue(t *testing.T) {
	src := `
fun provideLabel(): String = "hello"

val answer by lazy {
    val prefix = "ignored"
    provideLabel()
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	answer := fi.RootScope.Lookup("answer")
	if answer == nil {
		t.Fatal("expected property to be declared")
	}
	if answer.Name != "String" {
		t.Fatalf("expected property type String from call-expression last value, got %q", answer.Name)
	}
}

func TestPropertyInference_RememberDelegate_LastExpressionNavigation(t *testing.T) {
	src := `
val count by remember {
    val local = "hello"
    local.length
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil {
		t.Fatal("expected remembered property to be declared")
	}
	if count.Name != "Int" {
		t.Fatalf("expected remembered property type Int from navigation last expression, got %q", count.Name)
	}
}

func TestPropertyInference_DirectCallExpressionReceiver_First_ReturnsString(t *testing.T) {
	src := `
fun provideItems(): List<String> = listOf()
val result = provideItems().first()
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	result := fi.RootScope.Lookup("result")
	if result == nil {
		t.Fatal("expected direct initializer property to be declared")
	}
	if result.Name != "String" {
		t.Fatalf("expected direct initializer type String, got %q", result.Name)
	}
	if result.Nullable {
		t.Fatal("expected direct initializer type to be non-nullable")
	}
}

func TestPropertyInference_DirectCallExpressionReceiver_FirstOrNull_ReturnsNullableString(t *testing.T) {
	src := `
fun provideItems(): List<String> = listOf()
val result = provideItems().firstOrNull()
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	result := fi.RootScope.Lookup("result")
	if result == nil {
		t.Fatal("expected direct initializer property to be declared")
	}
	if result.Name != "String" {
		t.Fatalf("expected direct initializer type String, got %q", result.Name)
	}
	if !result.Nullable {
		t.Fatal("expected direct initializer type to be nullable")
	}
}

func TestPropertyInference_RememberDelegate_CallExpressionLastValue(t *testing.T) {
	src := `
fun provideCount(): Int = 42

val count by remember {
    val ignored = "still ignored"
    provideCount()
}
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil {
		t.Fatal("expected remembered property to be declared")
	}
	if count.Name != "Int" {
		t.Fatalf("expected remembered property type Int from call-expression last value, got %q", count.Name)
	}
}

func TestPropertyInference_ListOfInt_RetainsTypeArg_ForFirstOrNull(t *testing.T) {
	src := `
val items = listOf<Int>()
val result = items.firstOrNull()
`
	file := parseTestFile(t, src)
	resolver := buildTestResolver(t, file)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	items := fi.RootScope.Lookup("items")
	if items == nil {
		t.Fatal("expected items to be declared")
	}
	if items.Name != "List" {
		t.Fatalf("expected items type List, got %q", items.Name)
	}
	if len(items.TypeArgs) != 1 {
		t.Fatalf("expected items to retain 1 type arg, got %d", len(items.TypeArgs))
	}
	if items.TypeArgs[0].Name != "Int" {
		t.Fatalf("expected items type arg Int, got %q", items.TypeArgs[0].Name)
	}

	idx := flatFirstOfTypeWithText(file, "call_expression", "items.firstOrNull()")
	if idx == 0 {
		t.Fatal("expected to find call_expression for items.firstOrNull()")
	}

	got := resolver.ResolveFlatNode(idx, file)
	if got == nil {
		t.Fatal("expected type for items.firstOrNull(), got nil")
	}
	if got.Name != "Int" {
		t.Fatalf("expected firstOrNull result Int, got %q", got.Name)
	}
	if !got.Nullable {
		t.Fatal("expected firstOrNull result to be nullable")
	}
}

func TestDestructuringInference_PairConstructor(t *testing.T) {
	src := `
val (count, name) = Pair(42, "hello")
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil || count.Name != "Int" {
		t.Fatalf("expected destructured count Int, got %#v", count)
	}
	name := fi.RootScope.Lookup("name")
	if name == nil || name.Name != "String" {
		t.Fatalf("expected destructured name String, got %#v", name)
	}
}

func TestDestructuringInference_PairConstructor_CallArgs(t *testing.T) {
	src := `
fun provideCount(): Int = 42
fun provideName(): String = "hello"

val (count, name) = Pair(provideCount(), provideName())
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil || count.Name != "Int" {
		t.Fatalf("expected destructured count Int from call args, got %#v", count)
	}
	name := fi.RootScope.Lookup("name")
	if name == nil || name.Name != "String" {
		t.Fatalf("expected destructured name String from call args, got %#v", name)
	}
}

func TestDestructuringInference_TripleConstructor(t *testing.T) {
	src := `
val (count, name, enabled) = Triple(42, "hello", true)
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil || count.Name != "Int" {
		t.Fatalf("expected destructured count Int, got %#v", count)
	}
	name := fi.RootScope.Lookup("name")
	if name == nil || name.Name != "String" {
		t.Fatalf("expected destructured name String, got %#v", name)
	}
	enabled := fi.RootScope.Lookup("enabled")
	if enabled == nil || enabled.Name != "Boolean" {
		t.Fatalf("expected destructured enabled Boolean, got %#v", enabled)
	}
}

func TestDestructuringInference_TripleConstructor_CallArgs(t *testing.T) {
	src := `
fun provideCount(): Int = 42
fun provideName(): String = "hello"
fun provideEnabled(): Boolean = true

val (count, name, enabled) = Triple(provideCount(), provideName(), provideEnabled())
`
	file := parseTestFile(t, src)
	fi := IndexFileParallel(file)
	if fi == nil {
		t.Fatal("expected FileTypeInfo")
	}

	count := fi.RootScope.Lookup("count")
	if count == nil || count.Name != "Int" {
		t.Fatalf("expected destructured count Int from call args, got %#v", count)
	}
	name := fi.RootScope.Lookup("name")
	if name == nil || name.Name != "String" {
		t.Fatalf("expected destructured name String from call args, got %#v", name)
	}
	enabled := fi.RootScope.Lookup("enabled")
	if enabled == nil || enabled.Name != "Boolean" {
		t.Fatalf("expected destructured enabled Boolean from call args, got %#v", enabled)
	}
}
