package rules

// Port of slack-lints' DenyListedApiDetector (slackhq/slack-lints 0.11.1,
// slack-lint-checks/src/main/java/slack/lint/denylistedapis/DenyListedApiDetector.kt): DenyListedApi and
// DenyListedBlockingApi report calls, constructors, static field references and static field imports of a fixed
// list of APIs, each with the entry's own message.
//
// Lint matches an entry by the resolved member's containing class. krit proves the owner from source: a static or
// constructor call through the class name (imported, in full, or through an on-demand import), a static import, a
// Kotlin top-level function through its import, or an instance call through the receiver's resolved or declared
// type. A receiver whose type is not shown abstains. Entries that Lint 0.11.1 cannot match on real libraries are
// left out: ResourcesCompat.getDrawable lists (Context, int), which the method does not take; ViewPager2.setId
// resolves to View.setId; and runBlocking resolves to kotlinx-coroutines' BuildersKt__BuildersKt part class, not
// the BuildersKt the entry names. As in Lint, a Kotlin property of a listed class (`date.time`) is its getter call.

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	slackDenyListedAPI         = "DenyListedApi"
	slackDenyListedBlockingAPI = "DenyListedBlockingApi"
)

// denyEntry is one deny-listed function. owner is the class FQN, or for a Kotlin top-level function its package
// (topLevel). member is the function name or "<init>" for a constructor. args, when set, are the exact argument
// texts the call must have; otherwise nargs is the argument count the call must have, or -1 for any.
type denyEntry struct {
	issue    string
	owner    string
	topLevel bool
	member   string
	args     []string
	nargs    int
	message  string
}

const (
	denyDateMessage       = "Use java.time.Instant or java.time.ZonedDateTime instead. There is no reason to use java.util.Date in Java 8+."
	denyCalendarMessage   = "Use java.time.Instant or java.time.ZonedDateTime instead. There is no reason to use java.util.Calendar in Java 8+."
	denyDateFormatMessage = "Use java.time.DateTimeFormatter instead. There is no reason to use java.text.DateFormat in Java 8+."
)

func denyRxHideMessage(typ, sub, example string) string {
	return "There should be no reason to defend against downcasting " + typ + " to an implementation type like " + sub +
		" in a closed codebase. Doing this incurs needless runtime memory and performance overhead. " + example
}

func denyRxBlockingMessage(short string) string {
	or := " Completable (if you want to hide emission values but defer subscription),"
	if short == "Completable" {
		or = ""
	}
	return "Blocking calls in RxJava can cause deadlocks and application jank. Prefer making the enclosing method/function return this " +
		short + ", a Disposable to grant control to the caller," + or + " or refactoring this in a way to use non-blocking calls. " +
		"If running in a test, use the .test()/TestObserver API (https://reactivex.io/RxJava/3.x/javadoc/io/reactivex/rxjava3/observers/TestObserver.html) test synchronous values."
}

