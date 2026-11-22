package typeinfer

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestWrittenProjectLookalikeDoesNotPromoteUnresolvedType(t *testing.T) {
	file := parseTestFile(t, "package example\nclass Map\n")
	resolver := buildTestResolver(t, file)
	result := &ResolvedType{Name: "Map", FQN: "Map", Kind: TypeClass}
	resolver.markWrittenProjectType(result, "Map", file, 0)
	if !result.ProjectLookalike || result.IsResolved() {
		t.Fatalf("lookalike provenance must not promote an unresolved type: %+v", result)
	}
}

func TestStdlibMethodsProjectNameVisibility(t *testing.T) {
	for _, test := range []struct {
		name        string
		declaration string
		use         string
		unknown     bool
	}{
		{"same package", "package example\nclass Map", "package example", false},
		{"private other file", "package example\nprivate class Map", "package example", false},
		{"other package", "package other\nclass Map", "package example", false},
		{"explicit project import", "package other\nclass Map", "package example\nimport other.Map", false},
		{"explicit library import", "package example\nclass Map", "package example\nimport kotlin.collections.Map", false},
		{"nested same package", "package example\nclass Screen { class Map }", "package example", false},
		{"other package list", "package other\nclass List", "package example", false},
		{"same package context", "package example\nclass Context", "package example", false},
		{"explicit context import", "package other\nclass Context", "package example\nimport other.Context", false},
		{"unimported context", "package other\nclass Context", "package example", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			name, methodName := "Map", "get"
			typeArgs, arguments := "<String, Any>", `"key"`
			if test.name == "other package list" {
				name, methodName = "List", "firstOrNull"
				typeArgs, arguments = "<Any>", ""
			}
			if strings.Contains(test.name, "context") {
				name, methodName = "Context", "getSystemService"
				typeArgs = ""
			}
			declaration := parseTestFile(t, test.declaration+"\n")
			call := "items." + methodName + "(" + arguments + ")"
			file := parseTestFile(t, test.use+"\nfun read(items: "+name+typeArgs+") = "+call+"\n")
			resolver := NewResolver()
			resolver.IndexFilesParallel([]*scanner.File{declaration, file}, 1)
			idx := flatFirstOfTypeWithText(file, "call_expression", call)
			if idx == 0 {
				t.Fatal("missing receiver call")
			}
			got := resolver.ResolveFlatNode(idx, file)
			if (got.Kind == TypeUnknown) != test.unknown {
				t.Fatalf("type = %+v, want unknown = %v", got, test.unknown)
			}
			if !test.unknown && !got.IsNullable() {
				t.Fatalf("library receiver lost nullable signature: %+v", got)
			}
		})
	}
}

func TestStdlibMethodsExplicitImportBeatsPrivateTopLevelLookalike(t *testing.T) {
	cases := []struct {
		name string
		src  string
		call string
	}{
		{
			name: "android context",
			src: `import android.content.Context
private class Context
fun service(context: Context) = context.getSystemService("service")`,
			call: `context.getSystemService("service")`,
		},
		{
			name: "kotlin map",
			src: `import kotlin.collections.Map
private class Map
fun lookup(items: Map<String, Any>) = items.get("k")`,
			call: `items.get("k")`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := parseTestFile(t, "package example\n"+tc.src+"\n")
			resolver := buildTestResolver(t, file)
			idx := flatFirstOfTypeWithText(file, "call_expression", tc.call)
			if idx == 0 {
				t.Fatal("missing receiver call")
			}
			got := resolver.ResolveFlatNode(idx, file)
			if got.Kind == TypeUnknown || !got.IsNullable() {
				t.Fatalf("explicit library import did not retain its stdlib signature: %+v", got)
			}
		})
	}
}

func TestStdlibMethodsCachedSameFileTypes(t *testing.T) {
	declaration := parseTestFile(t, "package example\nclass Map\n")
	file := parseTestFile(t, `package example
class Context
fun written(items: Map<String, Any>) = items.get("key")
fun inferred() = mapOf("key" to "value").get("key")
fun local(context: Context) = context.getSystemService("service")
`)
	files := []*scanner.File{declaration, file}
	cacheDir := t.TempDir()
	for scan := 0; scan < 2; scan++ {
		resolver := NewResolver()
		hits, misses := resolver.IndexFilesParallelCachedWithTracker(files, 1, cacheDir, nil)
		if scan == 0 && (hits != 0 || misses != 2) || scan == 1 && (hits != 2 || misses != 0) {
			t.Fatalf("scan %d: hits %d, misses %d", scan, hits, misses)
		}
		for _, test := range []struct {
			call    string
			unknown bool
		}{
			{`items.get("key")`, false},
			{`mapOf("key" to "value").get("key")`, false},
			{`context.getSystemService("service")`, true},
		} {
			idx := flatFirstOfTypeWithText(file, "call_expression", test.call)
			if idx == 0 {
				t.Fatalf("missing call %s", test.call)
			}
			got := resolver.ResolveFlatNode(idx, file)
			if (got.Kind == TypeUnknown) != test.unknown || !test.unknown && !got.IsNullable() {
				t.Fatalf("scan %d, %s: unexpected type %+v", scan, test.call, got)
			}
		}
	}
}

