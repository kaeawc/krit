package typeinfer

import "testing"

// ---------------------------------------------------------------------------
// TestMapJavaToKotlin_Primitives
// ---------------------------------------------------------------------------

func TestMapJavaToKotlin_Primitives(t *testing.T) {
	cases := []struct {
		javaFQN  string
		wantKotlin string
	}{
		{"java.lang.String", "kotlin.String"},
		{"java.lang.Integer", "kotlin.Int"},
		{"java.lang.Long", "kotlin.Long"},
		{"java.lang.Short", "kotlin.Short"},
		{"java.lang.Byte", "kotlin.Byte"},
		{"java.lang.Float", "kotlin.Float"},
		{"java.lang.Double", "kotlin.Double"},
		{"java.lang.Boolean", "kotlin.Boolean"},
		{"java.lang.Character", "kotlin.Char"},
		{"java.lang.Object", "kotlin.Any"},
		{"java.lang.Void", "kotlin.Unit"},
		{"java.lang.Number", "kotlin.Number"},
	}
	for _, tc := range cases {
		t.Run(tc.javaFQN, func(t *testing.T) {
			got := MapJavaToKotlin(tc.javaFQN)
			if got != tc.wantKotlin {
				t.Errorf("MapJavaToKotlin(%q) = %q, want %q", tc.javaFQN, got, tc.wantKotlin)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMapJavaToKotlin_Collections
// ---------------------------------------------------------------------------

func TestMapJavaToKotlin_Collections(t *testing.T) {
	cases := []struct {
		javaFQN    string
		wantKotlin string
	}{
		{"java.util.List", "kotlin.collections.List"},
		{"java.util.ArrayList", "kotlin.collections.ArrayList"},
		{"java.util.Set", "kotlin.collections.Set"},
		{"java.util.HashSet", "kotlin.collections.HashSet"},
		{"java.util.LinkedHashSet", "kotlin.collections.LinkedHashSet"},
		{"java.util.TreeSet", "kotlin.collections.TreeSet"},
		{"java.util.Map", "kotlin.collections.Map"},
		{"java.util.HashMap", "kotlin.collections.HashMap"},
		{"java.util.LinkedHashMap", "kotlin.collections.LinkedHashMap"},
		{"java.util.TreeMap", "kotlin.collections.TreeMap"},
		{"java.util.Collection", "kotlin.collections.Collection"},
		{"java.util.Iterator", "kotlin.collections.Iterator"},
		{"java.lang.Iterable", "kotlin.collections.Iterable"},
	}
	for _, tc := range cases {
		t.Run(tc.javaFQN, func(t *testing.T) {
			got := MapJavaToKotlin(tc.javaFQN)
			if got != tc.wantKotlin {
				t.Errorf("MapJavaToKotlin(%q) = %q, want %q", tc.javaFQN, got, tc.wantKotlin)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMapJavaToKotlin_CommonTypes
// ---------------------------------------------------------------------------

func TestMapJavaToKotlin_CommonTypes(t *testing.T) {
	cases := []struct {
		javaFQN    string
		wantKotlin string
	}{
		{"java.lang.Comparable", "kotlin.Comparable"},
		{"java.lang.Enum", "kotlin.Enum"},
		{"java.lang.Annotation", "kotlin.Annotation"},
		{"java.lang.Cloneable", "kotlin.Cloneable"},
		{"java.lang.Throwable", "kotlin.Throwable"},
		{"java.lang.Exception", "kotlin.Exception"},
		{"java.lang.RuntimeException", "kotlin.RuntimeException"},
		{"java.lang.Error", "kotlin.Error"},
	}
	for _, tc := range cases {
		t.Run(tc.javaFQN, func(t *testing.T) {
			got := MapJavaToKotlin(tc.javaFQN)
			if got != tc.wantKotlin {
				t.Errorf("MapJavaToKotlin(%q) = %q, want %q", tc.javaFQN, got, tc.wantKotlin)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMapJavaToKotlin_FunctionalTypes
// ---------------------------------------------------------------------------

func TestMapJavaToKotlin_FunctionalTypes(t *testing.T) {
	got := MapJavaToKotlin("java.lang.Runnable")
	if got != "kotlin.Runnable" {
		t.Errorf("MapJavaToKotlin(java.lang.Runnable) = %q, want kotlin.Runnable", got)
	}
}

// ---------------------------------------------------------------------------
// TestMapJavaToKotlin_UnknownTypes
// ---------------------------------------------------------------------------

func TestMapJavaToKotlin_UnknownTypes(t *testing.T) {
	unknowns := []string{
		"com.example.MyClass",
		"java.util.concurrent.ConcurrentHashMap",
		"android.content.Context",
		"",
		"kotlin.String",
	}
	for _, fqn := range unknowns {
		t.Run(fqn, func(t *testing.T) {
			got := MapJavaToKotlin(fqn)
			if got != "" {
				t.Errorf("MapJavaToKotlin(%q) = %q, want empty string", fqn, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsKnownCloseable_Positive
// ---------------------------------------------------------------------------

func TestIsKnownCloseable_Positive(t *testing.T) {
	positives := []string{
		"InputStream", "OutputStream", "Reader", "Writer",
		"BufferedReader", "BufferedWriter",
		"FileInputStream", "FileOutputStream",
		"FileReader", "FileWriter",
		"InputStreamReader", "OutputStreamWriter",
		"PrintWriter", "PrintStream",
		"Socket", "ServerSocket",
		"Connection", "Statement", "PreparedStatement", "ResultSet",
		"Cursor", "TypedArray", "ParcelFileDescriptor",
		"HttpURLConnection", "ZipFile", "JarFile", "RandomAccessFile",
		"Channel", "AssetFileDescriptor", "Scanner",
		"ByteArrayInputStream", "ByteArrayOutputStream",
		"DataInputStream", "DataOutputStream",
		"ObjectInputStream", "ObjectOutputStream",
		"BufferedInputStream", "BufferedOutputStream",
	}
	for _, name := range positives {
		t.Run(name, func(t *testing.T) {
			if !IsKnownCloseable(name) {
				t.Errorf("IsKnownCloseable(%q) = false, want true", name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsKnownCloseable_Negative
// ---------------------------------------------------------------------------

func TestIsKnownCloseable_Negative(t *testing.T) {
	negatives := []string{
		"String", "Int", "List", "Map", "Boolean",
		"Activity", "Fragment", "ViewModel",
		"", "MyCustomClass",
	}
	for _, name := range negatives {
		t.Run(name, func(t *testing.T) {
			if IsKnownCloseable(name) {
				t.Errorf("IsKnownCloseable(%q) = true, want false", name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestJavaToKotlinRoundTrip_StdlibMethodLookup
// ---------------------------------------------------------------------------

func TestJavaToKotlinRoundTrip_StdlibMethodLookup(t *testing.T) {
	// Map java.util.List -> kotlin.collections.List, then verify List.size returns Int
	kotlinFQN := MapJavaToKotlin("java.util.List")
	if kotlinFQN != "kotlin.collections.List" {
		t.Fatalf("MapJavaToKotlin(java.util.List) = %q, want kotlin.collections.List", kotlinFQN)
	}

	m := LookupStdlibMethod("List", "size")
	if m == nil {
		t.Fatal("expected stdlib entry for List.size")
	}
	if m.ReturnType.Name != "Int" {
		t.Errorf("List.size return type = %q, want Int", m.ReturnType.Name)
	}

	// Map java.lang.String -> kotlin.String, then verify String.length returns Int
	kotlinFQN = MapJavaToKotlin("java.lang.String")
	if kotlinFQN != "kotlin.String" {
		t.Fatalf("MapJavaToKotlin(java.lang.String) = %q, want kotlin.String", kotlinFQN)
	}

	m = LookupStdlibMethod("String", "length")
	if m == nil {
		t.Fatal("expected stdlib entry for String.length")
	}
	if m.ReturnType.Name != "Int" {
		t.Errorf("String.length return type = %q, want Int", m.ReturnType.Name)
	}
}

// ---------------------------------------------------------------------------
// TestImportTableResolve_JavaMapping
// ---------------------------------------------------------------------------

func TestImportTableResolve_JavaMapping(t *testing.T) {
	it := &ImportTable{
		Explicit: map[string]string{
			"ArrayList": "java.util.ArrayList",
			"String":    "java.lang.String",
		},
		Aliases:  map[string]string{
			"JList": "java.util.List",
		},
		Wildcard: nil,
	}

	// Explicit import of java.util.ArrayList should resolve to kotlin.collections.ArrayList
	got := it.Resolve("ArrayList")
	if got != "kotlin.collections.ArrayList" {
		t.Errorf("Resolve(ArrayList) = %q, want kotlin.collections.ArrayList", got)
	}

	// Explicit import of java.lang.String should resolve to kotlin.String
	got = it.Resolve("String")
	if got != "kotlin.String" {
		t.Errorf("Resolve(String) = %q, want kotlin.String", got)
	}

	// Alias of java.util.List should resolve to kotlin.collections.List
	got = it.Resolve("JList")
	if got != "kotlin.collections.List" {
		t.Errorf("Resolve(JList) = %q, want kotlin.collections.List", got)
	}
}