var denyEntries = func() []denyEntry {
	rx := "io.reactivex.rxjava3.core."
	e := []denyEntry{
		{issue: slackDenyListedAPI, owner: rx + "Observable", member: "hide", nargs: -1, message: denyRxHideMessage("an Observable", "Relay or Subject",
			"Relays and Subjects both extend from Observable and can be supplied to functions accepting Observable directly. When returning a Relay or Subject, declare the return type explicitly as Observable (e.g., fun foo(): Observable<Foo> = fooRelay).")},
		{issue: slackDenyListedAPI, owner: rx + "Flowable", member: "hide", nargs: -1, message: denyRxHideMessage("an Flowable", "FlowableProcessor",
			"FlowableProcessor extends from Flowable and can be supplied to functions accepting Flowable directly. When returning a FlowableProcessor, declare the return type explicitly as Flowable (e.g., fun foo(): Flowable<Foo> = fooProcessor).")},
		{issue: slackDenyListedAPI, owner: rx + "Completable", member: "hide", nargs: -1, message: denyRxHideMessage("a Completable", "CompletableSubject",
			"CompletableSubject extends from Completable and can be supplied to functions accepting Completable directly. When returning a CompletableSubject, declare the return type explicitly as Completable (e.g., fun foo(): Completable<Foo> = fooSubject).")},
		{issue: slackDenyListedAPI, owner: rx + "Maybe", member: "hide", nargs: -1, message: denyRxHideMessage("a Maybe", "MaybeSubject",
			"MaybeSubject extends from Maybe and can be supplied to functions accepting Maybe directly. When returning a MaybeSubject, declare the return type explicitly as Maybe (e.g., fun foo(): Maybe<Foo> = fooSubject).")},
		{issue: slackDenyListedAPI, owner: rx + "Single", member: "hide", nargs: -1, message: denyRxHideMessage("a Single", "SingleSubject",
			"SingleSubject extends from Single and can be supplied to functions accepting Single directly. When returning a SingleSubject, declare the return type explicitly as Single (e.g., fun foo(): Single<Foo> = fooSubject).")},
		{issue: slackDenyListedAPI, owner: "androidx.core.content.ContextCompat", member: "getDrawable", nargs: 2, message: "Use Context#getDrawableCompat() instead"},
		{issue: slackDenyListedAPI, owner: "android.support.test.espresso.matcher.ViewMatchers", member: "withId", nargs: 1, message: "Consider matching the content description instead. IDs are implementation details of how a screen is built, not how it works. You can't tell a user to click on the button with ID 428194727 so our tests should not be doing that."},
		{issue: slackDenyListedAPI, owner: "android.view.View", member: "setOnClickListener", args: []string{"null"}, message: "This fails to also set View#isClickable. Use View#clearOnClickListener() instead"},
		{issue: slackDenyListedAPI, owner: "kotlinx.coroutines.flow", topLevel: true, member: "launchIn", nargs: -1, message: "Use the structured concurrent CoroutineScope#launch and Flow#collect APIs instead of reactive Flow#onEach and Flow#launchIn. Suspend calls like Flow#collect can be refactored into standalone suspend funs and mixed in with regular control flow in a suspend context, but calls that invoke CoroutineScope#launch and Flow#collect at the same time hide the suspend context, encouraging the developer to continue working in the reactive domain."},
		{issue: slackDenyListedAPI, owner: "java.util.LinkedList", member: "<init>", nargs: -1, message: "For a stack/queue/double-ended queue use ArrayDeque, for a list use ArrayList. Both are more efficient internally."},
		{issue: slackDenyListedAPI, owner: "java.util.Stack", member: "<init>", nargs: -1, message: "For a stack use ArrayDeque which is more efficient internally."},
		{issue: slackDenyListedAPI, owner: "java.util.Vector", member: "<init>", nargs: -1, message: "For a vector use ArrayList or ArrayDeque which are more efficient internally."},
		{issue: slackDenyListedAPI, owner: "io.reactivex.rxjava3.schedulers.Schedulers", member: "newThread", nargs: -1, message: "Use a scheduler which wraps a cached set of threads. There should be no reason to be arbitrarily creating threads on Android."},
		{issue: slackDenyListedAPI, owner: "kotlin", topLevel: true, member: "runCatching", nargs: -1, message: "runCatching has hidden issues when used with coroutines as it catches and doesn't rethrow CancellationException. This can interfere with coroutines cancellation handling! Prefer catching specific exceptions based on the current case."},
	}
	for _, name := range []string{"rxCompletable", "rxMaybe", "rxSingle", "rxObservable"} {
		// The entry lists the overload's arguments as `*`: one argument, the block, so no dispatcher.
		e = append(e, denyEntry{issue: slackDenyListedAPI, owner: "kotlinx.coroutines.rx3", topLevel: true, member: name, nargs: 1,
			message: name + " defaults to Dispatchers.Default. Provide an explicit dispatcher which can be replaced with a test dispatcher to make your tests more deterministic."})
	}
	streams := []string{"blockingFirst", "blockingForEach", "blockingIterable", "blockingLatest", "blockingMostRecent", "blockingNext", "blockingSingle", "blockingSubscribe"}
	for _, blocking := range []struct {
		short   string
		methods []string
	}{
		{"Completable", []string{"blockingAwait"}},
		{"Single", []string{"blockingGet", "blockingSubscribe"}},
		{"Maybe", []string{"blockingGet", "blockingSubscribe"}},
		{"Observable", streams},
		{"Flowable", streams},
	} {
		for _, m := range blocking.methods {
			e = append(e, denyEntry{issue: slackDenyListedBlockingAPI, owner: rx + blocking.short, member: m, nargs: -1, message: denyRxBlockingMessage(blocking.short)})
		}
	}
	return e
}()

// denyAllMemberOwners are the classes whose every function and constructor is deny-listed.
var denyAllMemberOwners = map[string]string{"java.util.Date": denyDateMessage, "java.util.Calendar": denyCalendarMessage,
	"java.text.DateFormat": denyDateFormatMessage, "java.text.SimpleDateFormat": denyDateFormatMessage}

