package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.types.upperBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go StateFlowMutableLeak rule: a publicly visible,
 * non-override property whose type exposes kotlinx.coroutines.flow.MutableStateFlow
 * (directly, nullable, through a type alias, or as a type argument such as
 * `List<MutableStateFlow<T>>`), reported on the property's first line (its
 * modifier list, or `val`/`var` when it has none), like Go.
 *
 * Covered like Go: class, object, companion, interface, and top-level
 * properties, explicit or inferred MutableStateFlow types, delegated and
 * getter-backed properties, `private set` (the getter stays public), and
 * properties of non-public classes (Go checks only the property's own
 * modifiers). Skipped like Go: private/protected/internal and override
 * properties, primary-constructor `val` parameters, anything declared inside
 * a function, getter, setter, or constructor body, and test source files.
 *
 * Deliberate precision differences from Go's substring match on the property
 * text (see the golden data): a property whose declared type is read-only is
 * not reported just because its initializer mentions MutableStateFlow
 * (`val s: StateFlow<Int> = MutableStateFlow(0)`,
 * `val s = MutableStateFlow(0).asStateFlow()`), nor because only its name,
 * a comment, or a string contains the word; locals and members of anonymous
 * objects or local classes declared in an init block or a property-initializer
 * lambda are not exposed; and a MutableStateFlow type the source does not spell
 * out (an inferred call result, an import alias, a type alias) is reported.
 */
internal object StateFlowMutableLeak : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "StateFlowMutableLeak"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(StateFlowMutableLeak)
    }

    private val mutableStateFlow = ClassId(FqName("kotlinx.coroutines.flow"), Name.identifier("MutableStateFlow"))

    // scanner.defaultTestPaths plus the Gradle test source-set check in
    // internal/scanner/testpath.go: Go skips these files for this rule.
    private val testPathMarkers = listOf(
        "/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/jvmAndroidTest/",
        "/commonJvmTest/", "/browserCommonTest/", "/jvmCommonTest/",
        "/androidUnitTest/", "/androidInstrumentedTest/", "/jsTest/", "/iosTest/",
        "/nativeTest/", "/nonJvmCommonTest/",
        "/testShared/", "/sharedTest/",
        "/benchmark/", "/canary/",
        "/test-utils/",
        "/javatests/", "/kotlintests/", "/javatest/", "/kotlintest/",
        "/functionalTest/", "/functionaltests/",
        "/test/resources/", "/testResources/", "/testFixtures/",
        "/integration-tests/", "/integrationTest/",
        "/nonEmulatorCommonTest/", "/nonEmulatorJvmTest/",
        "/testData/", "/testdata/", "/test-data/",
        "/test/data/", "/compiler-tests/", "/compilertests/",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        if (declaration.isLocal) return
        val source = declaration.source ?: return
        // Primary-constructor `val`/`var` parameters are class parameters in Go,
        // not property declarations.
        if (source.kind is KtFakeSourceElementKind) return
        if (!isExposedContainer(declaration)) return
        if (declaration.isOverride) return
        val visibility = declaration.visibility
        if (visibility == Visibilities.Private || visibility == Visibilities.Protected ||
            visibility == Visibilities.Internal || visibility == Visibilities.PrivateToThis
        ) return
        if (!mentionsMutableStateFlow(declaration.returnTypeRef.coneType, context.session, depth = 0)) return
        if (isTestFile(context.containingFile?.path)) return

        report(source, "MutableStateFlow '${declaration.name.asString()}' is publicly exposed. Keep it private and expose as StateFlow<T>.")
    }

    // Top-level properties and members of named, non-local classes and objects.
    // Anything nested in a function, accessor, constructor, init block, lambda,
    // anonymous object, or local class is not part of the declaring API.
    context(context: CheckerContext)
    private fun isExposedContainer(declaration: FirProperty): Boolean =
        context.containingDeclarations.all { symbol ->
            symbol == declaration.symbol ||
                symbol is FirFileSymbol ||
                (symbol is FirRegularClassSymbol && symbol.resolvedStatus.visibility != Visibilities.Local)
        }

    private fun mentionsMutableStateFlow(type: ConeKotlinType, session: FirSession, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return false
        val expanded = type.fullyExpandedType(session)
        val bounds = listOf(expanded.lowerBoundIfFlexible(), expanded.upperBoundIfFlexible()).distinct()
        return bounds.any { bound ->
            bound.classId == mutableStateFlow ||
                bound.typeArguments.any { argument ->
                    val argumentType = argument.type ?: return@any false
                    mentionsMutableStateFlow(argumentType, session, depth + 1)
                }
        }
    }

    private fun isTestFile(path: String?): Boolean {
        if (path == null) return false
        val slash = path.replace('\\', '/')
        if (testPathMarkers.any { slash.contains(it) }) return true
        return isGradleTestSourceSet(slash)
    }

    private fun isGradleTestSourceSet(slash: String): Boolean {
        fun isTestSegment(segment: String) = segment == "test" || segment.endsWith("Test")
        if (slash.startsWith("src/") && isTestSegment(slash.removePrefix("src/").substringBefore('/'))) return true
        var offset = 0
        while (true) {
            val index = slash.indexOf("/src/", offset)
            if (index < 0) return false
            val start = index + "/src/".length
            if (isTestSegment(slash.substring(start).substringBefore('/'))) return true
            offset = start
        }
    }

    private const val MAX_TYPE_DEPTH = 16
}
