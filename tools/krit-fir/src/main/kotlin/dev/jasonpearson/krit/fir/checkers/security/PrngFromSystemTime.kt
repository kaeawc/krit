package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import java.io.File

/**
 * Port of the Go PrngFromSystemTime rule: a `java.util.Random` (or
 * `kotlin.random.Random`) created with a seed derived from the system clock,
 * in a file that imports `javax.crypto`, `java.security`, or `javax.net.ssl`.
 * Reported on the call expression, which starts where Go's call_expression
 * starts.
 *
 * Mirrored from Go:
 * - the call creates a `java.util.Random` (constructor), or a subclass of
 *   java.util.Random or kotlin.random.Random named Random, or calls a function
 *   named Random that returns one of them (`kotlin.random.Random(seed)`, or a
 *   project factory). Go takes any call spelled `Random(...)` once the file
 *   imports or mentions one of those FQNs;
 * - the seed is the argument bound to the first parameter (Go: the first
 *   unlabeled argument), so a clock read passed to another parameter is not
 *   the seed;
 * - the file imports a `javax.crypto`, `java.security`, or `javax.net.ssl`
 *   declaration (Go: the file text contains `import javax.crypto`, ...);
 * - Go's own path test, applied to the scan spelling of the path: skipped when
 *   the lower-cased path contains `/src/test/` or `/src/androidtest/` and not
 *   `/tests/fixtures/`. Go does not use scanner.IsTestFile here, so neither
 *   does the checker;
 * - the seed contains a system-clock read anywhere inside it (Go: a substring
 *   of the seed text): `System.currentTimeMillis()`, `System.nanoTime()`,
 *   `Date().time` / `Date().getTime()` (Go's text also matches a Date
 *   subclass whose name ends in Date, and a Joda-Time `toDate()` of the
 *   current instant, `DateTime().toDate().time`),
 *   `Calendar.getInstance().timeInMillis` / `.getTimeInMillis()` (the
 *   java.util, android.icu and ICU4J Calendars, and their subclasses), or
 *   `Instant.now().toEpochMilli()`, each with no arguments where Go's text has
 *   none. The walk enters lambdas and object literals inside the seed, as
 *   Go's text match does.
 *
 * Deliberate differences from Go, pinned by goldens and listed in the PR:
 * - Precision: each piece must resolve. The call must create a
 *   java.util.Random or kotlin.random.Random, so a local class or function
 *   named Random that builds neither is not reported; the clock read must
 *   resolve to java.lang.System, a java.util.Date built now, a java.util,
 *   android.icu or ICU4J Calendar, or a java.time or ThreeTen Instant, so a
 *   lookalike (`MySystem.nanoTime()`, a local `Date` class, a function named
 *   `startDate()`, `toDate()` of a parameter or of a fixed Joda instant) or a
 *   string literal in the seed is not reported, and `Date().timezoneOffset`
 *   (which Go's `Date().time` substring matches) is not a clock read; the
 *   security gate reads the import list, so a comment or string saying
 *   `import java.security` does not count.
 * - Recall: Go matches spelled text, so it misses an import alias or type
 *   alias of Random (including a typealias in another file), a Random
 *   subclass imported into a file that never mentions java.util.Random, an
 *   import alias of a clock owner (`import java.util.Date as D`), a
 *   statically imported clock function (aliased or not), a named seed
 *   argument (`Random(seed = ...)`), a clock read split across lines or
 *   parenthesized (`(Date()).time`), and a backticked `Random`.
 */
