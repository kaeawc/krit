package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// moshiTypeKind classifies a property type the way MoshiUsageDetector.checkMoshiType
// sees the PSI type behind it.
type moshiTypeKind int

const (
	moshiTypeUnknown    moshiTypeKind = iota // not provable from source: abstain
	moshiTypeIgnored                         // primitive, boxed, String, Any, a type parameter, a wildcard
	moshiTypeArray                           // Array<T>, IntArray, ...
	moshiTypeJSONList                        // List, Collection, Set and their Mutable forms
	moshiTypeJSONMap                         // Map, MutableMap
	moshiTypeCollection                      // another Collection implementation
	moshiTypeMap                             // another Map implementation
	moshiTypePlatform                        // another java.*, kotlin.*, android.*, ... class
	moshiTypeEnum                            // an enum declared in the project
	moshiTypeMoshiClass                      // a project class with @JsonClass or @AdaptedBy
	moshiTypeOther                           // any other class: no MoshiUsage issue ports it
)

// moshiTypeInfo is a resolved type reference.
type moshiTypeInfo struct {
	kind     moshiTypeKind
	name     string // simple class name, for messages
	final    int    // 1 final, 0 open, -1 unknown
	moshi    bool   // for enums: annotated with @JsonClass or @AdaptedBy
	variance []int  // for generic classes: per type parameter, 0 invariant, 1 out, -1 in
	args     []moshiTypeArg
}

// moshiTypeArg is one type argument as written.
type moshiTypeArg struct {
	node       uint32 // the type node, or 0 for `*`
	projection int    // 0 none, 1 out, -1 in
}

// Kotlin declaration-site variance of the JSON collection interfaces' type parameters.
var moshiKotlinCollectionVariance = map[string][]int{
	"List": {1}, "Collection": {1}, "Set": {1}, "Map": {0, 1},
	"MutableList": {0}, "MutableCollection": {0}, "MutableSet": {0}, "MutableMap": {0, 0},
}

// moshiKotlinDefaults maps the simple names Kotlin's default imports provide
// to how the detector sees them. Names missing here are unknown.
var moshiKotlinDefaults = map[string]moshiTypeInfo{
	"Int": {kind: moshiTypeIgnored, final: 1}, "Long": {kind: moshiTypeIgnored, final: 1},
	"Short": {kind: moshiTypeIgnored, final: 1}, "Byte": {kind: moshiTypeIgnored, final: 1},
	"Double": {kind: moshiTypeIgnored, final: 1}, "Float": {kind: moshiTypeIgnored, final: 1},
	"Boolean": {kind: moshiTypeIgnored, final: 1}, "Char": {kind: moshiTypeIgnored, final: 1},
	"String": {kind: moshiTypeIgnored, final: 1}, "Any": {kind: moshiTypeIgnored, final: 0},
	"Array":        {kind: moshiTypeArray, final: 1},
	"IntArray":     {kind: moshiTypeArray, final: 1},
	"LongArray":    {kind: moshiTypeArray, final: 1},
	"ShortArray":   {kind: moshiTypeArray, final: 1},
	"ByteArray":    {kind: moshiTypeArray, final: 1},
	"DoubleArray":  {kind: moshiTypeArray, final: 1},
	"FloatArray":   {kind: moshiTypeArray, final: 1},
	"BooleanArray": {kind: moshiTypeArray, final: 1},
	"CharArray":    {kind: moshiTypeArray, final: 1},
	"List":         {kind: moshiTypeJSONList, final: 0}, "MutableList": {kind: moshiTypeJSONList, final: 0},
	"Collection": {kind: moshiTypeJSONList, final: 0}, "MutableCollection": {kind: moshiTypeJSONList, final: 0},
	"Set": {kind: moshiTypeJSONList, final: 0}, "MutableSet": {kind: moshiTypeJSONList, final: 0},
	"Map": {kind: moshiTypeJSONMap, final: 0}, "MutableMap": {kind: moshiTypeJSONMap, final: 0},
	"ArrayList":               {kind: moshiTypeCollection, name: "ArrayList", final: 0},
	"HashSet":                 {kind: moshiTypeCollection, name: "HashSet", final: 0},
	"LinkedHashSet":           {kind: moshiTypeCollection, name: "LinkedHashSet", final: 0},
	"HashMap":                 {kind: moshiTypeMap, name: "HashMap", final: 0},
	"LinkedHashMap":           {kind: moshiTypeMap, name: "LinkedHashMap", final: 0},
	"Pair":                    {kind: moshiTypePlatform, name: "Pair", final: 1},
	"Triple":                  {kind: moshiTypePlatform, name: "Triple", final: 1},
	"Unit":                    {kind: moshiTypePlatform, name: "Unit", final: 1},
	"CharSequence":            {kind: moshiTypePlatform, name: "CharSequence", final: 0},
	"Number":                  {kind: moshiTypePlatform, name: "Number", final: 0},
	"Iterable":                {kind: moshiTypePlatform, name: "Iterable", final: 0},
	"MutableIterable":         {kind: moshiTypePlatform, name: "Iterable", final: 0},
	"Sequence":                {kind: moshiTypePlatform, name: "Sequence", final: 0},
	"Comparable":              {kind: moshiTypePlatform, name: "Comparable", final: 0},
	"Throwable":               {kind: moshiTypePlatform, name: "Throwable", final: 0},
	"Regex":                   {kind: moshiTypePlatform, name: "Regex", final: 1},
	"IntRange":                {kind: moshiTypePlatform, name: "IntRange", final: 0},
	"LongRange":               {kind: moshiTypePlatform, name: "LongRange", final: 0},
	"Map.Entry":               {kind: moshiTypePlatform, name: "Entry", final: 0},
	"MutableMap.MutableEntry": {kind: moshiTypePlatform, name: "Entry", final: 0},
}