func TestStdlibMethodsRejectLocalLookalikes(t *testing.T) {
	for key := range StdlibMethods {
		if strings.HasPrefix(key, "_.") {
			continue
		}
		receiver, methodName, _ := strings.Cut(key, ".")
		t.Run(key, func(t *testing.T) {
			file := parseTestFile(t, "package example\nclass "+receiver+"\nfun read(items: "+receiver+") = items\n")
			resolver := buildTestResolver(t, file)
			idx := flatFirstOfTypeWithText(file, "user_type", receiver)
			if idx == 0 {
				t.Fatal("missing written receiver type")
			}
			typ := resolver.ResolveFlatNode(idx, file)
			if !typ.ProjectLookalike || typ.FQN != "example."+receiver {
				t.Fatalf("missing project type provenance: %+v", typ)
			}
			if got := resolver.lookupReceiverStdlibMethod(typ, methodName); got == nil || got.ReturnType.Kind != TypeUnknown {
				t.Fatalf("local lookalike matched stdlib signature: %+v", got)
			}
		})
	}
}

func TestStdlibMethodsWrittenTypeSegments(t *testing.T) {
	for _, test := range []struct {
		name string
		want bool
	}{
		{"Map", true},
		{"Map<String, Any>", true},
		{"Map.Entry", false},
		{"kotlin.collections.Map<String, Any>", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := parseTestFile(t, "package example\nclass Map\nfun read(items: "+test.name+") = items\n")
			resolver := buildTestResolver(t, file)
			idx := flatFirstOfTypeWithText(file, "user_type", test.name)
			if idx == 0 {
				t.Fatal("missing written type")
			}
			if typ := resolver.ResolveFlatNode(idx, file); typ.ProjectLookalike != test.want {
				t.Fatalf("type = %+v, want project lookalike = %v", typ, test.want)
			}
		})
	}
}

func TestStdlibMethodsLocalNameScopes(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		unknown bool
	}{
		{"visible nested", `class Screen { class Map { fun get(key: String): Any = Any() }
fun read(items: Map) = items.get("key") }`, true},
		{"visible local", `fun read() { class Map { fun get(key: String): Any = Any() }
val items = Map()
items.get("key") }`, true},
		{"bare generic constructor", `class Map<Key, Value> { fun get(key: Key): Value = TODO() }
fun read() { val items = Map<String, Any>(); items.get("key") }`, true},
		{"visible object", `object Map { fun get(key: String): Any = Any() }
fun read(items: Map) = items.get("key")`, true},
		{"visible interface", `interface Map { fun get(key: String): Any }
fun read(items: Map) = items.get("key")`, true},
		{"nested before import", `import kotlin.collections.Map
class Screen { class Map { fun get(key: String): Any = Any() }
fun read(items: Map) = items.get("key") }`, true},
		{"property annotation", `class Map { fun get(key: String): Any = Any() }
val items: Map = TODO()
fun read() = items.get("key")`, true},
		{"return annotation", `class Map { fun get(key: String): Any = Any() }
fun items(): Map = TODO()
fun read() = items().get("key")`, true},
		{"invisible local", `fun unrelated() { class Map }
fun read(items: Map<String, Any>) = items.get("key")`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := parseTestFile(t, "package example\n"+test.source+"\n")
			resolver := buildTestResolver(t, file)
			call := `items.get("key")`
			if test.name == "return annotation" {
				call = `items().get("key")`
			}
			idx := flatFirstOfTypeWithText(file, "call_expression", call)
			if idx == 0 {
				t.Fatal("missing receiver call")
			}
			got := resolver.ResolveFlatNode(idx, file)
			if (got.Kind == TypeUnknown) != test.unknown {
				t.Fatalf("type = %+v, want unknown = %v", got, test.unknown)
			}
		})
	}
}

func TestStdlibMethodsResolveLibraryReceivers(t *testing.T) {
	file := parseTestFile(t, "package example\n")
	resolver := buildTestResolver(t, file)
	for key, method := range StdlibMethods {
		if strings.HasPrefix(key, "_.") {
			continue
		}
		name, methodName, _ := strings.Cut(key, ".")
		t.Run(key, func(t *testing.T) {
			fqn := KotlinStdlibTypes[name]
			if primitive := PrimitiveTypes[name]; primitive != "" {
				fqn = primitive
			}
			switch name {
			case "Context":
				fqn = "android.content.Context"
			case "File":
				fqn = "java.io.File"
			case "CoroutineScope":
				fqn = "kotlinx.coroutines.CoroutineScope"
			case "ArrayList", "HashSet", "LinkedHashSet", "HashMap", "LinkedHashMap":
				fqn = "kotlin.collections." + name
			}
			if fqn == "" {
				t.Fatalf("missing library identity for %s", name)
			}
			receiver := &ResolvedType{Name: name, FQN: fqn, Kind: TypeClass, Resolved: true}
			if got := resolver.lookupReceiverStdlibMethod(receiver, methodName); got != method {
				t.Fatalf("library receiver lost stdlib signature: got %+v, want %+v", got, method)
			}
		})
	}
}

