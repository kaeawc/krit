package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.inheritsFromAny
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirExpressionChecker
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirCall
import org.jetbrains.kotlin.fir.expressions.FirDelegatedConstructorCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedConstructorSymbol
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name

// Flags a SimpleDateFormat constructor call that does not take a locale, as
// Android Lint's DateFormatDetector does: `SimpleDateFormat()`,
// `SimpleDateFormat(pattern)`, and `SimpleDateFormat(pattern, symbols)` format
// with the default locale (the calendar and digits come from it even when the
// symbols do not). java.text.SimpleDateFormat, ICU4J's
// com.ibm.icu.text.SimpleDateFormat, and android.icu.text.SimpleDateFormat
// (Android's repackaged ICU4J) all count; a constructor that declares a
// java.util.Locale or an ICU ULocale parameter states the locale. A project
// class that extends one of them counts too when it, or the call, is named
// SimpleDateFormat (a typealias or import alias): it still builds a
// SimpleDateFormat whose locale the call does not state, and Go reports the
// call by its name. A subclass called by another name
// (`class RootFormat : SimpleDateFormat`) is not reported, like Go and Lint.
//
// Like the Go rule:
// - a constructor reference (`::SimpleDateFormat`) is not a call and is not
//   reported;
// - the finding sits on the first line of the call expression, including the
//   package qualifier of a fully qualified call.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the constructor is identified by resolution, so an import alias
//   (`import java.text.SimpleDateFormat as Sdf`), a typealias, and a
//   backticked name are reported. Go needs a call whose callee is spelled
//   SimpleDateFormat.
// - Recall (SimpleDateFormatLocale): as in Lint, a constructor without a
//   locale parameter is reported whatever its argument
//   (`SimpleDateFormat(pattern, symbols)`; Go sees only symbols built in the
//   argument), and so is a superclass delegation to one
//   (`class F : SimpleDateFormat("x")`, `object : SimpleDateFormat("x") {}`,
//   `constructor() : super("x")`), on the delegation call. Go reads only call
//   expressions.
// - Precision: a call named SimpleDateFormat that does not construct a
//   SimpleDateFormat is not reported: a project, local, or nested class named
//   SimpleDateFormat that is neither one of the three classes above nor a
//   subtype of one, a function or member function, a lambda-typed local, or a
//   member called on a receiver. Go reports every call spelled
//   SimpleDateFormat with fewer than two arguments.
internal object SimpleDateFormat : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SimpleDateFormat"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SimpleDateFormat)
        override val callCheckers = setOf(Delegations)
    }

    private const val MESSAGE =
        "To get local formatting use `getDateInstance()`, `getDateTimeInstance()`, or `getTimeInstance()`, or use `new SimpleDateFormat(String template, Locale locale)` with for example `Locale.US` for ASCII dates."

    private val simpleDateFormatClassIds = setOf(
        classIdOf("java.text", "SimpleDateFormat"),
        classIdOf("android.icu.text", "SimpleDateFormat"),
        classIdOf("com.ibm.icu.text", "SimpleDateFormat"),
    )

    private val localeClassIds = setOf(
        classIdOf("java.util", "Locale"),
        classIdOf("android.icu.util", "ULocale"),
        classIdOf("com.ibm.icu.util", "ULocale"),
    )

    private val simpleDateFormatName = Name.identifier("SimpleDateFormat")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val constructor = expression.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
        if (specifiesLocale(constructor)) return
        // The return type names the constructed class; for a typealias
        // constructor it is expanded to the aliased class.
        val classId = constructedClassId(constructor) ?: return
        if (classId !in simpleDateFormatClassIds) {
            // A subclass counts when either the class or the call is named
            // SimpleDateFormat: Go matches the written call name, which a
            // typealias or import alias can give to a subclass named otherwise.
            val named = classId.shortClassName == simpleDateFormatName ||
                expression.calleeReference.name == simpleDateFormatName
            if (!named || !extendsSimpleDateFormat(constructor)) return
        }
        report(expression.source, MESSAGE)
    }

    // A superclass delegation to a SimpleDateFormat constructor without a
    // locale: the subclass formats with the default locale. A delegation the
    // source does not write (a secondary constructor with no `: super(..)`)
    // has no call to report.
    private object Delegations : FirExpressionChecker<FirCall>(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirCall) {
            val call = expression as? FirDelegatedConstructorCall ?: return
            val source = call.source ?: return
            if (source.kind is KtFakeSourceElementKind) return
            val constructor = call.calleeReference.toResolvedConstructorSymbol() ?: return
            if (specifiesLocale(constructor)) return
            if (constructedClassId(constructor) !in simpleDateFormatClassIds) return
            SimpleDateFormat.report(source, MESSAGE)
        }
    }

    context(context: CheckerContext)
    private fun specifiesLocale(constructor: FirConstructorSymbol): Boolean =
        constructor.valueParameterSymbols.any { it.resolvedReturnType.expandedClassType()?.lookupTag?.classId in localeClassIds }

    context(context: CheckerContext)
    private fun constructedClassId(constructor: FirConstructorSymbol): ClassId? =
        constructor.resolvedReturnType.expandedClassType()?.lookupTag?.classId

    // A class named SimpleDateFormat that is not one of the known classes may
    // still subclass one. The class comes from the constructor's containing
    // class lookup tag, which is bound to local classes; looking a local class
    // up by class id throws. Only class ids already in hand are compared.
    context(context: CheckerContext)
    private fun extendsSimpleDateFormat(constructor: FirConstructorSymbol): Boolean {
        val owner = constructor.getContainingClassSymbol() ?: return false
        return inheritsFromAny(owner, simpleDateFormatClassIds, context.session, lookupInterfaces = false)
    }
}