// moshiJavaUtilKinds classifies java.util / java.util.concurrent and Android
// collection classes by simple name. Other classes in those packages are
// unknown, since they may implement Collection or Map.
var moshiJavaUtilKinds = map[string]moshiTypeInfo{
	"List": {kind: moshiTypeJSONList}, "Collection": {kind: moshiTypeJSONList}, "Set": {kind: moshiTypeJSONList},
	"Map":        {kind: moshiTypeJSONMap},
	"ArrayList":  {kind: moshiTypeCollection, final: 0},
	"LinkedList": {kind: moshiTypeCollection, final: 0},
	"HashSet":    {kind: moshiTypeCollection, final: 0}, "LinkedHashSet": {kind: moshiTypeCollection, final: 0},
	"TreeSet": {kind: moshiTypeCollection, final: 0}, "ArrayDeque": {kind: moshiTypeCollection, final: 0},
	"Vector": {kind: moshiTypeCollection, final: 0}, "Stack": {kind: moshiTypeCollection, final: 0},
	"PriorityQueue": {kind: moshiTypeCollection, final: 0}, "Queue": {kind: moshiTypeCollection, final: 0},
	"Deque": {kind: moshiTypeCollection, final: 0}, "SortedSet": {kind: moshiTypeCollection, final: 0},
	"NavigableSet": {kind: moshiTypeCollection, final: 0}, "EnumSet": {kind: moshiTypeCollection, final: 0},
	"CopyOnWriteArrayList":  {kind: moshiTypeCollection, final: 0},
	"CopyOnWriteArraySet":   {kind: moshiTypeCollection, final: 0},
	"ConcurrentLinkedQueue": {kind: moshiTypeCollection, final: 0},
	"HashMap":               {kind: moshiTypeMap, final: 0}, "LinkedHashMap": {kind: moshiTypeMap, final: 0},
	"TreeMap": {kind: moshiTypeMap, final: 0}, "Hashtable": {kind: moshiTypeMap, final: 0},
	"WeakHashMap": {kind: moshiTypeMap, final: 0}, "IdentityHashMap": {kind: moshiTypeMap, final: 0},
	"EnumMap": {kind: moshiTypeMap, final: 0}, "SortedMap": {kind: moshiTypeMap, final: 0},
	"NavigableMap": {kind: moshiTypeMap, final: 0}, "Properties": {kind: moshiTypeMap, final: 0},
	"ConcurrentHashMap": {kind: moshiTypeMap, final: 0}, "ConcurrentMap": {kind: moshiTypeMap, final: 0},
	"ConcurrentSkipListMap": {kind: moshiTypeMap, final: 0},
	"ArrayMap":              {kind: moshiTypeMap, final: 1}, "ArraySet": {kind: moshiTypeCollection, final: 1},
	"Date": {kind: moshiTypePlatform, final: 0}, "UUID": {kind: moshiTypePlatform, final: 1},
	"Locale": {kind: moshiTypePlatform, final: 1}, "Currency": {kind: moshiTypePlatform, final: 1},
	"Calendar": {kind: moshiTypePlatform, final: 0}, "TimeZone": {kind: moshiTypePlatform, final: 0},
	"Optional": {kind: moshiTypePlatform, final: 1}, "BitSet": {kind: moshiTypePlatform, final: 0},
	"SparseArray": {kind: moshiTypePlatform, final: 0},
}