// denyAllFieldOwners are the classes whose every field is deny-listed.
var denyAllFieldOwners = map[string]string{"java.util.Calendar": denyCalendarMessage, "java.text.DateFormat": denyDateFormatMessage,
	"java.text.SimpleDateFormat": denyDateFormatMessage}

// denyKnownClasses are the classes the list names, by FQN, with the simple name of each.
var denyKnownClasses = func() map[string]string {
	m := map[string]string{"android.view.View": "View"}
	for _, e := range denyEntries {
		if !e.topLevel {
			m[e.owner] = e.owner[strings.LastIndexByte(e.owner, '.')+1:]
		}
	}
	for owner := range denyAllMemberOwners {
		m[owner] = owner[strings.LastIndexByte(owner, '.')+1:]
	}
	return m
}()

// denyMemberNames are the names of the listed functions outside denyAllMemberOwners, which every other call is
// filtered by before any resolution.
var denyMemberNames = func() map[string]bool {
	m := map[string]bool{}
	for _, e := range denyEntries {
		m[e.member] = true
	}
	return m
}()

// denyFileGateWords are the texts a file must contain for DenyListedApi to look at it: an import or full name of a
// listed package or class, or a listed function name.
var denyFileGateWords = [][]byte{[]byte("java.util"), []byte("java.text"), []byte(".hide("), []byte("newThread"),
	[]byte("launchIn"), []byte("runCatching"), []byte("kotlinx.coroutines.rx3"), []byte("ContextCompat"),
	[]byte("setOnClickListener"), []byte("withId")}

func denyListFileGate(file *scanner.File) bool {
	for _, w := range denyFileGateWords {
		if bytes.Contains(file.Content, w) {
			return true
		}
	}
	return false
}

func denyBlockingFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("rxjava3")) && bytes.Contains(file.Content, []byte(".blocking"))
}

// checkDenyListedAPI reports DenyListedApi findings at a call, constructor call, static field reference or import.
func checkDenyListedAPI(ctx *api.Context) {
	var hits []denyHit
	switch ctx.File.FlatType(ctx.Idx) {
	case "call_expression", "method_invocation", "object_creation_expression":
		hits = denyCallHits(ctx, ctx.Idx)
	case "navigation_expression", "field_access":
		hits = denyFieldHits(ctx, ctx.Idx)
	case "import_header", "import_declaration":
		hits = denyImportHits(ctx.File, ctx.Idx)
	}
	denyEmit(ctx, hits, slackDenyListedAPI)
}

// checkDenyListedBlockingAPI reports DenyListedBlockingApi findings at a blocking RxJava 3 call.
func checkDenyListedBlockingAPI(ctx *api.Context) {
	if name := slackCalleeNameNode(ctx.File, ctx.Idx); name == 0 || !strings.HasPrefix(ctx.File.FlatNodeText(name), "blocking") {
		return
	}
	denyEmit(ctx, denyCallHits(ctx, ctx.Idx), slackDenyListedBlockingAPI)
}

func denyEmit(ctx *api.Context, hits []denyHit, issue string) {
	for _, h := range hits {
		if h.issue == issue {
			ctx.EmitAtNode(h.node, h.message)
		}
	}
}

type denyHit struct {
	issue, message string
	node           uint32
}

// denyCallHits matches a call or constructor call against the list.
func denyCallHits(ctx *api.Context, idx uint32) []denyHit {
	file := ctx.File
	isJavaNew := file.FlatType(idx) == "object_creation_expression"
	// `f(x) { ... }` parses as a call whose callee is the call `f(x)`: the inner call is matched, and the outer
	// one's trailing lambda counts as its argument.
	if !isJavaNew && file.FlatType(file.FlatFirstChild(idx)) == "call_expression" {
		return nil
	}
	if !isJavaNew {
		if name := slackCalleeNameNode(file, idx); name == 0 || !denyCallMayMatch(file, file.FlatNodeText(name)) {
			return nil
		}
	}
	call, ok := lintCallAt(file, idx)
	if !ok {
		return nil
	}
	if isJavaNew {
		return denyConstructorHits(denyClassFQN(file, call.TypeText), call, idx)
	}
	name := slackCalleeNameNode(file, idx)
	if name == 0 {
		return nil
	}
	if file.Language == scanner.LangKotlin && call.Name != "" && call.Name[0] >= 'A' && call.Name[0] <= 'Z' {
		// A Kotlin constructor call, `LinkedList<T>()` or `java.util.LinkedList()`.
		return denyConstructorHits(denyClassFQN(file, call.TypeText), call, name)
	}
	nargs := len(call.Args)
	if denyTrailingLambda(file, idx) {
		nargs++
	}
	if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == idx && denyTrailingLambda(file, parent) {
		nargs++
	}
	var out []denyHit
	for _, owner := range denyCallOwners(ctx, call) {
		if msg, ok := denyAllMemberOwners[owner]; ok {
			if denyDeclares(owner, call.Name) {
				return []denyHit{{slackDenyListedAPI, msg, name}}
			}
			continue
		}
		out = append(out, denyMatch(owner, call.Name, call, nargs, name)...)
	}
	return out
}