func TestContextGetSystemServiceReceiverIdentity(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		matched bool
	}{
		{"local", "class Context { fun getSystemService(name: String): Any = Any() }", false},
		{"unresolved", "", true},
		{"unrelated import", "import example.other.Context", true},
		{"android import", "import android.content.Context", true},
		{"android alias", "import android.content.Context as AndroidContext", false},
		{"android qualified", "", false},
		{"activity", "import android.app.Activity", false},
		{"local activity", "class Activity", false},
		{"source subtype", "import android.content.Context\nclass CustomContext : Context()", false},
		{"android wildcard", "import android.content.*", true},
		{"local subtype", "class Context\nclass CustomContext : Context()", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			typeName := "Context"
			switch tc.name {
			case "android alias":
				typeName = "AndroidContext"
			case "android qualified":
				typeName = "android.content.Context"
			case "activity", "local activity":
				typeName = "Activity"
			case "source subtype", "local subtype":
				typeName = "CustomContext"
			}
			file := parseTestFile(t, "package example\n"+tc.source+"\nfun service(context: "+typeName+") = context.getSystemService(\"service\")\n")
			resolver := buildTestResolver(t, file)
			idx := flatFirstOfTypeWithText(file, "call_expression", `context.getSystemService("service")`)
			if idx == 0 {
				t.Fatal("missing service call")
			}
			got := resolver.ResolveFlatNode(idx, file)
			if got.IsNullable() != tc.matched {
				t.Fatalf("nullable = %v, want %v: %+v", got.IsNullable(), tc.matched, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStdlibStringMethods
// ---------------------------------------------------------------------------

func TestStdlibStringMethods(t *testing.T) {
	cases := []struct {
		method       string
		wantName     string
		wantNullable bool
	}{
		{"toInt", "Int", false},
		{"toLong", "Long", false},
		{"toFloat", "Float", false},
		{"toDouble", "Double", false},
		{"toByte", "Byte", false},
		{"toShort", "Short", false},
		{"toIntOrNull", "Int", true},
		{"toLongOrNull", "Long", true},
		{"lowercase", "String", false},
		{"uppercase", "String", false},
		{"trim", "String", false},
		{"trimIndent", "String", false},
		{"trimMargin", "String", false},
		{"replace", "String", false},
		{"reversed", "String", false},
		{"length", "Int", false},
		{"isEmpty", "Boolean", false},
		{"isBlank", "Boolean", false},
		{"isNotEmpty", "Boolean", false},
		{"isNotBlank", "Boolean", false},
		{"startsWith", "Boolean", false},
		{"endsWith", "Boolean", false},
		{"contains", "Boolean", false},
		{"toByteArray", "ByteArray", false},
		{"split", "List", false},
		{"toCharArray", "CharArray", false},
		{"toRegex", "Regex", false},
	}

	for _, tc := range cases {
		t.Run("String."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod("String", tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for String.%s", tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("String.%s: want return type %q, got %q", tc.method, tc.wantName, m.ReturnType.Name)
			}
			if m.Nullable != tc.wantNullable {
				t.Errorf("String.%s: want nullable=%v, got %v", tc.method, tc.wantNullable, m.Nullable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStdlibCollectionMethods
// ---------------------------------------------------------------------------

func TestStdlibCollectionMethods(t *testing.T) {
	cases := []struct {
		receiver     string
		method       string
		wantName     string
		wantNullable bool
	}{
		{"List", "map", "List", false},
		{"List", "filter", "List", false},
		{"List", "filterNot", "List", false},
		{"List", "filterNotNull", "List", false},
		{"List", "flatMap", "List", false},
		{"List", "first", "Any", false},
		{"List", "firstOrNull", "Any", true},
		{"List", "lastOrNull", "Any", true},
		{"List", "find", "Any", true},
		{"List", "any", "Boolean", false},
		{"List", "all", "Boolean", false},
		{"List", "none", "Boolean", false},
		{"List", "count", "Int", false},
		{"List", "size", "Int", false},
		{"List", "isEmpty", "Boolean", false},
		{"List", "isNotEmpty", "Boolean", false},
		{"List", "toList", "List", false},
		{"List", "toMutableList", "MutableList", false},
		{"List", "toSet", "Set", false},
		{"List", "sorted", "List", false},
		{"List", "distinct", "List", false},
		{"List", "take", "List", false},
		{"List", "drop", "List", false},
		{"List", "zip", "List", false},
		{"List", "associate", "Map", false},
		{"List", "groupBy", "Map", false},
		{"List", "joinToString", "String", false},
		{"List", "forEach", "Unit", false},
		{"List", "sumOf", "Int", false},
		// Verify Collection and MutableList share the same entries
		{"Collection", "map", "List", false},
		{"MutableList", "filter", "List", false},
		{"Iterable", "any", "Boolean", false},
	}

	for _, tc := range cases {
		t.Run(tc.receiver+"."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod(tc.receiver, tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for %s.%s", tc.receiver, tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("%s.%s: want return type %q, got %q", tc.receiver, tc.method, tc.wantName, m.ReturnType.Name)
			}
			if m.Nullable != tc.wantNullable {
				t.Errorf("%s.%s: want nullable=%v, got %v", tc.receiver, tc.method, tc.wantNullable, m.Nullable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStdlibMapMethods
// ---------------------------------------------------------------------------

func TestStdlibMapMethods(t *testing.T) {
	cases := []struct {
		method       string
		wantName     string
		wantNullable bool
	}{
		{"get", "Any", true},
		{"getValue", "Any", false},
		{"keys", "Set", false},
		{"values", "Collection", false},
		{"entries", "Set", false},
		{"containsKey", "Boolean", false},
		{"containsValue", "Boolean", false},
		{"isEmpty", "Boolean", false},
		{"size", "Int", false},
	}

	for _, tc := range cases {
		t.Run("Map."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod("Map", tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for Map.%s", tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("Map.%s: want return type %q, got %q", tc.method, tc.wantName, m.ReturnType.Name)
			}
			if m.Nullable != tc.wantNullable {
				t.Errorf("Map.%s: want nullable=%v, got %v", tc.method, tc.wantNullable, m.Nullable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStdlibSequenceMethods
// ---------------------------------------------------------------------------

func TestStdlibSequenceMethods(t *testing.T) {
	cases := []struct {
		method   string
		wantName string
	}{
		{"map", "Sequence"},
		{"filter", "Sequence"},
		{"flatMap", "Sequence"},
		{"toList", "List"},
		{"toSet", "Set"},
		{"first", "Any"},
		{"count", "Int"},
		{"any", "Boolean"},
	}

	for _, tc := range cases {
		t.Run("Sequence."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod("Sequence", tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for Sequence.%s", tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("Sequence.%s: want return type %q, got %q", tc.method, tc.wantName, m.ReturnType.Name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExceptionHierarchy
// ---------------------------------------------------------------------------

func TestExceptionHierarchy(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		// Same type
		{"IOException", "IOException", true},
		// Direct parent
		{"CancellationException", "IllegalStateException", true},
		// Transitive
		{"CancellationException", "RuntimeException", true},
		{"CancellationException", "Exception", true},
		{"CancellationException", "Throwable", true},
		// Other hierarchies
		{"FileNotFoundException", "IOException", true},
		{"FileNotFoundException", "Exception", true},
		{"NumberFormatException", "IllegalArgumentException", true},
		{"ArrayIndexOutOfBoundsException", "IndexOutOfBoundsException", true},
		{"ArrayIndexOutOfBoundsException", "RuntimeException", true},
		{"ConnectException", "SocketException", true},
		{"ConnectException", "IOException", true},
		// java.net.SocketTimeoutException extends InterruptedIOException.
		{"SocketTimeoutException", "InterruptedIOException", true},
		{"SocketTimeoutException", "IOException", true},
		{"InterruptedIOException", "IOException", true},
		// Root types
		{"Throwable", "Throwable", true},
		{"Exception", "Throwable", true},
		{"RuntimeException", "Exception", true},
		{"RuntimeException", "Throwable", true},
		// Deep chain through coroutines
		{"TimeoutCancellationException", "CancellationException", true},
		{"TimeoutCancellationException", "Throwable", true},
		// Kotlin-specific
		{"KotlinNullPointerException", "NullPointerException", true},
		// Error subtypes
		{"OutOfMemoryError", "Error", true},
		{"AssertionError", "Error", true},
		{"NotImplementedError", "Error", true},
		// Android
		{"DeadObjectException", "RemoteException", true},
		{"SQLiteConstraintException", "SQLiteException", true},
		// SSL chain
		{"SSLHandshakeException", "IOException", true},
		// Not related
		{"IOException", "RuntimeException", false},
		{"NullPointerException", "IOException", false},
		{"SocketTimeoutException", "SocketException", false},
		// Reverse direction is not subtype
		{"Exception", "CancellationException", false},
		{"Throwable", "IOException", false},
		// Error is not Exception
		{"Error", "Exception", false},
	}

	for _, tc := range cases {
		t.Run(tc.a+"_subtypeOf_"+tc.b, func(t *testing.T) {
			got := IsSubtypeOfException(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("IsSubtypeOfException(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExceptionHierarchy_Unknown
// ---------------------------------------------------------------------------

func TestExceptionHierarchy_Unknown(t *testing.T) {
	// Unknown exception is not a subtype of anything (except itself)
	if IsSubtypeOfException("SomeCustomException", "Exception") {
		t.Error("expected unknown exception not to be a subtype of Exception")
	}
	if IsSubtypeOfException("SomeCustomException", "Throwable") {
		t.Error("expected unknown exception not to be a subtype of Throwable")
	}
	// Self-equality still holds
	if !IsSubtypeOfException("SomeCustomException", "SomeCustomException") {
		t.Error("expected unknown exception to be equal to itself")
	}
}

// ---------------------------------------------------------------------------
// TestIsExceptionSubtype_ViaResolver
// ---------------------------------------------------------------------------

func TestIsExceptionSubtype_ViaResolver(t *testing.T) {
	resolver := NewResolver()
	if !resolver.IsExceptionSubtype("CancellationException", "IllegalStateException") {
		t.Error("resolver should report CancellationException as subtype of IllegalStateException")
	}
	if resolver.IsExceptionSubtype("IOException", "RuntimeException") {
		t.Error("resolver should not report IOException as subtype of RuntimeException")
	}
}

// ---------------------------------------------------------------------------
// TestStdlibFlowMethods
// ---------------------------------------------------------------------------

func TestStdlibFlowMethods(t *testing.T) {
	cases := []struct {
		receiver     string
		method       string
		wantName     string
		wantNullable bool
	}{
		{"Flow", "map", "Flow", false},
		{"Flow", "filter", "Flow", false},
		{"Flow", "flatMapConcat", "Flow", false},
		{"Flow", "onEach", "Flow", false},
		{"Flow", "catch", "Flow", false},
		{"Flow", "take", "Flow", false},
		{"Flow", "distinctUntilChanged", "Flow", false},
		{"Flow", "debounce", "Flow", false},
		{"Flow", "flowOn", "Flow", false},
		{"Flow", "buffer", "Flow", false},
		{"Flow", "collect", "Unit", false},
		{"Flow", "collectLatest", "Unit", false},
		{"Flow", "first", "Any", false},
		{"Flow", "firstOrNull", "Any", true},
		{"Flow", "single", "Any", false},
		{"Flow", "singleOrNull", "Any", true},
		{"Flow", "toList", "List", false},
		{"Flow", "toSet", "Set", false},
		{"Flow", "count", "Int", false},
		{"Flow", "stateIn", "StateFlow", false},
		{"Flow", "shareIn", "SharedFlow", false},
		{"Flow", "launchIn", "Job", false},
		// StateFlow/SharedFlow inherit flow methods
		{"StateFlow", "map", "Flow", false},
		{"SharedFlow", "filter", "Flow", false},
		{"MutableStateFlow", "collect", "Unit", false},
		{"MutableSharedFlow", "toList", "List", false},
	}

	for _, tc := range cases {
		t.Run(tc.receiver+"."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod(tc.receiver, tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for %s.%s", tc.receiver, tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("%s.%s: want return type %q, got %q", tc.receiver, tc.method, tc.wantName, m.ReturnType.Name)
			}
			if m.Nullable != tc.wantNullable {
				t.Errorf("%s.%s: want nullable=%v, got %v", tc.receiver, tc.method, tc.wantNullable, m.Nullable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStdlibStateFlowSpecific
// ---------------------------------------------------------------------------

func TestStdlibStateFlowSpecific(t *testing.T) {
	// StateFlow.value
	m := LookupStdlibMethod("StateFlow", "value")
	if m == nil {
		t.Fatal("expected stdlib entry for StateFlow.value")
	}
	if m.ReturnType.Name != "Any" {
		t.Errorf("StateFlow.value: want return type Any, got %q", m.ReturnType.Name)
	}
	if m.ReturnTypeArgIndex != 0 {
		t.Errorf("StateFlow.value: want ReturnTypeArgIndex=0, got %d", m.ReturnTypeArgIndex)
	}

	// MutableStateFlow.compareAndSet
	m = LookupStdlibMethod("MutableStateFlow", "compareAndSet")
	if m == nil {
		t.Fatal("expected stdlib entry for MutableStateFlow.compareAndSet")
	}
	if m.ReturnType.Name != "Boolean" {
		t.Errorf("MutableStateFlow.compareAndSet: want Boolean, got %q", m.ReturnType.Name)
	}

	// MutableSharedFlow.emit
	m = LookupStdlibMethod("MutableSharedFlow", "emit")
	if m == nil {
		t.Fatal("expected stdlib entry for MutableSharedFlow.emit")
	}
	if m.ReturnType.Name != "Unit" {
		t.Errorf("MutableSharedFlow.emit: want Unit, got %q", m.ReturnType.Name)
	}

	// MutableSharedFlow.tryEmit
	m = LookupStdlibMethod("MutableSharedFlow", "tryEmit")
	if m == nil {
		t.Fatal("expected stdlib entry for MutableSharedFlow.tryEmit")
	}
	if m.ReturnType.Name != "Boolean" {
		t.Errorf("MutableSharedFlow.tryEmit: want Boolean, got %q", m.ReturnType.Name)
	}

	// SharedFlow.replayCache
	m = LookupStdlibMethod("SharedFlow", "replayCache")
	if m == nil {
		t.Fatal("expected stdlib entry for SharedFlow.replayCache")
	}
	if m.ReturnType.Name != "List" {
		t.Errorf("SharedFlow.replayCache: want List, got %q", m.ReturnType.Name)
	}
}

// ---------------------------------------------------------------------------
// TestStdlibSetSpecificMethods
// ---------------------------------------------------------------------------

func TestStdlibSetSpecificMethods(t *testing.T) {
	cases := []struct {
		receiver string
		method   string
		wantName string
	}{
		{"Set", "intersect", "Set"},
		{"Set", "union", "Set"},
		{"Set", "subtract", "Set"},
		{"Set", "plus", "Set"},
		{"Set", "minus", "Set"},
		{"Set", "toMutableSet", "MutableSet"},
		{"Set", "contains", "Boolean"},
		{"MutableSet", "intersect", "Set"},
		{"HashSet", "union", "Set"},
		{"LinkedHashSet", "contains", "Boolean"},
	}

	for _, tc := range cases {
		t.Run(tc.receiver+"."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod(tc.receiver, tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for %s.%s", tc.receiver, tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("%s.%s: want return type %q, got %q", tc.receiver, tc.method, tc.wantName, m.ReturnType.Name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStdlibMutableCollectionMethods
// ---------------------------------------------------------------------------

func TestStdlibMutableCollectionMethods(t *testing.T) {
	cases := []struct {
		receiver     string
		method       string
		wantName     string
		wantNullable bool
	}{
		{"MutableList", "add", "Boolean", false},
		{"MutableList", "remove", "Boolean", false},
		{"MutableList", "removeAt", "Any", false},
		{"MutableList", "set", "Any", false},
		{"MutableList", "clear", "Unit", false},
		{"MutableList", "addAll", "Boolean", false},
		{"MutableList", "removeAll", "Boolean", false},
		{"MutableList", "retainAll", "Boolean", false},
		{"MutableList", "subList", "MutableList", false},
		{"ArrayList", "add", "Boolean", false},
		{"ArrayList", "clear", "Unit", false},
		// MutableSet mutations
		{"MutableSet", "add", "Boolean", false},
		{"MutableSet", "remove", "Boolean", false},
		{"MutableSet", "clear", "Unit", false},
		{"HashSet", "addAll", "Boolean", false},
		{"LinkedHashSet", "removeAll", "Boolean", false},
		// MutableMap mutations
		{"MutableMap", "put", "Any", true},
		{"MutableMap", "remove", "Any", true},
		{"MutableMap", "clear", "Unit", false},
		{"MutableMap", "putAll", "Unit", false},
		{"MutableMap", "getOrPut", "Any", false},
		{"MutableMap", "putIfAbsent", "Any", true},
		{"HashMap", "put", "Any", true},
		{"LinkedHashMap", "clear", "Unit", false},
	}

	for _, tc := range cases {
		t.Run(tc.receiver+"."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod(tc.receiver, tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for %s.%s", tc.receiver, tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("%s.%s: want return type %q, got %q", tc.receiver, tc.method, tc.wantName, m.ReturnType.Name)
			}
			if m.Nullable != tc.wantNullable {
				t.Errorf("%s.%s: want nullable=%v, got %v", tc.receiver, tc.method, tc.wantNullable, m.Nullable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestStdlibCoroutineFunctions
// ---------------------------------------------------------------------------

func TestStdlibCoroutineFunctions(t *testing.T) {
	cases := []struct {
		method       string
		wantName     string
		wantNullable bool
	}{
		{"flowOf", "Flow", false},
		{"flow", "Flow", false},
		{"channelFlow", "Flow", false},
		{"callbackFlow", "Flow", false},
		{"emptyFlow", "Flow", false},
		{"MutableStateFlow", "MutableStateFlow", false},
		{"MutableSharedFlow", "MutableSharedFlow", false},
		{"coroutineScope", "", false},
		{"supervisorScope", "", false},
		{"yield", "Unit", false},
		{"withTimeout", "", false},
		{"withTimeoutOrNull", "", true},
	}

	for _, tc := range cases {
		t.Run("_."+tc.method, func(t *testing.T) {
			m := LookupStdlibMethod("_", tc.method)
			if m == nil {
				t.Fatalf("expected stdlib entry for _.%s", tc.method)
			}
			if m.ReturnType.Name != tc.wantName {
				t.Errorf("_.%s: want return type %q, got %q", tc.method, tc.wantName, m.ReturnType.Name)
			}
			if m.Nullable != tc.wantNullable {
				t.Errorf("_.%s: want nullable=%v, got %v", tc.method, tc.wantNullable, m.Nullable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestLookupStdlibMethod_NotFound
// ---------------------------------------------------------------------------

func TestLookupStdlibMethod_NotFound(t *testing.T) {
	if m := LookupStdlibMethod("String", "nonExistentMethod"); m != nil {
		t.Errorf("expected nil for unknown method, got %+v", m)
	}
	if m := LookupStdlibMethod("UnknownType", "toInt"); m != nil {
		t.Errorf("expected nil for unknown receiver, got %+v", m)
	}
}

// ---------------------------------------------------------------------------
// TestKnownInterfaces
// ---------------------------------------------------------------------------

func TestKnownInterfaces_Serializable(t *testing.T) {
	impls := KnownInterfaces["java.io.Serializable"]
	if len(impls) == 0 {
		t.Fatal("expected Serializable to have known implementors")
	}
	want := map[string]bool{
		"java.lang.String": true, "java.lang.Number": true,
		"java.util.ArrayList": true, "kotlin.Pair": true,
	}
	for name := range want {
		found := false
		for _, impl := range impls {
			if impl == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in Serializable implementors", name)
		}
	}
}

func TestKnownInterfaces_Closeable(t *testing.T) {
	impls := KnownInterfaces["java.io.Closeable"]
	if len(impls) == 0 {
		t.Fatal("expected Closeable to have known implementors")
	}
	want := "java.io.InputStream"
	found := false
	for _, impl := range impls {
		if impl == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in Closeable implementors", want)
	}
}

func TestKnownInterfaces_Parcelable(t *testing.T) {
	impls := KnownInterfaces["android.os.Parcelable"]
	if len(impls) == 0 {
		t.Fatal("expected Parcelable to have known implementors")
	}
}

func TestImplementsInterface(t *testing.T) {
	cases := []struct {
		typeFQN      string
		interfaceFQN string
		want         bool
	}{
		{"java.lang.String", "java.io.Serializable", true},
		{"java.util.ArrayList", "java.io.Serializable", true},
		{"kotlin.Pair", "java.io.Serializable", true},
		{"java.io.InputStream", "java.io.Closeable", true},
		{"android.os.Bundle", "android.os.Parcelable", true},
		// Not an implementor
		{"java.lang.String", "java.io.Closeable", false},
		{"java.io.InputStream", "java.io.Serializable", false},
		// Unknown interface
		{"java.lang.String", "com.example.Unknown", false},
	}

	for _, tc := range cases {
		t.Run(tc.typeFQN+"_implements_"+tc.interfaceFQN, func(t *testing.T) {
			got := ImplementsInterface(tc.typeFQN, tc.interfaceFQN)
			if got != tc.want {
				t.Errorf("ImplementsInterface(%q, %q) = %v, want %v", tc.typeFQN, tc.interfaceFQN, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestKnownClassHierarchy
// ---------------------------------------------------------------------------

func TestKnownClassHierarchy_Activity(t *testing.T) {
	supers, ok := KnownClassHierarchy["android.app.Activity"]
	if !ok {
		t.Fatal("expected android.app.Activity in KnownClassHierarchy")
	}
	if len(supers) != 2 {
		t.Fatalf("expected 2 supertypes for Activity, got %d", len(supers))
	}
	if supers[0] != "android.content.ContextWrapper" {
		t.Errorf("expected first supertype ContextWrapper, got %q", supers[0])
	}
	if supers[1] != "android.content.Context" {
		t.Errorf("expected second supertype Context, got %q", supers[1])
	}
}

func TestKnownClassHierarchy_AppCompatActivity(t *testing.T) {
	supers, ok := KnownClassHierarchy["androidx.appcompat.app.AppCompatActivity"]
	if !ok {
		t.Fatal("expected AppCompatActivity in KnownClassHierarchy")
	}
	if len(supers) < 2 {
		t.Fatalf("expected at least 2 supertypes, got %d", len(supers))
	}
}

func TestKnownClassHierarchy_Views(t *testing.T) {
	supers := KnownClassHierarchy["android.widget.Button"]
	if len(supers) != 2 {
		t.Fatalf("expected 2 supertypes for Button, got %d", len(supers))
	}
	if supers[0] != "android.widget.TextView" {
		t.Errorf("expected first supertype TextView, got %q", supers[0])
	}
	if supers[1] != "android.view.View" {
		t.Errorf("expected second supertype View, got %q", supers[1])
	}
}

// ---------------------------------------------------------------------------
// TestSimpleNameOf
// ---------------------------------------------------------------------------

func TestSimpleNameOf(t *testing.T) {
	cases := []struct {
		fqn  string
		want string
	}{
		{"android.app.Activity", "Activity"},
		{"java.lang.String", "String"},
		{"kotlin.Pair", "Pair"},
		{"NoPackage", "NoPackage"},
	}
	for _, tc := range cases {
		if got := simpleNameOf(tc.fqn); got != tc.want {
			t.Errorf("simpleNameOf(%q) = %q, want %q", tc.fqn, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestIsKnownSubtype
// ---------------------------------------------------------------------------

func TestIsKnownSubtype(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		// Self
		{"android.app.Activity", "android.app.Activity", true},
		// Direct supertype
		{"android.app.Activity", "android.content.ContextWrapper", true},
		// Transitive supertype
		{"android.app.Activity", "android.content.Context", true},
		// Deep chain
		{"androidx.fragment.app.FragmentActivity", "android.content.Context", true},
		// View hierarchy
		{"android.widget.Button", "android.view.View", true},
		{"android.widget.Button", "android.widget.TextView", true},
		// Interface implementor
		{"java.lang.String", "java.io.Serializable", true},
		{"android.os.Bundle", "android.os.Parcelable", true},
		// Not a subtype
		{"android.content.Context", "android.app.Activity", false},
		{"java.lang.String", "java.io.Closeable", false},
		// Unknown type
		{"com.example.Unknown", "android.app.Activity", false},
	}

	for _, tc := range cases {
		t.Run(tc.a+"_subtypeOf_"+tc.b, func(t *testing.T) {
			got := IsKnownSubtype(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("IsKnownSubtype(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestClassHierarchy_FallbackToKnown
// ---------------------------------------------------------------------------

func TestClassHierarchy_FallbackToKnown(t *testing.T) {
	resolver := NewResolver()

	// Should return ClassInfo from KnownClassHierarchy for a framework type
	info := resolver.ClassHierarchy("android.app.Activity")
	if info == nil {
		t.Fatal("expected ClassInfo for android.app.Activity from known hierarchy")
	}
	if info.Name != "Activity" {
		t.Errorf("expected Name=Activity, got %q", info.Name)
	}
	if info.FQN != "android.app.Activity" {
		t.Errorf("expected FQN=android.app.Activity, got %q", info.FQN)
	}
	if len(info.Supertypes) != 2 {
		t.Fatalf("expected 2 supertypes, got %d", len(info.Supertypes))
	}

	// Unknown type should still return nil
	if resolver.ClassHierarchy("com.example.Unknown") != nil {
		t.Error("expected nil for unknown type")
	}
}

// ---------------------------------------------------------------------------
// TestIsKnownValueType
// ---------------------------------------------------------------------------

func TestIsKnownValueType(t *testing.T) {
	cases := []struct {
		name string
		rt   *ResolvedType
		want bool
	}{
		{"Int", &ResolvedType{Name: "Int", FQN: "kotlin.Int", Kind: TypePrimitive}, true},
		{"String", &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive}, true},
		{"Boolean", &ResolvedType{Name: "Boolean", FQN: "kotlin.Boolean", Kind: TypePrimitive}, true},
		{"CustomClass", &ResolvedType{Name: "MyCustomClass", FQN: "com.example.MyCustomClass", Kind: TypeClass}, false},
		{"nil", nil, false},
		{"Unknown", &ResolvedType{Kind: TypeUnknown}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsKnownValueType(tc.rt)
			if got != tc.want {
				t.Errorf("IsKnownValueType(%v) = %v, want %v", tc.rt, got, tc.want)
			}
		})
	}
}

func TestStdlibSpecDecoding(t *testing.T) {
	cases := []struct {
		spec     string
		name     string
		fqn      string
		kind     TypeKind
		nullable bool
		argIdx   int
	}{
		{"pInt", "Int", "kotlin.Int", TypePrimitive, false, -1},
		{"cList", "List", "kotlin.collections.List", TypeClass, false, -1},
		{"cAny!@1", "Any", "kotlin.Any", TypeClass, true, 1},
		{"u", "Unit", "kotlin.Unit", TypeUnit, false, -1},
		{"?!", "", "", TypeUnknown, true, -1},
	}
	for _, tc := range cases {
		m := sm(tc.spec)
		rt := m.ReturnType
		if rt.Name != tc.name || rt.FQN != tc.fqn || rt.Kind != tc.kind || m.Nullable != tc.nullable || m.ReturnTypeArgIndex != tc.argIdx {
			t.Errorf("sm(%q) = %+v nullable=%v idx=%d", tc.spec, *rt, m.Nullable, m.ReturnTypeArgIndex)
		}
	}
	got := stdlibSpecs("a=pInt b=cList!")
	if len(got) != 2 || got["a"].ReturnType.Name != "Int" || !got["b"].Nullable {
		t.Errorf("stdlibSpecs decoded %v", got)
	}
}