// moshiCollectionPackages hold collection implementations, so their classes
// need an entry in moshiJavaUtilKinds to be classified.
var moshiCollectionPackages = map[string]bool{
	"java.util": true, "java.util.concurrent": true, "android.util": true, "androidx.collection": true,
}

// moshiKnownFinalPlatform lists platform classes known to be final, which
// Kotlin passes to Java without a wildcard in covariant positions.
var moshiKnownFinalPlatform = map[string]bool{
	"java.time.Instant": true, "java.time.LocalDate": true, "java.time.LocalDateTime": true, "java.time.LocalTime": true,
	"java.time.OffsetDateTime": true, "java.time.ZonedDateTime": true, "java.time.Duration": true, "java.time.Period": true,
	"java.time.Year": true, "java.time.YearMonth": true, "java.time.ZoneOffset": true, "java.time.OffsetTime": true,
	"java.net.URI": true, "java.net.URL": true, "java.lang.Class": true,
}

// moshiPlatformPackages are the packages slack-lints' isPlatformType treats as platform.
var moshiPlatformPackages = map[string]bool{
	"android": true, "androidx": true, "java": true, "javax": true, "kotlin": true, "kotlinx": true, "scala": true,
}

// classifyFQN classifies a class outside the project by FQN.
func moshiClassifyExternalFQN(fqn string) moshiTypeInfo {
	switch fqn {
	case "java.lang.String", "java.lang.Integer", "java.lang.Long", "java.lang.Short", "java.lang.Byte",
		"java.lang.Double", "java.lang.Float", "java.lang.Boolean", "java.lang.Character":
		return moshiTypeInfo{kind: moshiTypeIgnored, final: 1}
	case "java.lang.Object", "kotlin.Any":
		return moshiTypeInfo{kind: moshiTypeIgnored, final: 0}
	}
	if strings.HasPrefix(fqn, "kotlin.") {
		simple := strings.TrimPrefix(fqn, "kotlin.")
		simple = strings.TrimPrefix(simple, "collections.")
		simple = strings.TrimPrefix(simple, "sequences.")
		simple = strings.TrimPrefix(simple, "ranges.")
		simple = strings.TrimPrefix(simple, "text.")
		if info, ok := moshiKotlinDefaults[simple]; ok {
			return info
		}
	}
	dot := strings.LastIndex(fqn, ".")
	if dot < 0 {
		return moshiTypeInfo{kind: moshiTypeUnknown}
	}
	pkg, simple := fqn[:dot], fqn[dot+1:]
	if moshiCollectionPackages[pkg] {
		info, ok := moshiJavaUtilKinds[simple]
		if !ok {
			return moshiTypeInfo{kind: moshiTypeUnknown}
		}
		info.name = simple
		return info
	}
	first, _, _ := strings.Cut(fqn, ".")
	if moshiPlatformPackages[first] {
		final := -1
		if moshiKnownFinalPlatform[fqn] {
			final = 1
		}
		return moshiTypeInfo{kind: moshiTypePlatform, name: simple, final: final}
	}
	return moshiTypeInfo{kind: moshiTypeOther, final: -1}
}

