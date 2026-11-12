package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/strutil"
)

// StdlibMethod describes a known stdlib method's return type.
type StdlibMethod struct {
	ReturnType         *ResolvedType
	Nullable           bool
	ReturnTypeArgIndex int // -1 = use fixed ReturnType; 0+ = use receiver's TypeArgs[i]
}

// stdlibType is a shorthand to build a ResolvedType for the stdlib table.
func stdlibType(name string, kind TypeKind) *ResolvedType {
	fqn := ""
	if f, ok := PrimitiveTypes[name]; ok {
		fqn = f
	} else if f, ok := KotlinStdlibTypes[name]; ok {
		fqn = f
	}
	return &ResolvedType{Name: name, FQN: fqn, Kind: kind, Resolved: true}
}

func stdlibPrimitive(name string) *ResolvedType {
	return stdlibType(name, TypePrimitive)
}

func stdlibClass(name string) *ResolvedType {
	return stdlibType(name, TypeClass)
}

func stdlibUnit() *ResolvedType {
	return &ResolvedType{Name: "Unit", FQN: "kotlin.Unit", Kind: TypeUnit, Resolved: true}
}

// sm decodes one compact method spec: a kind letter (p primitive, c class,
// u Unit, ? unknown), the type name for p and c, then "!" for a nullable
// result and "@N" for the receiver type-argument index.
func sm(spec string) *StdlibMethod {
	m := &StdlibMethod{ReturnTypeArgIndex: -1}
	if i := strings.IndexByte(spec, '@'); i >= 0 {
		m.ReturnTypeArgIndex = int(spec[i+1] - '0')
		spec = spec[:i]
	}
	if strings.HasSuffix(spec, "!") {
		m.Nullable = true
		spec = strings.TrimSuffix(spec, "!")
	}
	switch spec[0] {
	case 'p':
		m.ReturnType = stdlibPrimitive(spec[1:])
	case 'c':
		m.ReturnType = stdlibClass(spec[1:])
	case 'u':
		m.ReturnType = stdlibUnit()
	default:
		m.ReturnType = &ResolvedType{Kind: TypeUnknown}
	}
	return m
}

// stdlibSpecs decodes space-separated "method=spec" pairs (see sm).
func stdlibSpecs(pairs string) map[string]*StdlibMethod {
	out := make(map[string]*StdlibMethod)
	for _, pair := range strings.Fields(pairs) {
		name, spec, _ := strings.Cut(pair, "=")
		out[name] = sm(spec)
	}
	return out
}

// StdlibMethods maps "ReceiverType.methodName" -> return type info.
// The receiver type uses simple names (String, List, Map, etc.).
// For collection types, List/Collection/Iterable/MutableList share entries.
var StdlibMethods map[string]*StdlibMethod