func denyConstructorHits(owner string, call lintCall, at uint32) []denyHit {
	if msg, ok := denyAllMemberOwners[owner]; ok {
		return []denyHit{{slackDenyListedAPI, msg, at}}
	}
	return denyMatch(owner, "<init>", call, len(call.Args), at)
}

// denyTrailingLambda reports whether a Kotlin call passes a lambda after its parentheses.
func denyTrailingLambda(file *scanner.File, call uint32) bool {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return false
	}
	_, ok = file.FlatFindChild(suffix, "annotated_lambda")
	return ok
}

// denyMatch returns the hits of the entries for owner.member whose arguments the call matches.
func denyMatch(owner, member string, call lintCall, nargs int, at uint32) []denyHit {
	var out []denyHit
	for _, e := range denyEntries {
		if e.owner != owner || e.member != member {
			continue
		}
		if e.args != nil {
			if nargs != len(e.args) || len(call.Args) != len(e.args) {
				continue
			}
			matched := true
			for i, want := range e.args {
				if denyArgText(call, i) != want {
					matched = false
				}
			}
			if !matched {
				continue
			}
		} else if e.nargs >= 0 && nargs != e.nargs {
			continue
		}
		out = append(out, denyHit{e.issue, e.message, at})
	}
	return out
}

// denyArgText returns the source text of a call's argument i. A Kotlin `null` argument has no expression node, so
// its value_argument's text is used.
func denyArgText(call lintCall, i int) string {
	file := call.File
	if call.Args[i] != 0 {
		return strings.TrimSpace(file.FlatNodeText(call.Args[i]))
	}
	n := 0
	for arg := file.FlatFirstChild(flatCallKeyArguments(file, call.Idx)); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			if n == i {
				return strings.TrimSpace(file.FlatNodeText(arg))
			}
			n++
		}
	}
	return ""
}

// denyCallOwners returns the classes, or for Kotlin top-level functions the packages, a call is proven to belong
// to.
func denyCallOwners(ctx *api.Context, call lintCall) []string {
	file := ctx.File
	imp := lintImportsOf(file)
	full, imported := imp.bySimple[call.Name]
	if call.ReceiverNode == 0 {
		if !denyMemberNames[call.Name] && !denyMentionsAllMemberOwner(file) {
			return nil
		}
		if file.Language == scanner.LangKotlin && slackFunctionShadows(file, call.Idx, call.Name, imported) {
			return nil
		}
		// A statically imported function, or a Kotlin top-level one.
		if imported {
			if dot := strings.LastIndexByte(full, '.'); dot > 0 && full[dot+1:] == call.Name {
				return []string{full[:dot]}
			}
			return nil
		}
		owners := append([]string(nil), imp.stars...)
		switch file.Language {
		case scanner.LangKotlin:
			owners = append(owners, "kotlin")
		case scanner.LangJava:
			if cls := denyEnclosingJavaSuperclass(file, call.Idx); cls != "" {
				owners = append(owners, cls)
			}
		}
		return owners
	}
	// A static call through the class name.
	if fqn := denyClassFQN(file, call.ReceiverText); fqn != "" {
		return []string{fqn}
	}
	var owners []string
	if file.Language == scanner.LangKotlin {
		// An extension function: through its import, an on-demand import of its package, or kotlin's default import.
		for _, e := range denyEntries {
			if e.topLevel && e.member == call.Name && (full == e.owner+"."+call.Name || e.owner == "kotlin" || denyHasStar(imp, e.owner)) {
				owners = append(owners, e.owner)
			}
		}
	}
	if !denyMemberNames[call.Name] && !denyMentionsAllMemberOwner(file) {
		return owners
	}
	if denyNullableAnyExtension(ctx, call) {
		return owners
	}
	if typ := denyReceiverType(ctx, call.ReceiverNode); typ != "" {
		owners = append(owners, denyMethodOwner(ctx, typ))
	}
	return owners
}

func denyHasStar(imp *lintImports, pkg string) bool {
	for _, s := range imp.stars {
		if s == pkg {
			return true
		}
	}
	return false
}