// moshiTypeScope is the context a property type resolves in.
type moshiTypeScope struct {
	a          *moshiAnalyzer
	file       *scanner.File
	pkg        string
	class      uint32          // the class whose constructor declares the property
	typeParams map[string]bool // the class's type parameter names
	// wildcardsOverridden is set when @JvmSuppressWildcards or @JvmWildcard
	// may change which type arguments Kotlin passes as wildcards.
	wildcardsOverridden bool
}

// moshiMentionsWildcardControl reports whether text names an annotation that
// changes Kotlin's wildcard generation.
func moshiMentionsWildcardControl(text string) bool {
	return strings.Contains(text, "JvmSuppressWildcards") || strings.Contains(text, "JvmWildcard")
}

// moshiTypeArgsOf returns the type arguments written on a user_type.
func moshiTypeArgsOf(file *scanner.File, userType uint32) []moshiTypeArg {
	var out []moshiTypeArg
	args := uint32(0)
	file.FlatForEachChild(userType, func(c uint32) {
		if file.FlatType(c) == "type_arguments" {
			args = c
		}
	})
	if args == 0 {
		return nil
	}
	file.FlatForEachChild(args, func(p uint32) {
		if file.FlatType(p) != "type_projection" {
			return
		}
		arg := moshiTypeArg{}
		file.FlatForEachChild(p, func(c uint32) {
			switch file.FlatType(c) {
			case "type_projection_modifiers":
				if strings.Contains(file.FlatNodeText(c), "out") {
					arg.projection = 1
				} else if strings.Contains(file.FlatNodeText(c), "in") {
					arg.projection = -1
				}
			case "*":
			default:
				if file.FlatIsNamed(c) && arg.node == 0 {
					arg.node = c
				}
			}
		})
		out = append(out, arg)
	})
	return out
}

// resolve classifies the type at node.
func (s moshiTypeScope) resolve(node uint32) moshiTypeInfo {
	file := s.file
	switch file.FlatType(node) {
	case "nullable_type", "parenthesized_type":
		if inner := file.FlatNamedChild(node, 0); inner != 0 {
			return s.resolve(inner)
		}
		return moshiTypeInfo{kind: moshiTypeUnknown}
	case "function_type":
		if file.FlatHasChildOfType(node, "suspend") || strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(node)), "suspend") {
			return moshiTypeInfo{kind: moshiTypeUnknown}
		}
		// FunctionN counts the parameters plus a receiver, which precedes them.
		arity, seenParams := 0, false
		file.FlatForEachChild(node, func(c uint32) {
			switch {
			case file.FlatType(c) == "function_type_parameters":
				seenParams = true
				file.FlatForEachNamedChild(c, func(uint32) { arity++ })
			case file.FlatIsNamed(c) && !seenParams:
				arity++
			}
		})
		name := fmt.Sprintf("Function%d", arity)
		if arity > 22 {
			// Kotlin compiles larger function types to kotlin.jvm.functions.FunctionN.
			name = "FunctionN"
		}
		return moshiTypeInfo{kind: moshiTypePlatform, name: name, final: 0}
	case "user_type":
	default:
		return moshiTypeInfo{kind: moshiTypeUnknown}
	}

	var parts []string
	file.FlatForEachChild(node, func(c uint32) {
		if file.FlatType(c) == "type_identifier" {
			parts = append(parts, file.FlatNodeText(c))
		}
	})
	name := strings.Join(parts, ".")
	args := moshiTypeArgsOf(file, node)
	if len(parts) == 1 && s.typeParams[name] {
		return moshiTypeInfo{kind: moshiTypeIgnored, final: 0}
	}

	info := s.resolveName(name)
	info.args = args
	return info
}