internal object PrngFromSystemTime : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "PrngFromSystemTime"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(PrngFromSystemTime)
    }

    private const val MESSAGE =
        "java.util.Random seeded from system time is predictable in security-sensitive code. " +
            "Use SecureRandom without a deterministic seed."

    private val javaUtilRandom = ClassId(FqName("java.util"), Name.identifier("Random"))
    private val randomClasses = setOf(javaUtilRandom, ClassId(FqName("kotlin.random"), Name.identifier("Random")))
    private val RANDOM = Name.identifier("Random")

    private val securityPackages = listOf("javax.crypto", "java.security", "javax.net.ssl")

    private val system = ClassId(FqName("java.lang"), Name.identifier("System"))
    private val systemClock = setOf(
        CallableId(system, Name.identifier("currentTimeMillis")),
        CallableId(system, Name.identifier("nanoTime")),
    )
    private val date = ClassId(FqName("java.util"), Name.identifier("Date"))
    private val calendars = setOf(
        ClassId(FqName("java.util"), Name.identifier("Calendar")),
        ClassId(FqName("android.icu.util"), Name.identifier("Calendar")),
        ClassId(FqName("com.ibm.icu.util"), Name.identifier("Calendar")),
    )
    private val GET_INSTANCE = Name.identifier("getInstance")
    private val instantNow = setOf(
        CallableId(ClassId(FqName("java.time"), Name.identifier("Instant")), Name.identifier("now")),
        CallableId(ClassId(FqName("org.threeten.bp"), Name.identifier("Instant")), Name.identifier("now")),
    )

    // Joda-Time: `toDate()` of a value built for the current instant.
    private val joda = FqName("org.joda.time")
    private val TO_DATE = Name.identifier("toDate")
    private val NOW = Name.identifier("now")

    // Arguments that keep a Joda constructor or now() at the current instant:
    // `DateTime(DateTimeZone.UTC)`, `DateTime.now(chronology)`.
    private val jodaZones = setOf(
        ClassId(joda, Name.identifier("DateTimeZone")),
        ClassId(joda, Name.identifier("Chronology")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (!createsSeedableRandom(expression)) return
        if (!importsSecurityPackage()) return
        if (isGoTestPath(containingScanPath())) return
        val seed = seedArgument(expression) ?: return
        if (!readsSystemClock(seed)) return

        report(expression.source, MESSAGE)
    }

    // Go reports any call spelled `Random(...)`, so a subclass of either
    // Random named Random, and a function named Random that returns either,
    // still count; another subclass or function name never does.
    context(context: CheckerContext)
    private fun createsSeedableRandom(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol()
        if (callee is FirNamedFunctionSymbol) {
            if (callee.name != RANDOM) return false
            val returned = classType(call) ?: return false
            return isOrExtendsAny(returned, randomClasses)
        }
        val constructed = constructedClass(call) ?: return false
        if (constructed.classId == javaUtilRandom) return true
        return constructed.classId?.shortClassName == RANDOM && extendsAny(constructed, randomClasses)
    }

    // The argument bound to the first parameter. Go reads the first unlabeled
    // argument, the same one unless the seed is named (`Random(seed = ...)`,
    // which Go misses).
    private fun seedArgument(call: FirFunctionCall): FirExpression? {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return null
        val first = callee.valueParameterSymbols.firstOrNull() ?: return null
        return call.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == first }?.key
    }

    // The class an expression evaluates to, type aliases expanded.
    context(context: CheckerContext)
    private fun classType(expression: FirExpression): ConeClassLikeType? =
        expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType

    // The class a constructor call creates.
    context(context: CheckerContext)
    private fun constructedClass(call: FirFunctionCall): ConeClassLikeType? {
        if (call.calleeReference.toResolvedCallableSymbol() !is FirConstructorSymbol) return null
        return classType(call)
    }

    // Supertypes come from the class's own lookup tag, which is bound for a
    // local class, so no class id is resolved through the symbol provider.
    context(context: CheckerContext)
    private fun extendsAny(type: ConeClassLikeType, targets: Set<ClassId>): Boolean {
        val symbol = type.lookupTag.toRegularClassSymbol(context.session) ?: return false
        return extendsAny(symbol, targets)
    }

    context(context: CheckerContext)
    private fun extendsAny(symbol: FirClassLikeSymbol<*>, targets: Set<ClassId>): Boolean =
        lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = context.session)
            .any { it.classId in targets }

    context(context: CheckerContext)
    private fun isOrExtendsAny(type: ConeClassLikeType, targets: Set<ClassId>): Boolean =
        type.classId in targets || extendsAny(type, targets)

    // Go tests the file text for `import javax.crypto` and the like; FIR reads
    // the import list, so aliases and star imports count and comments do not.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun importsSecurityPackage(): Boolean {
        val imports = context.containingFileSymbol?.fir?.imports ?: return false
        return imports.any { import ->
            val fqName = import.importedFqName?.asString() ?: return@any false
            securityPackages.any { fqName.startsWith(it) }
        }
    }

    // Go's prngFromSystemTimeTestPath, on the scan spelling of the path.
    private fun isGoTestPath(path: String?): Boolean {
        if (path == null) return false
        val normalized = path.replace(File.separatorChar, '/').lowercase()
        if ("/tests/fixtures/" in normalized) return false
        return "/src/test/" in normalized || "/src/androidtest/" in normalized
    }

    // Every qualified access inside the seed, lambdas and object literals
    // included, since Go matches the whole seed text.
    context(context: CheckerContext)
    private fun readsSystemClock(seed: FirExpression): Boolean {
        val accesses = ArrayList<FirQualifiedAccessExpression>()
        seed.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirQualifiedAccessExpression) accesses += element
                element.acceptChildren(this)
            }
        })
        return accesses.any { isClockRead(it) }
    }

    context(context: CheckerContext)
    private fun isClockRead(access: FirQualifiedAccessExpression): Boolean {
        val callee = access.calleeReference.toResolvedCallableSymbol() ?: return false
        return when (callee.name.asString()) {
            "currentTimeMillis", "nanoTime" ->
                access is FirFunctionCall && callee.unwrapFakeOverrides().callableId in systemClock
            "time" -> access is FirPropertyAccessExpression && isCurrentDate(access.explicitReceiver)
            "getTime" -> isNoArgCall(access) && isCurrentDate(access.explicitReceiver)
            "timeInMillis" -> access is FirPropertyAccessExpression && isCalendarGetInstance(access.explicitReceiver)
            "getTimeInMillis" -> isNoArgCall(access) && isCalendarGetInstance(access.explicitReceiver)
            "toEpochMilli" -> isNoArgCall(access) && isNoArgCallTo(access.explicitReceiver, instantNow)
            else -> false
        }
    }

    private fun isNoArgCall(access: FirQualifiedAccessExpression): Boolean =
        access is FirFunctionCall && access.argumentList.arguments.isEmpty()

    // A java.util.Date of the current instant: a no-argument Date constructor
    // call (Go matches the text `Date().time`, so a subclass whose name ends
    // in Date counts as well), or a Joda-Time `toDate()` of the current
    // instant, whose text `toDate().time` Go's `Date().time` also matches.
    context(context: CheckerContext)
    private fun isCurrentDate(receiver: FirExpression?): Boolean {
        val call = unwrap(receiver) as? FirFunctionCall ?: return false
        if (call.argumentList.arguments.isNotEmpty()) return false
        val constructed = constructedClass(call) ?: return isJodaToDateOfNow(call)
        if (constructed.classId == date) return true
        val name = constructed.classId?.shortClassName ?: return false
        return name.asString().endsWith("Date") && extendsAny(constructed, setOf(date))
    }

    // `toDate()` returning a java.util.Date, on a Joda-Time value built for
    // the current instant: a constructor or `now()` taking no arguments, or
    // only a time zone or chronology. Any other receiver (a parameter,
    // `DateTime(0L)`) is not the system time, though Go's text match reports
    // it too.
    context(context: CheckerContext)
    private fun isJodaToDateOfNow(toDate: FirFunctionCall): Boolean {
        val callee = toDate.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (callee.name != TO_DATE || classType(toDate)?.classId != date) return false
        val source = unwrap(toDate.explicitReceiver) as? FirFunctionCall ?: return false
        val owner = when (val built = source.calleeReference.toResolvedCallableSymbol()) {
            is FirConstructorSymbol -> constructedClass(source)?.classId
            is FirNamedFunctionSymbol -> if (built.name == NOW) built.unwrapFakeOverrides().callableId.classId else null
            else -> null
        } ?: return false
        if (owner.packageFqName != joda) return false
        return source.argumentList.arguments.all { argument ->
            val type = classType(argument)
            type != null && isOrExtendsAny(type, jodaZones)
        }
    }

    // A no-argument `getInstance()` of a known Calendar. A static inherited
    // through a subclass (GregorianCalendar.getInstance()) resolves to the
    // Calendar's own, and Go's `Calendar.getInstance()` text also matches a
    // subclass named ...Calendar that declares its own getInstance().
    context(context: CheckerContext)
    private fun isCalendarGetInstance(receiver: FirExpression?): Boolean {
        val call = unwrap(receiver) as? FirFunctionCall ?: return false
        if (call.argumentList.arguments.isNotEmpty()) return false
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        if (callee.name != GET_INSTANCE) return false
        val original = callee.unwrapFakeOverrides()
        if (original.callableId.classId in calendars) return true
        val owner = original.getContainingClassSymbol() ?: return false
        return owner.classId.shortClassName.asString().endsWith("Calendar") && extendsAny(owner, calendars)
    }

    private fun isNoArgCallTo(receiver: FirExpression?, ids: Set<CallableId>): Boolean {
        val call = unwrap(receiver) as? FirFunctionCall ?: return false
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        return callee.unwrapFakeOverrides().callableId in ids && call.argumentList.arguments.isEmpty()
    }

    private fun unwrap(expression: FirExpression?): FirExpression? =
        if (expression is FirWrappedArgumentExpression) expression.expression else expression
}