// denyMentionsAllMemberOwner reports whether the file imports or names in full one of denyAllMemberOwners, whose
// every method is listed, so that calls and properties of any name need their receiver typed. It is computed once per
// file.
func denyMentionsAllMemberOwner(file *scanner.File) bool {
	return filefacts.FileFact(fileFactsCache(), file, slotDenyAllMemberOwner, func() bool {
		imp := lintImportsOf(file)
		for _, full := range imp.bySimple {
			if _, ok := denyAllMemberOwners[full]; ok {
				return true
			}
		}
		for _, star := range imp.stars {
			if star == "java.util" || star == "java.text" {
				return true
			}
		}
		return bytes.Contains(file.Content, []byte("java.util.Date")) || bytes.Contains(file.Content, []byte("java.util.Calendar")) ||
			bytes.Contains(file.Content, []byte("java.text."))
	})
}

const slotDenyAllMemberOwner = "denyAllMemberOwner"

// denyConstructorNames are the simple names of the listed classes whose constructors are listed.
var denyConstructorNames = map[string]bool{"LinkedList": true, "Stack": true, "Vector": true, "Date": true, "SimpleDateFormat": true}

// denyCallMayMatch is the cheap filter run before a call is resolved: a listed function name, a listed constructor's
// class name, or any name in a file that can reach a class whose every method is listed.
func denyCallMayMatch(file *scanner.File, name string) bool {
	return denyMemberNames[name] || denyConstructorNames[name] || denyMentionsAllMemberOwner(file)
}

// denyNullableAnyExtension reports whether a Kotlin `x.toString()`, `x.equals(y)` or `x.hashCode()` without a safe
// call has a nullable receiver, which makes it kotlin's `Any?` extension rather than the class's method.
func denyNullableAnyExtension(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if file.Language != scanner.LangKotlin || (call.Name != "toString" && call.Name != "equals" && call.Name != "hashCode") {
		return false
	}
	if nav := file.FlatFirstChild(call.Idx); file.FlatType(nav) == "navigation_expression" && strings.Contains(file.FlatNodeText(nav), "?.") {
		return false
	}
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(call.ReceiverNode, file); typ != nil && typ.Name != "" {
			return typ.Nullable
		}
	}
	return file.FlatType(call.ReceiverNode) == "simple_identifier" && strings.HasSuffix(denyVariableTypeText(file, call.ReceiverNode), "?")
}

// denyMethodOwner maps a receiver type to the listed class a method called on it is declared in: RxJava 3 subjects,
// processors, connectable and grouped streams and RxRelay relays inherit the five base types' methods, and every
// View subtype inherits View's.
func denyMethodOwner(ctx *api.Context, fqn string) string {
	simple := fqn[strings.LastIndexByte(fqn, '.')+1:]
	pkg := strings.TrimSuffix(fqn, "."+simple)
	switch pkg {
	case "io.reactivex.rxjava3.core", "io.reactivex.rxjava3.subjects", "io.reactivex.rxjava3.processors",
		"io.reactivex.rxjava3.observables", "io.reactivex.rxjava3.flowables", "com.jakewharton.rxrelay3":
		for _, base := range []string{"Completable", "Single", "Maybe", "Flowable"} {
			if strings.Contains(simple, base) {
				return "io.reactivex.rxjava3.core." + base
			}
		}
		if strings.HasSuffix(simple, "Processor") {
			return "io.reactivex.rxjava3.core.Flowable"
		}
		return "io.reactivex.rxjava3.core.Observable"
	}
	if slackClassExtends(ctx.Resolver, fqn, "android.view.View") {
		return "android.view.View"
	}
	return fqn
}