func (s moshiTypeScope) resolveName(name string) moshiTypeInfo {
	file := s.file
	head, rest, _ := strings.Cut(name, ".")
	imports := lintImportsOf(file)
	idx := s.a.projectClasses()
	if imported, ok := imports.bySimple[head]; ok {
		fqn := imported
		if rest != "" {
			fqn += "." + rest
		}
		if c := idx.byFQN[fqn]; c != nil {
			return s.classifyProject(c)
		}
		return moshiClassifyExternalFQN(fqn)
	}
	if fqn := idx.resolve(file, s.pkg, s.class, name); fqn != "" {
		return s.classifyProject(idx.byFQN[fqn])
	}
	if info, ok := moshiKotlinDefaults[name]; ok {
		if info.name == "" {
			info.name = name
		}
		if v, ok := moshiKotlinCollectionVariance[name]; ok {
			info.variance = v
		}
		return info
	}
	if strings.Contains(name, ".") {
		// A fully qualified reference to a class outside the project.
		return moshiClassifyExternalFQN(name)
	}
	return moshiTypeInfo{kind: moshiTypeUnknown}
}

// classifyProject classifies a class declared in the project.
func (s moshiTypeScope) classifyProject(c *moshiDeclaredClass) moshiTypeInfo {
	file := c.file
	decl, ok := moshiClassDecl(file, c.node)
	if !ok {
		return moshiTypeInfo{kind: moshiTypeUnknown}
	}
	final := 1
	if file.Language == scanner.LangJava {
		if !decl.isEnum && file.FlatType(c.node) != "record_declaration" && !file.FlatHasModifier(c.node, "final") {
			final = 0
		}
	} else if decl.isInterface || moshiHasModifier(decl, "open") || moshiHasModifier(decl, "abstract") ||
		moshiHasModifier(decl, "sealed") || moshiHasModifier(decl, "annotation") {
		final = 0
	}
	name := file.FlatNodeText(decl.nameNode)
	moshi := moshiFindAnnotation(file, c.anns, fqcnMoshiJSONClass) != nil || moshiFindAnnotation(file, c.anns, fqcnMoshixAdaptedBy) != nil
	if decl.isEnum {
		// Kotlin keeps a wildcard for enum type arguments (entries may be
		// subclasses), so List<SomeEnum> is not checked. Java enums are unverified.
		enumFinal := 0
		if file.Language == scanner.LangJava {
			enumFinal = -1
		}
		return moshiTypeInfo{kind: moshiTypeEnum, name: name, final: enumFinal, moshi: moshi}
	}
	switch kind, known := s.inheritedCollectionKind(c, map[*moshiDeclaredClass]bool{}); {
	case !known:
		return moshiTypeInfo{kind: moshiTypeUnknown}
	case kind != moshiTypeOther:
		return moshiTypeInfo{kind: kind, name: name, final: final}
	}
	if moshi {
		return moshiTypeInfo{kind: moshiTypeMoshiClass, name: name, final: final, variance: moshiDeclaredVariance(file, c.node)}
	}
	return moshiTypeInfo{kind: moshiTypeOther, name: name, final: final}
}

// inheritedCollectionKind reports whether a project class implements
// Collection (moshiTypeCollection) or Map (moshiTypeMap) through its supertypes.
func (s moshiTypeScope) inheritedCollectionKind(c *moshiDeclaredClass, seen map[*moshiDeclaredClass]bool) (moshiTypeKind, bool) {
	if seen[c] {
		return moshiTypeOther, true
	}
	seen[c] = true
	known := true
	sub := moshiTypeScope{a: s.a, file: c.file, pkg: c.pkg, class: c.node}
	for _, super := range c.supertypes {
		if moshiTrivialSupertype(super) {
			continue
		}
		var info moshiTypeInfo
		if fqn := s.a.projectClasses().resolve(c.file, c.pkg, c.node, super); fqn != "" {
			kind, ok := s.inheritedCollectionKind(s.a.projectClasses().byFQN[fqn], seen)
			if !ok {
				known = false
				continue
			}
			info.kind = kind
		} else {
			info = sub.resolveName(super)
		}
		switch info.kind {
		case moshiTypeJSONList, moshiTypeCollection:
			return moshiTypeCollection, true
		case moshiTypeJSONMap, moshiTypeMap:
			return moshiTypeMap, true
		case moshiTypeUnknown:
			known = false
		}
	}
	return moshiTypeOther, known
}