func init() {
	StdlibMethods = make(map[string]*StdlibMethod)

	// -------------------------------------------------------------------------
	// kotlin.String methods
	// -------------------------------------------------------------------------
	stringMethods := stdlibSpecs("toInt=pInt toLong=pLong toFloat=pFloat toDouble=pDouble toByte=pByte toShort=pShort toIntOrNull=pInt! toLongOrNull=pLong! lowercase=pString uppercase=pString trim=pString trimIndent=pString trimMargin=pString replace=pString reversed=pString length=pInt isEmpty=pBoolean isBlank=pBoolean isNotEmpty=pBoolean isNotBlank=pBoolean startsWith=pBoolean endsWith=pBoolean contains=pBoolean toByteArray=cByteArray split=cList toCharArray=cCharArray toRegex=cRegex")
	for method, info := range stringMethods {
		StdlibMethods["String."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.collections.List / Collection / Iterable methods
	// -------------------------------------------------------------------------
	collectionMethods := stdlibSpecs("map=cList filter=cList filterNot=cList filterNotNull=cList flatMap=cList first=cAny@0 last=cAny@0 single=cAny@0 firstOrNull=cAny!@0 lastOrNull=cAny!@0 singleOrNull=cAny!@0 find=cAny!@0 findLast=cAny!@0 any=pBoolean all=pBoolean none=pBoolean count=pInt size=pInt isEmpty=pBoolean isNotEmpty=pBoolean toList=cList toMutableList=cMutableList toSet=cSet sorted=cList sortedBy=cList sortedWith=cList sortedDescending=cList sortedByDescending=cList reversed=cList distinct=cList distinctBy=cList take=cList drop=cList takeWhile=cList dropWhile=cList takeLast=cList dropLast=cList zip=cList associate=cMap groupBy=cMap joinToString=pString forEach=u sumOf=pInt")
	for _, receiver := range []string{"List", "Collection", "Iterable", "MutableList", "ArrayList", "Set", "MutableSet", "HashSet", "LinkedHashSet"} {
		for method, info := range collectionMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// -------------------------------------------------------------------------
	// kotlin.collections.Map methods
	// -------------------------------------------------------------------------
	mapMethods := stdlibSpecs("get=cAny!@1 getValue=cAny@1 getOrDefault=cAny@1 getOrElse=cAny@1 keys=cSet values=cCollection entries=cSet containsKey=pBoolean containsValue=pBoolean isEmpty=pBoolean size=pInt")
	for _, receiver := range []string{"Map", "MutableMap", "HashMap", "LinkedHashMap"} {
		for method, info := range mapMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// -------------------------------------------------------------------------
	// kotlin.sequences.Sequence methods
	// -------------------------------------------------------------------------
	seqMethods := stdlibSpecs("map=cSequence filter=cSequence flatMap=cSequence sorted=cSequence sortedBy=cSequence sortedWith=cSequence sortedDescending=cSequence sortedByDescending=cSequence distinct=cSequence distinctBy=cSequence drop=cSequence dropWhile=cSequence take=cSequence takeWhile=cSequence zip=cSequence toList=cList toSet=cSet first=cAny@0 firstOrNull=cAny!@0 count=pInt any=pBoolean all=pBoolean none=pBoolean")
	for method, info := range seqMethods {
		StdlibMethods["Sequence."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Int methods
	// -------------------------------------------------------------------------
	intMethods := stdlibSpecs("toString=pString toLong=pLong toFloat=pFloat toDouble=pDouble toShort=pShort toByte=pByte toChar=pChar coerceIn=pInt coerceAtLeast=pInt coerceAtMost=pInt compareTo=pInt plus=pInt minus=pInt times=pInt div=pInt rem=pInt mod=pInt inc=pInt dec=pInt unaryMinus=pInt unaryPlus=pInt shl=pInt shr=pInt ushr=pInt and=pInt or=pInt xor=pInt inv=pInt")
	for method, info := range intMethods {
		StdlibMethods["Int."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Long methods
	// -------------------------------------------------------------------------
	longMethods := stdlibSpecs("toString=pString toInt=pInt toFloat=pFloat toDouble=pDouble toShort=pShort toByte=pByte toChar=pChar coerceIn=pLong coerceAtLeast=pLong coerceAtMost=pLong compareTo=pInt plus=pLong minus=pLong times=pLong div=pLong rem=pLong mod=pLong inc=pLong dec=pLong unaryMinus=pLong unaryPlus=pLong shl=pLong shr=pLong ushr=pLong and=pLong or=pLong xor=pLong inv=pLong")
	for method, info := range longMethods {
		StdlibMethods["Long."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Double methods
	// -------------------------------------------------------------------------
	doubleMethods := stdlibSpecs("toString=pString toInt=pInt toLong=pLong toFloat=pFloat toByte=pByte toShort=pShort coerceIn=pDouble coerceAtLeast=pDouble coerceAtMost=pDouble compareTo=pInt plus=pDouble minus=pDouble times=pDouble div=pDouble rem=pDouble inc=pDouble dec=pDouble unaryMinus=pDouble unaryPlus=pDouble isNaN=pBoolean isInfinite=pBoolean isFinite=pBoolean")
	for method, info := range doubleMethods {
		StdlibMethods["Double."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Float methods
	// -------------------------------------------------------------------------
	floatMethods := stdlibSpecs("toString=pString toInt=pInt toLong=pLong toDouble=pDouble toByte=pByte toShort=pShort coerceIn=pFloat coerceAtLeast=pFloat coerceAtMost=pFloat compareTo=pInt plus=pFloat minus=pFloat times=pFloat div=pFloat rem=pFloat inc=pFloat dec=pFloat unaryMinus=pFloat unaryPlus=pFloat isNaN=pBoolean isInfinite=pBoolean isFinite=pBoolean")
	for method, info := range floatMethods {
		StdlibMethods["Float."+method] = info
	}

	// -------------------------------------------------------------------------
	// kotlin.Boolean methods
	// -------------------------------------------------------------------------
	boolMethods := stdlibSpecs("toString=pString compareTo=pInt not=pBoolean and=pBoolean or=pBoolean xor=pBoolean")
	for method, info := range boolMethods {
		StdlibMethods["Boolean."+method] = info
	}

	// -------------------------------------------------------------------------
	// Android Context methods
	// -------------------------------------------------------------------------
	StdlibMethods["Context.getString"] = sm("pString")
	StdlibMethods["Context.getResources"] = sm("cResources")
	StdlibMethods["Context.getSystemService"] = sm("cAny!")
	StdlibMethods["Context.getPackageName"] = sm("pString")
	StdlibMethods["Context.getApplicationContext"] = sm("cContext")
	StdlibMethods["Context.startActivity"] = sm("u")

	// -------------------------------------------------------------------------
	// java.io.File methods
	// -------------------------------------------------------------------------
	fileMethods := stdlibSpecs("exists=pBoolean isFile=pBoolean isDirectory=pBoolean readText=pString readLines=cList writeText=u delete=pBoolean mkdirs=pBoolean getName=pString getPath=pString getParent=pString! length=pLong listFiles=cArray! absolutePath=pString")
	for method, info := range fileMethods {
		StdlibMethods["File."+method] = info
	}

	// -------------------------------------------------------------------------
	// Set-specific methods (not shared with List)
	// -------------------------------------------------------------------------
	setSpecificMethods := stdlibSpecs("intersect=cSet union=cSet subtract=cSet plus=cSet minus=cSet toMutableSet=cMutableSet contains=pBoolean")
	for _, receiver := range []string{"Set", "MutableSet", "HashSet", "LinkedHashSet"} {
		for method, info := range setSpecificMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// -------------------------------------------------------------------------
	// MutableCollection mutation methods
	// -------------------------------------------------------------------------
	// MutableList
	for _, receiver := range []string{"MutableList", "ArrayList"} {
		StdlibMethods[receiver+".add"] = sm("pBoolean")
		StdlibMethods[receiver+".remove"] = sm("pBoolean")
		StdlibMethods[receiver+".removeAt"] = sm("cAny@0")
		StdlibMethods[receiver+".set"] = sm("cAny@0")
		StdlibMethods[receiver+".clear"] = sm("u")
		StdlibMethods[receiver+".addAll"] = sm("pBoolean")
		StdlibMethods[receiver+".removeAll"] = sm("pBoolean")
		StdlibMethods[receiver+".retainAll"] = sm("pBoolean")
		StdlibMethods[receiver+".subList"] = sm("cMutableList")
	}

	// MutableSet
	for _, receiver := range []string{"MutableSet", "HashSet", "LinkedHashSet"} {
		StdlibMethods[receiver+".add"] = sm("pBoolean")
		StdlibMethods[receiver+".remove"] = sm("pBoolean")
		StdlibMethods[receiver+".clear"] = sm("u")
		StdlibMethods[receiver+".addAll"] = sm("pBoolean")
		StdlibMethods[receiver+".removeAll"] = sm("pBoolean")
		StdlibMethods[receiver+".retainAll"] = sm("pBoolean")
	}

	// MutableMap
	for _, receiver := range []string{"MutableMap", "HashMap", "LinkedHashMap"} {
		StdlibMethods[receiver+".put"] = sm("cAny!@1")
		StdlibMethods[receiver+".remove"] = sm("cAny!@1")
		StdlibMethods[receiver+".clear"] = sm("u")
		StdlibMethods[receiver+".putAll"] = sm("u")
		StdlibMethods[receiver+".getOrPut"] = sm("cAny@1")
		StdlibMethods[receiver+".putIfAbsent"] = sm("cAny!@1")
	}

	// -------------------------------------------------------------------------
	// kotlinx.coroutines.flow.Flow methods
	// -------------------------------------------------------------------------
	flowMethods := stdlibSpecs("map=cFlow filter=cFlow filterNot=cFlow filterNotNull=cFlow flatMapConcat=cFlow flatMapMerge=cFlow flatMapLatest=cFlow transform=cFlow onEach=cFlow onStart=cFlow onCompletion=cFlow catch=cFlow retry=cFlow retryWhen=cFlow take=cFlow takeWhile=cFlow drop=cFlow dropWhile=cFlow distinctUntilChanged=cFlow debounce=cFlow sample=cFlow conflate=cFlow buffer=cFlow flowOn=cFlow combine=cFlow zip=cFlow collect=u collectLatest=u first=cAny@0 firstOrNull=cAny!@0 single=cAny@0 singleOrNull=cAny!@0 toList=cList toSet=cSet count=pInt fold=cAny reduce=cAny@0 stateIn=cStateFlow shareIn=cSharedFlow launchIn=cJob")
	for _, receiver := range []string{"Flow", "StateFlow", "SharedFlow", "MutableStateFlow", "MutableSharedFlow"} {
		for method, info := range flowMethods {
			StdlibMethods[receiver+"."+method] = info
		}
	}

	// StateFlow/SharedFlow specific properties
	StdlibMethods["StateFlow.value"] = sm("cAny@0")
	StdlibMethods["MutableStateFlow.value"] = sm("cAny@0")
	StdlibMethods["SharedFlow.replayCache"] = sm("cList")
	StdlibMethods["MutableSharedFlow.replayCache"] = sm("cList")
	StdlibMethods["MutableSharedFlow.emit"] = sm("u")
	StdlibMethods["MutableSharedFlow.tryEmit"] = sm("pBoolean")
	StdlibMethods["MutableStateFlow.compareAndSet"] = sm("pBoolean")

	// -------------------------------------------------------------------------
	// kotlinx.coroutines methods
	// -------------------------------------------------------------------------
	StdlibMethods["CoroutineScope.launch"] = sm("cJob")
	StdlibMethods["CoroutineScope.async"] = sm("cDeferred")
	StdlibMethods["_.withContext"] = sm("?")
	StdlibMethods["_.delay"] = sm("u")
	StdlibMethods["_.runBlocking"] = sm("?")
	StdlibMethods["_.coroutineScope"] = sm("?")
	StdlibMethods["_.supervisorScope"] = sm("?")
	StdlibMethods["_.yield"] = sm("u")
	StdlibMethods["_.withTimeout"] = sm("?")
	StdlibMethods["_.withTimeoutOrNull"] = sm("?!")
	StdlibMethods["_.flow"] = sm("cFlow")
	StdlibMethods["_.flowOf"] = sm("cFlow")
	StdlibMethods["_.channelFlow"] = sm("cFlow")
	StdlibMethods["_.callbackFlow"] = sm("cFlow")
	StdlibMethods["_.emptyFlow"] = sm("cFlow")
	StdlibMethods["_.MutableStateFlow"] = sm("cMutableStateFlow")
	StdlibMethods["_.MutableSharedFlow"] = sm("cMutableSharedFlow")

	// -------------------------------------------------------------------------
	// Kotlin scope functions
	// -------------------------------------------------------------------------
	// let/run return the lambda result (unknown without full inference)
	// apply/also return the receiver — handled specially in resolve.go
	// with is a top-level function that returns the lambda result
	StdlibMethods["_.let"] = sm("?")
	StdlibMethods["_.run"] = sm("?")
	StdlibMethods["_.with"] = sm("?")

	// -------------------------------------------------------------------------
	// Nothing-returning top-level functions
	// -------------------------------------------------------------------------
	// Only stdlib globals with no plausible user-defined conflict are listed
	// here. Workspace functions whose declared return type is Nothing are
	// resolved via r.functions in the resolver and don't need entries.
	nothing := &StdlibMethod{ReturnType: &ResolvedType{Name: "Nothing", FQN: "kotlin.Nothing", Kind: TypeNothing, Resolved: true}, ReturnTypeArgIndex: -1}
	StdlibMethods["_.TODO"] = nothing
	StdlibMethods["_.error"] = nothing
	StdlibMethods["_.exitProcess"] = nothing
}

// LookupStdlibMethod looks up a stdlib method by receiver type name and method name.
// It tries the exact receiver first, then falls back to common supertype aliases.
func LookupStdlibMethod(receiverType, methodName string) *StdlibMethod {
	key := receiverType + "." + methodName
	if m, ok := StdlibMethods[key]; ok {
		return m
	}
	// Try top-level function lookup (receiver-less)
	topKey := "_." + methodName
	if m, ok := StdlibMethods[topKey]; ok {
		return m
	}
	return nil
}

// ---------------------------------------------------------------------------
// Known interface implementors (framework types tree-sitter can't resolve)
// ---------------------------------------------------------------------------

// KnownInterfaces maps interface FQN -> known implementors.
var KnownInterfaces = strutil.DecodeListMap(`
		android.os.Parcelable=android.os.Bundle,android.content.Intent,android.net.Uri,android.graphics.Bitmap
		java.io.Closeable=java.io.InputStream,java.io.OutputStream,java.io.FileInputStream,java.io.FileOutputStream,java.io.BufferedReader,java.io.BufferedWriter,java.io.InputStreamReader,java.io.OutputStreamWriter,java.io.PrintWriter,java.io.PrintStream,java.net.Socket,java.net.ServerSocket,java.sql.Connection,java.sql.Statement,java.sql.ResultSet,java.util.Scanner
		java.io.Serializable=java.lang.Number,java.lang.String,java.lang.Enum,java.util.ArrayList,java.util.HashMap,java.util.HashSet,java.util.LinkedList,java.util.TreeMap,java.util.TreeSet,kotlin.Pair,kotlin.Triple,java.util.Date,java.util.Calendar,java.math.BigDecimal,java.math.BigInteger,java.io.File,java.net.URL,java.net.URI
	`)

// ---------------------------------------------------------------------------
// Known class hierarchies (Android/Java framework types)
// ---------------------------------------------------------------------------

// KnownClassHierarchy maps FQN -> ordered list of supertypes (nearest first).
var KnownClassHierarchy = strutil.DecodeListMap(`
		android.app.Activity=android.content.ContextWrapper,android.content.Context
		android.app.Application=android.content.ContextWrapper,android.content.Context
		android.app.Service=android.content.ContextWrapper,android.content.Context
		android.content.BroadcastReceiver=
		android.content.ContentProvider=
		android.view.ViewGroup=android.view.View
		android.widget.AutoCompleteTextView=android.widget.EditText,android.widget.TextView,android.view.View
		android.widget.Button=android.widget.TextView,android.view.View
		android.widget.CheckBox=android.widget.CompoundButton,android.widget.Button,android.widget.TextView,android.view.View
		android.widget.CompoundButton=android.widget.Button,android.widget.TextView,android.view.View
		android.widget.EditText=android.widget.TextView,android.view.View
		android.widget.FrameLayout=android.view.ViewGroup,android.view.View
		android.widget.ImageView=android.view.View
		android.widget.LinearLayout=android.view.ViewGroup,android.view.View
		android.widget.MultiAutoCompleteTextView=android.widget.AutoCompleteTextView,android.widget.EditText,android.widget.TextView,android.view.View
		android.widget.RadioButton=android.widget.CompoundButton,android.widget.Button,android.widget.TextView,android.view.View
		android.widget.RelativeLayout=android.view.ViewGroup,android.view.View
		android.widget.Switch=android.widget.CompoundButton,android.widget.Button,android.widget.TextView,android.view.View
		android.widget.TextView=android.view.View
		android.widget.ToggleButton=android.widget.CompoundButton,android.widget.Button,android.widget.TextView,android.view.View
		androidx.appcompat.app.AppCompatActivity=androidx.fragment.app.FragmentActivity,android.app.Activity
		androidx.appcompat.widget.AppCompatButton=androidx.appcompat.widget.AppCompatTextView,android.widget.Button,android.widget.TextView,android.view.View
		androidx.appcompat.widget.AppCompatCheckBox=android.widget.CheckBox,android.widget.CompoundButton,android.widget.Button,android.widget.TextView,android.view.View
		androidx.appcompat.widget.AppCompatEditText=android.widget.EditText,androidx.appcompat.widget.AppCompatTextView,android.widget.TextView,android.view.View
		androidx.appcompat.widget.AppCompatRadioButton=android.widget.RadioButton,android.widget.CompoundButton,android.widget.Button,android.widget.TextView,android.view.View
		androidx.appcompat.widget.AppCompatTextView=android.widget.TextView,android.view.View
		androidx.appcompat.widget.SwitchCompat=android.widget.CompoundButton,android.widget.Button,android.widget.TextView,android.view.View
		androidx.fragment.app.FragmentActivity=android.app.Activity,android.content.ContextWrapper,android.content.Context
		com.google.android.material.button.MaterialButton=androidx.appcompat.widget.AppCompatButton,android.widget.Button,android.widget.TextView,android.view.View
		com.google.android.material.textfield.TextInputEditText=androidx.appcompat.widget.AppCompatEditText,android.widget.EditText,android.widget.TextView,android.view.View
		java.lang.Exception=java.lang.Throwable
		java.lang.RuntimeException=java.lang.Exception,java.lang.Throwable
	`)

// simpleNameOf extracts the simple class name from a fully-qualified name.
func simpleNameOf(fqn string) string {
	for i := len(fqn) - 1; i >= 0; i-- {
		if fqn[i] == '.' {
			return fqn[i+1:]
		}
	}
	return fqn
}

// ImplementsInterface checks whether a given FQN is a known implementor of
// the specified interface FQN.
func ImplementsInterface(typeFQN, interfaceFQN string) bool {
	implementors, ok := KnownInterfaces[interfaceFQN]
	if !ok {
		return false
	}
	for _, impl := range implementors {
		if impl == typeFQN {
			return true
		}
	}
	return false
}

// IsKnownSubtype checks whether typeFQN is a known subtype of supertypeFQN
// using KnownClassHierarchy (transitive) and KnownInterfaces.
func IsKnownSubtype(typeFQN, supertypeFQN string) bool {
	if typeFQN == supertypeFQN {
		return true
	}
	// Check class hierarchy (transitive)
	if supers, ok := KnownClassHierarchy[typeFQN]; ok {
		for _, s := range supers {
			if s == supertypeFQN {
				return true
			}
		}
	}
	// Check interface implementors
	if ImplementsInterface(typeFQN, supertypeFQN) {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Exception hierarchy
// ---------------------------------------------------------------------------

// ExceptionAncestors maps exception class name -> ordered list of supertypes.
var ExceptionAncestors = strutil.DecodeListMap(`
		ActivityNotFoundException=RuntimeException,Exception,Throwable
		AndroidException=Exception,Throwable
		ArithmeticException=RuntimeException,Exception,Throwable
		ArrayIndexOutOfBoundsException=IndexOutOfBoundsException,RuntimeException,Exception,Throwable
		AssertionError=Error,Throwable
		BadParcelableException=RuntimeException,Exception,Throwable
		CancellationException=IllegalStateException,RuntimeException,Exception,Throwable
		CertificateException=GeneralSecurityException,Exception,Throwable
		ClassCastException=RuntimeException,Exception,Throwable
		ClassNotFoundException=ReflectiveOperationException,Exception,Throwable
		CloneNotSupportedException=Exception,Throwable
		ConcurrentModificationException=RuntimeException,Exception,Throwable
		ConnectException=SocketException,IOException,Exception,Throwable
		DeadObjectException=RemoteException,AndroidException,Exception,Throwable
		EOFException=IOException,Exception,Throwable
		Error=Throwable
		Exception=Throwable
		ExceptionInInitializerError=LinkageError,Error,Throwable
		FileNotFoundException=IOException,Exception,Throwable
		GeneralSecurityException=Exception,Throwable
		IOException=Exception,Throwable
		IllegalArgumentException=RuntimeException,Exception,Throwable
		IllegalStateException=RuntimeException,Exception,Throwable
		IndexOutOfBoundsException=RuntimeException,Exception,Throwable
		InterruptedException=Exception,Throwable
		InterruptedIOException=IOException,Exception,Throwable
		InvalidKeyException=GeneralSecurityException,Exception,Throwable
		JSONException=Exception,Throwable
		KotlinNullPointerException=NullPointerException,RuntimeException,Exception,Throwable
		LinkageError=Error,Throwable
		MalformedURLException=IOException,Exception,Throwable
		NegativeArraySizeException=RuntimeException,Exception,Throwable
		NoClassDefFoundError=LinkageError,Error,Throwable
		NoSuchAlgorithmException=GeneralSecurityException,Exception,Throwable
		NoSuchElementException=RuntimeException,Exception,Throwable
		NoSuchFieldException=ReflectiveOperationException,Exception,Throwable
		NoSuchMethodException=ReflectiveOperationException,Exception,Throwable
		NotImplementedError=Error,Throwable
		NullPointerException=RuntimeException,Exception,Throwable
		NumberFormatException=IllegalArgumentException,RuntimeException,Exception,Throwable
		OutOfMemoryError=VirtualMachineError,Error,Throwable
		ParseException=Exception,Throwable
		ReflectiveOperationException=Exception,Throwable
		RemoteException=AndroidException,Exception,Throwable
		RuntimeException=Exception,Throwable
		SQLiteConstraintException=SQLiteException,RuntimeException,Exception,Throwable
		SQLiteException=RuntimeException,Exception,Throwable
		SSLException=IOException,Exception,Throwable
		SSLHandshakeException=SSLException,IOException,Exception,Throwable
		SecurityException=RuntimeException,Exception,Throwable
		SocketException=IOException,Exception,Throwable
		SocketTimeoutException=InterruptedIOException,IOException,Exception,Throwable
		StackOverflowError=VirtualMachineError,Error,Throwable
		StringIndexOutOfBoundsException=IndexOutOfBoundsException,RuntimeException,Exception,Throwable
		Throwable=
		TimeoutCancellationException=CancellationException,IllegalStateException,RuntimeException,Exception,Throwable
		TimeoutException=Exception,Throwable
		TypeCastException=ClassCastException,RuntimeException,Exception,Throwable
		URISyntaxException=Exception,Throwable
		UninitializedPropertyAccessException=RuntimeException,Exception,Throwable
		UnknownHostException=IOException,Exception,Throwable
		UnsupportedOperationException=RuntimeException,Exception,Throwable
		VirtualMachineError=Error,Throwable
	`)

// KnownValueTypes lists types where referential equality (===) is almost
// certainly wrong and structural equality (==) should be used instead.
// Both Kotlin and Java FQNs are included so the lookup works regardless of
// whether the resolver produced a simple name or a fully-qualified name.
var KnownValueTypes = map[string]bool{
	// Kotlin primitives & wrappers
	"String": true, "kotlin.String": true,
	"Int": true, "kotlin.Int": true,
	"Long": true, "kotlin.Long": true,
	"Double": true, "kotlin.Double": true,
	"Float": true, "kotlin.Float": true,
	"Boolean": true, "kotlin.Boolean": true,
	"Byte": true, "kotlin.Byte": true,
	"Short": true, "kotlin.Short": true,
	"Char": true, "kotlin.Char": true,
	"UInt": true, "kotlin.UInt": true,
	"ULong": true, "kotlin.ULong": true,
	"UByte": true, "kotlin.UByte": true,
	"UShort": true, "kotlin.UShort": true,
	// Kotlin data holders
	"Pair": true, "kotlin.Pair": true,
	"Triple": true, "kotlin.Triple": true,
	// Java boxed types
	"java.lang.String": true, "java.lang.Integer": true, "java.lang.Long": true,
	"java.lang.Double": true, "java.lang.Float": true, "java.lang.Boolean": true,
	"java.lang.Byte": true, "java.lang.Short": true, "java.lang.Character": true,
	// Java math types
	"BigDecimal": true, "java.math.BigDecimal": true,
	"BigInteger": true, "java.math.BigInteger": true,
}

// IsKnownValueType returns true if the resolved type (by Name or FQN) is a
// known value type where structural equality should be preferred.
func IsKnownValueType(rt *ResolvedType) bool {
	if rt == nil || rt.Kind == TypeUnknown {
		return false
	}
	if KnownValueTypes[rt.Name] {
		return true
	}
	if KnownValueTypes[rt.FQN] {
		return true
	}
	return false
}

// IsSubtypeOfException checks if exceptionA is a subtype of exceptionB.
func IsSubtypeOfException(a, b string) bool {
	if a == b {
		return true
	}
	ancestors, ok := ExceptionAncestors[a]
	if !ok {
		return false
	}
	for _, anc := range ancestors {
		if anc == b {
			return true
		}
	}
	return false
}