// denyReceiverType returns the FQN of a receiver's type: the source resolver's, the declared type of the variable a
// name refers to (or the listed class its initializer constructs), or the class a constructor call or a listed
// class's static factory returns.
func denyReceiverType(ctx *api.Context, recv uint32) string {
	file := ctx.File
	// The resolver answers a member it does not know with the owner's type (`cal.time` as Calendar), so a
	// navigation receiver is typed below instead.
	if ctx.Resolver != nil && file.FlatType(recv) != "navigation_expression" {
		if typ := ctx.Resolver.ResolveFlatNode(recv, file); typ != nil && typ.Name != "" {
			// The file's imports decide which listed class a simple name is: the resolver can pick a same-named
			// source class from another package (HOM-31).
			if listed := denyClassFQN(file, typ.Name); listed != "" && lintImportsOf(file).bySimple[typ.Name] != typ.FQN {
				return listed
			}
			if strings.Contains(typ.FQN, ".") {
				return typ.FQN
			}
			if fqn := lintImportsOf(file).bySimple[typ.Name]; fqn != "" {
				return fqn
			}
			// A source class the resolver names without its package.
			if info := ctx.Resolver.ClassHierarchy(typ.Name); info != nil && strings.Contains(info.FQN, ".") {
				return info.FQN
			}
		}
	}
	switch file.FlatType(recv) {
	case "simple_identifier", "identifier":
		return denyAnyClassFQN(file, denyVariableTypeText(file, recv))
	case "call_expression", "object_creation_expression":
		return denyCallResultClass(file, recv)
	case "navigation_expression":
		// `calendar.time` is Calendar.getTime(), a Date.
		if sel := denySelectorNode(file, recv); sel != 0 && file.FlatNodeText(sel) == "time" &&
			denyReceiverType(ctx, file.FlatFirstChild(recv)) == "java.util.Calendar" {
			return "java.util.Date"
		}
	case "parenthesized_expression":
		for c := file.FlatFirstChild(recv); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				return denyReceiverType(ctx, c)
			}
		}
	}
	return ""
}

// denyCallResultClass returns the listed class a constructor call (`Date()`, `new Date()`) creates or a listed
// class's static call (`Calendar.getInstance()`) returns, or "".
func denyCallResultClass(file *scanner.File, call uint32) string {
	c, ok := lintCallAt(file, call)
	if !ok {
		return ""
	}
	if file.FlatType(call) == "object_creation_expression" || c.ReceiverText == "" {
		return denyClassFQN(file, c.TypeText)
	}
	if fqn := denyClassFQN(file, c.ReceiverText); denyAllMemberOwners[fqn] != "" && (c.Name == "getInstance" || c.Name == "getDateInstance" ||
		c.Name == "getTimeInstance" || c.Name == "getDateTimeInstance") {
		return fqn
	}
	return ""
}

// denyFieldHits reports a static field reference through the class name (`Calendar.YEAR`), and a Kotlin property
// of a class whose every method is listed (`date.time`), which Lint resolves to its getter.
func denyFieldHits(ctx *api.Context, idx uint32) []denyHit {
	file := ctx.File
	if !denyMentionsAllMemberOwner(file) {
		return nil
	}
	if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == idx {
		return nil
	}
	recv := file.FlatFirstChild(idx)
	selector := denySelectorNode(file, idx)
	if recv == 0 || selector == 0 {
		return nil
	}
	if denyIsConstantName(file.FlatNodeText(selector)) {
		if msg, ok := denyAllFieldOwners[denyClassFQN(file, file.FlatNodeText(recv))]; ok {
			return []denyHit{{slackDenyListedAPI, msg, idx}}
		}
		return nil
	}
	if file.Language != scanner.LangKotlin {
		return nil
	}
	owner := denyReceiverType(ctx, recv)
	if msg, ok := denyAllMemberOwners[owner]; ok && denyDeclaresProperty(owner, file.FlatNodeText(selector)) {
		return []denyHit{{slackDenyListedAPI, msg, selector}}
	}
	return nil
}

// denyDeclaredMethods are the public methods each of denyAllMemberOwners declares (JDK 17). Lint matches the class
// that declares the resolved method, so an inherited Object method or a Kotlin extension (`date.let {}`) is not
// reported, and a SimpleDateFormat call of a DateFormat method is reported as DateFormat's.
var denyDeclaredMethods = map[string]map[string]bool{
	"java.util.Date": denyNameSet("after before clone compareTo equals from getDate getDay getHours getMinutes getMonth " +
		"getSeconds getTime getTimezoneOffset getYear hashCode parse setDate setHours setMinutes setMonth setSeconds setTime " +
		"setYear toGMTString toInstant toLocaleString toString UTC"),
	"java.util.Calendar": denyNameSet("add after before clear clone compareTo equals get getActualMaximum getActualMinimum " +
		"getAvailableCalendarTypes getAvailableLocales getCalendarType getDisplayName getDisplayNames getFirstDayOfWeek " +
		"getGreatestMinimum getInstance getLeastMaximum getMaximum getMinimalDaysInFirstWeek getMinimum getTime " +
		"getTimeInMillis getTimeZone getWeeksInWeekYear getWeekYear hashCode isLenient isSet isWeekDateSupported roll set " +
		"setFirstDayOfWeek setLenient setMinimalDaysInFirstWeek setTime setTimeInMillis setTimeZone setWeekDate toInstant toString"),
	"java.text.DateFormat": denyNameSet("clone equals format getAvailableLocales getCalendar getDateInstance " +
		"getDateTimeInstance getInstance getNumberFormat getTimeInstance getTimeZone hashCode isLenient parse parseObject " +
		"setCalendar setLenient setNumberFormat setTimeZone"),
	"java.text.SimpleDateFormat": denyNameSet("applyLocalizedPattern applyPattern clone equals format " +
		"formatToCharacterIterator get2DigitYearStart getDateFormatSymbols hashCode parse set2DigitYearStart " +
		"setDateFormatSymbols toLocalizedPattern toPattern"),
}