// moshiDeclaredVariance returns the declared variance of a Kotlin class's
// type parameters (Java type parameters are invariant).
func moshiDeclaredVariance(file *scanner.File, node uint32) []int {
	params, ok := file.FlatFindChild(node, "type_parameters")
	if !ok {
		return nil
	}
	var out []int
	file.FlatForEachChild(params, func(p uint32) {
		if file.FlatType(p) != "type_parameter" {
			return
		}
		v := 0
		if mods, ok := file.FlatFindChild(p, "type_parameter_modifiers"); ok {
			text := file.FlatNodeText(mods)
			if strings.Contains(text, "out") {
				v = 1
			} else if strings.Contains(text, "in") {
				v = -1
			}
		}
		out = append(out, v)
	})
	return out
}

// moshiTypeArgChecked reports whether the detector sees type argument i of a
// generic type as a class type (not a wildcard). Kotlin emits a wildcard for
// a use-site projection, for an `in` parameter, and for an `out` parameter
// unless the argument type is final.
func (s moshiTypeScope) moshiTypeArgChecked(owner moshiTypeInfo, i int, arg moshiTypeArg, argInfo moshiTypeInfo) (checked, known bool) {
	if arg.node == 0 || arg.projection != 0 {
		return false, true
	}
	variance := 0
	if i < len(owner.variance) {
		variance = owner.variance[i]
	}
	switch variance {
	case -1:
		return false, true
	case 1:
		if s.wildcardsOverridden {
			return false, false
		}
		switch argInfo.final {
		case 1:
			return true, true
		case 0:
			return false, true
		}
		return false, false
	}
	return true, true
}

// checkPropertyType ports MoshiUsageDetector.checkMoshiType. typeNode is the
// property's declared type, where every finding is reported, and node the
// type being checked (the property's type or one of its type arguments).
func (s moshiTypeScope) checkPropertyType(typeNode, node uint32, defaultValue uint32, nested bool) {
	a, file := s.a, s.file
	info := s.resolve(node)
	switch info.kind {
	case moshiTypeUnknown, moshiTypeIgnored, moshiTypeOther:
	case moshiTypeArray:
		a.reportIssue("MoshiUsageArray", file, typeNode)
	case moshiTypeEnum:
		if !info.moshi {
			a.reportIssue("MoshiUsageEnumPropertyCouldBeMoshi", file, typeNode)
		} else if !nested && defaultValue != 0 && moshiIsReferenceExpression(file, defaultValue) &&
			strings.Contains(file.FlatNodeText(defaultValue), "UNKNOWN") {
			a.reportIssue("MoshiUsageEnumPropertyDefaultUnknown", file, defaultValue)
		}
	case moshiTypeJSONList, moshiTypeJSONMap:
		// Lint checks mutability on the property's type as written, for every
		// level of nesting.
		source := strings.TrimSpace(file.FlatNodeText(typeNode))
		for _, prefix := range []string{"MutableCollection", "MutableList", "MutableSet", "MutableMap"} {
			if strings.HasPrefix(source, prefix) {
				a.reportIssue("MoshiUsageMutableCollections", file, typeNode)
				break
			}
		}
		s.checkTypeArgs(typeNode, info)
	case moshiTypeCollection:
		a.report("MoshiUsageNonMoshiClassCollection", file, typeNode,
			strings.ReplaceAll(moshiIssueMessage("MoshiUsageNonMoshiClassCollection"), "%HINT%", info.name))
	case moshiTypeMap:
		a.report("MoshiUsageNonMoshiClassMap", file, typeNode,
			strings.ReplaceAll(moshiIssueMessage("MoshiUsageNonMoshiClassMap"), "%HINT%", info.name))
	case moshiTypePlatform:
		a.report("MoshiUsageNonMoshiClassPlatform", file, typeNode,
			strings.ReplaceAll(moshiIssueMessage("MoshiUsageNonMoshiClassPlatform"), "%HINT%", info.name))
	case moshiTypeMoshiClass:
		s.checkTypeArgs(typeNode, info)
	}
}

