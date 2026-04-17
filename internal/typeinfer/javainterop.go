package typeinfer

// JavaToKotlinTypes maps Java FQNs to their Kotlin equivalents.
var JavaToKotlinTypes = map[string]string{
	// Primitives and wrappers
	"java.lang.String":    "kotlin.String",
	"java.lang.Integer":   "kotlin.Int",
	"java.lang.Long":      "kotlin.Long",
	"java.lang.Short":     "kotlin.Short",
	"java.lang.Byte":      "kotlin.Byte",
	"java.lang.Float":     "kotlin.Float",
	"java.lang.Double":    "kotlin.Double",
	"java.lang.Boolean":   "kotlin.Boolean",
	"java.lang.Character": "kotlin.Char",
	"java.lang.Object":    "kotlin.Any",
	"java.lang.Void":      "kotlin.Unit",
	"java.lang.Number":    "kotlin.Number",

	// Collections
	"java.util.List":          "kotlin.collections.List",
	"java.util.ArrayList":     "kotlin.collections.ArrayList",
	"java.util.Set":           "kotlin.collections.Set",
	"java.util.HashSet":       "kotlin.collections.HashSet",
	"java.util.LinkedHashSet": "kotlin.collections.LinkedHashSet",
	"java.util.TreeSet":       "kotlin.collections.TreeSet",
	"java.util.Map":           "kotlin.collections.Map",
	"java.util.HashMap":       "kotlin.collections.HashMap",
	"java.util.LinkedHashMap": "kotlin.collections.LinkedHashMap",
	"java.util.TreeMap":       "kotlin.collections.TreeMap",
	"java.util.Collection":    "kotlin.collections.Collection",
	"java.util.Iterator":      "kotlin.collections.Iterator",
	"java.lang.Iterable":      "kotlin.collections.Iterable",

	// Common types
	"java.lang.Comparable":       "kotlin.Comparable",
	"java.lang.Enum":             "kotlin.Enum",
	"java.lang.Annotation":       "kotlin.Annotation",
	"java.lang.Cloneable":        "kotlin.Cloneable",
	"java.lang.Throwable":        "kotlin.Throwable",
	"java.lang.Exception":        "kotlin.Exception",
	"java.lang.RuntimeException": "kotlin.RuntimeException",
	"java.lang.Error":            "kotlin.Error",

	// Functional types
	"java.lang.Runnable": "kotlin.Runnable",
}

// MapJavaToKotlin returns the Kotlin equivalent FQN for a Java FQN.
// Returns "" if no mapping exists.
func MapJavaToKotlin(javaFQN string) string {
	if kotlinFQN, ok := JavaToKotlinTypes[javaFQN]; ok {
		return kotlinFQN
	}
	return ""
}

// KnownCloseableTypes lists types known to implement Closeable/AutoCloseable.
var KnownCloseableTypes = map[string]bool{
	"InputStream":          true,
	"OutputStream":         true,
	"Reader":               true,
	"Writer":               true,
	"BufferedReader":        true,
	"BufferedWriter":        true,
	"FileInputStream":      true,
	"FileOutputStream":     true,
	"FileReader":           true,
	"FileWriter":           true,
	"InputStreamReader":    true,
	"OutputStreamWriter":   true,
	"PrintWriter":          true,
	"PrintStream":          true,
	"Socket":               true,
	"ServerSocket":         true,
	"Connection":           true,
	"Statement":            true,
	"PreparedStatement":    true,
	"ResultSet":            true,
	"Cursor":               true,
	"TypedArray":           true,
	"ParcelFileDescriptor": true,
	"HttpURLConnection":    true,
	"ZipFile":              true,
	"JarFile":              true,
	"RandomAccessFile":     true,
	"Channel":              true,
	"AssetFileDescriptor":  true,
	"Scanner":              true,
	"ByteArrayInputStream":  true,
	"ByteArrayOutputStream": true,
	"DataInputStream":       true,
	"DataOutputStream":      true,
	"ObjectInputStream":     true,
	"ObjectOutputStream":    true,
	"BufferedInputStream":   true,
	"BufferedOutputStream":  true,
}

// IsKnownCloseable returns true if the type name (simple name) is known to implement Closeable.
func IsKnownCloseable(typeName string) bool {
	return KnownCloseableTypes[typeName]
}