func denyNameSet(names string) map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(names) {
		m[n] = true
	}
	return m
}

// denyDeclares reports whether a method called name on owner resolves to one of the listed classes: owner itself,
// or for SimpleDateFormat its superclass DateFormat.
func denyDeclares(owner, name string) bool {
	if denyDeclaredMethods[owner][name] {
		return true
	}
	return owner == "java.text.SimpleDateFormat" && denyDeclaredMethods["java.text.DateFormat"][name]
}

// denyDeclaresProperty reports whether a Kotlin property of owner is one of its listed getters or setters.
func denyDeclaresProperty(owner, name string) bool {
	if name == "" {
		return false
	}
	// A Boolean getter keeps its name (`isLenient`); others drop `get` (`time` is getTime and setTime).
	if strings.HasPrefix(name, "is") && denyDeclares(owner, name) {
		return true
	}
	capital := strings.ToUpper(name[:1]) + name[1:]
	return denyDeclares(owner, "get"+capital) || denyDeclares(owner, "set"+capital)
}

// denyImportHits reports an import of a listed static field (`import java.util.Calendar.YEAR`).
func denyImportHits(file *scanner.File, idx uint32) []denyHit {
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "import"))
	text = strings.TrimSpace(strings.TrimPrefix(text, "static "))
	text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
	if i := strings.Index(text, " as "); i >= 0 {
		text = text[:i]
	}
	text = strings.Join(strings.Fields(text), "")
	dot := strings.LastIndexByte(text, '.')
	if dot < 0 {
		return nil
	}
	if msg, ok := denyAllFieldOwners[text[:dot]]; ok && denyIsConstantName(text[dot+1:]) {
		return []denyHit{{slackDenyListedAPI, msg, idx}}
	}
	return nil
}

func denyIsConstantName(s string) bool {
	return s != "" && s[0] >= 'A' && s[0] <= 'Z' && strings.ToUpper(s) == s
}

// denyClassFQN resolves the name of a listed class written in file: in full, through an import or through an
// on-demand import of its package. It returns "" for anything else.
func denyClassFQN(file *scanner.File, text string) string {
	fqn := denyAnyClassFQN(file, text)
	if _, ok := denyKnownClasses[fqn]; ok {
		return fqn
	}
	return ""
}

// denyAnyClassFQN resolves a class name through the file's imports; a name with no import resolves to itself, or
// to a listed class of an on-demand-imported package.
func denyAnyClassFQN(file *scanner.File, text string) string {
	text = strings.TrimSuffix(strings.Join(strings.Fields(text), ""), "?")
	if i := strings.IndexByte(text, '<'); i >= 0 {
		text = text[:i]
	}
	if text == "" {
		return ""
	}
	imp := lintImportsOf(file)
	head, rest, nested := strings.Cut(text, ".")
	if full, ok := imp.bySimple[head]; ok {
		if nested {
			return full + "." + rest
		}
		return full
	}
	if !nested {
		for _, star := range imp.stars {
			if _, ok := denyKnownClasses[star+"."+text]; ok {
				return star + "." + text
			}
		}
	}
	return text
}

// denyEnclosingJavaSuperclass returns the listed class a Java class enclosing idx directly extends, whose methods an
// unqualified call can reach, or "".
func denyEnclosingJavaSuperclass(file *scanner.File, idx uint32) string {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "class_declaration" {
			continue
		}
		if sc, ok := file.FlatFindChild(p, "superclass"); ok {
			for c := file.FlatFirstChild(sc); c != 0; c = file.FlatNextSib(c) {
				if fqn := denyClassFQN(file, file.FlatNodeText(c)); fqn != "" && file.FlatIsNamed(c) {
					return fqn
				}
			}
		}
		return ""
	}
	return ""
}