func (s moshiTypeScope) checkTypeArgs(typeNode uint32, owner moshiTypeInfo) {
	for i, arg := range owner.args {
		if arg.node == 0 {
			continue
		}
		argInfo := s.resolve(arg.node)
		checked, known := s.moshiTypeArgChecked(owner, i, arg, argInfo)
		if checked && known {
			s.checkPropertyType(typeNode, arg.node, 0, true)
		}
	}
}

// moshiIsReferenceExpression reports whether a default value is a simple or
// qualified reference (KtReferenceExpression / KtQualifiedExpression).
func moshiIsReferenceExpression(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "simple_identifier", "navigation_expression":
		return !strings.Contains(file.FlatNodeText(node), "(")
	}
	return false
}

// moshiTypeParamNames returns the names of a Kotlin class's type parameters.
func moshiTypeParamNames(file *scanner.File, node uint32) map[string]bool {
	out := map[string]bool{}
	params, ok := file.FlatFindChild(node, "type_parameters")
	if !ok {
		return out
	}
	file.FlatForEachChild(params, func(p uint32) {
		if file.FlatType(p) == "type_parameter" {
			if id, ok := file.FlatFindChild(p, "type_identifier"); ok {
				out[file.FlatNodeText(id)] = true
			}
		}
	})
	return out
}

// moshiKnownNonQualifiers are annotations a Moshi property commonly carries
// that are never JsonQualifiers.
var moshiKnownNonQualifiers = []string{
	fqcnMoshiJSON, fqcnKotlinJvmTransient, fqcnGsonSerializedName, fqcnMoshixAdaptedBy,
	"kotlin.Deprecated", "kotlin.Suppress", "kotlin.jvm.JvmField", "kotlin.jvm.JvmStatic",
	"kotlin.jvm.JvmSuppressWildcards", "kotlin.jvm.JvmWildcard", "androidx.annotation.Keep",
}

// moshiPropertyHasQualifier reports whether a property may carry a
// JsonQualifier: an annotation that is a project @JsonQualifier, or one that
// is neither a known non-qualifier nor a project annotation class.
func (s moshiTypeScope) moshiPropertyHasQualifier(anns []moshiAnnotation) bool {
	idx := s.a.projectClasses()
	for _, ann := range anns {
		known := false
		for _, fqn := range moshiKnownNonQualifiers {
			if moshiRefIsKotlinDefault(s.file, ann.name, fqn) {
				known = true
				break
			}
		}
		if known {
			continue
		}
		fqn := ""
		head, _, _ := strings.Cut(ann.name, ".")
		if imported, ok := lintImportsOf(s.file).bySimple[head]; ok && head == ann.name {
			fqn = imported
		} else {
			fqn = idx.resolve(s.file, s.pkg, s.class, ann.name)
		}
		c := idx.byFQN[fqn]
		if c == nil {
			return true
		}
		if moshiFindAnnotation(c.file, c.anns, fqcnMoshiJSONQualifier) != nil {
			return true
		}
	}
	return false
}

// moshiFileAnnotationsText returns the text of the file's @file: annotations.
func moshiFileAnnotationsText(file *scanner.File) string {
	var b strings.Builder
	file.FlatForEachChild(0, func(c uint32) {
		if file.FlatType(c) == "file_annotation" {
			b.WriteString(file.FlatNodeText(c))
		}
	})
	return b.String()
}