// denySelectorNode returns the member name of a Kotlin navigation_expression or Java field_access, or 0.
func denySelectorNode(file *scanner.File, nav uint32) uint32 {
	var last uint32
	for c := file.FlatFirstChild(nav); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "navigation_suffix":
			if id, ok := file.FlatFindChild(c, "simple_identifier"); ok {
				last = id
			}
		case "identifier":
			last = c
		}
	}
	if last == file.FlatFirstChild(nav) {
		return 0
	}
	return last
}

// denyVariableTypeText returns the declared type of the Kotlin or Java variable or parameter a name refers to, or,
// for a Kotlin property without a type, the listed class its initializer creates; "" when neither is shown.
func denyVariableTypeText(file *scanner.File, use uint32) string {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements", "block", "class_body", "source_file":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if t, matched := denyDeclarationType(file, sib, name); matched {
					return t
				}
			}
			if file.FlatType(parent) == "class_body" {
				for sib, found := file.FlatNextSibling(child); found; sib, found = file.FlatNextSibling(sib) {
					if t, matched := denyDeclarationType(file, sib, name); matched {
						return t
					}
				}
			}
		case "function_declaration", "method_declaration", "constructor_declaration", "secondary_constructor":
			for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) != "function_value_parameters" && file.FlatType(c) != "formal_parameters" {
					continue
				}
				for p := file.FlatFirstChild(c); p != 0; p = file.FlatNextSib(p) {
					if t, matched := denyParameterType(file, p, name); matched {
						return t
					}
				}
			}
		case "lambda_literal":
			if params, ok := file.FlatFindChild(parent, "lambda_parameters"); ok && lintParamsDeclare(file, params, name) {
				return ""
			}
		case "class_declaration":
			if ctor, ok := file.FlatFindChild(parent, "primary_constructor"); ok {
				if params, ok := file.FlatFindChild(ctor, "class_parameters"); ok {
					ctor = params
				}
				for p := file.FlatFirstChild(ctor); p != 0; p = file.FlatNextSib(p) {
					if t, matched := denyParameterType(file, p, name); matched {
						return t
					}
				}
			}
		}
		child = parent
	}
	return ""
}

func denyParameterType(file *scanner.File, p uint32, name string) (string, bool) {
	switch file.FlatType(p) {
	case "parameter", "class_parameter":
		if extractIdentifierFlat(file, p) == name {
			return denyKotlinTypeText(file, slackDeclaredTypeAfterColon(file, p)), true
		}
	case "formal_parameter":
		if id, ok := file.FlatFindChild(p, "identifier"); ok && file.FlatNodeText(id) == name {
			return denyJavaTypeText(file, p), true
		}
	}
	return "", false
}

func denyDeclarationType(file *scanner.File, decl uint32, name string) (string, bool) {
	switch file.FlatType(decl) {
	case "property_declaration":
		v, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || extractIdentifierFlat(file, v) != name {
			return "", false
		}
		if t := denyKotlinTypeText(file, slackDeclaredTypeAfterColon(file, v)); t != "" {
			return t, true
		}
		if init := lintInitializerAfterEquals(file, decl); file.FlatType(init) == "call_expression" {
			return denyCallResultClass(file, init), true
		}
		return "", true
	case "local_variable_declaration", "field_declaration":
		for d := file.FlatFirstChild(decl); d != 0; d = file.FlatNextSib(d) {
			if file.FlatType(d) == "variable_declarator" {
				if id, ok := file.FlatFindChild(d, "identifier"); ok && file.FlatNodeText(id) == name {
					return denyJavaTypeText(file, decl), true
				}
			}
		}
	}
	return "", false
}

// denyKotlinTypeText returns the (possibly qualified) name of a Kotlin user type, with a `?` suffix when it is
// nullable, or "".
func denyKotlinTypeText(file *scanner.File, typ uint32) string {
	suffix := ""
	if file.FlatType(typ) == "nullable_type" {
		typ = file.FlatFirstChild(typ)
		suffix = "?"
	}
	if typ == 0 || file.FlatType(typ) != "user_type" {
		return ""
	}
	var parts []string
	for c := file.FlatFirstChild(typ); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "type_identifier" {
			parts = append(parts, file.FlatNodeText(c))
		}
	}
	return strings.Join(parts, ".") + suffix
}

// denyJavaTypeText returns the type name (without type arguments) of a Java declaration or parameter, or "".
func denyJavaTypeText(file *scanner.File, decl uint32) string {
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			text := strings.Join(strings.Fields(file.FlatNodeText(c)), "")
			if i := strings.IndexByte(text, '<'); i >= 0 {
				text = text[:i]
			}
			return text
		}
	}
	return ""
}
