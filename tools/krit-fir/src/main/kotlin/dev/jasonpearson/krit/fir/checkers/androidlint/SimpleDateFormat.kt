package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a SimpleDateFormat constructor call that passes fewer than two
// arguments: `SimpleDateFormat()` and `SimpleDateFormat(pattern)` format with
// the default locale. java.text.SimpleDateFormat, ICU4J's
// com.ibm.icu.text.SimpleDateFormat, and android.icu.text.SimpleDateFormat
// (Android's repackaged ICU4J) all count; their one- and zero-argument
// constructors use the default (format) locale. A project class that extends
// one of them counts too when it, or the call, is named SimpleDateFormat (a
// typealias or import alias): it still builds a SimpleDateFormat whose locale
// the call does not state, and Go reports the call by its name. A subclass
// called by another name (`class RootFormat : SimpleDateFormat`) is not
// reported, like Go.
//
// Like the Go rule:
// - any call with two or more arguments is accepted, whatever the second
//   argument is (`SimpleDateFormat(pattern, DateFormatSymbols)` included);
// - only constructor calls count. A superclass delegation
//   (`class F : SimpleDateFormat("x")`, `object : SimpleDateFormat("x") {}`,
//   `constructor() : super("x")`) and a constructor reference
//   (`::SimpleDateFormat`) are not call expressions of SimpleDateFormat, and
//   Go does not report them either;
// - the finding sits on the first line of the call expression, including the
//   package qualifier of a fully qualified call.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Recall: the constructor is identified by resolution, so an import alias
//   (`import java.text.SimpleDateFormat as Sdf`), a typealias, and a
//   backticked name are reported. Go needs a call whose callee is spelled
//   SimpleDateFormat.
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
    }

    private const val MESSAGE =
        "SimpleDateFormat without explicit Locale. Use SimpleDateFormat(pattern, Locale) to avoid locale bugs."

    private val simpleDateFormatClassIds = setOf(
        ClassId(FqName("java.text"), Name.identifier("SimpleDateFormat")),
        ClassId(FqName("android.icu.text"), Name.identifier("SimpleDateFormat")),
        ClassId(FqName("com.ibm.icu.text"), Name.identifier("SimpleDateFormat")),
    )

    private val simpleDateFormatName = Name.identifier("SimpleDateFormat")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        if (expression.argumentList.arguments.size >= 2) return
        val constructor = expression.calleeReference.toResolvedCallableSymbol() as? FirConstructorSymbol ?: return
        // The return type names the constructed class; for a typealias
        // constructor it is expanded to the aliased class.
        val constructed = constructor.resolvedReturnType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType
            ?: return
        val classId = constructed.lookupTag.classId
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

    // A class named SimpleDateFormat that is not one of the known classes may
    // still subclass one. The class comes from the constructor's containing
    // class lookup tag, which is bound to local classes; looking a local class
    // up by class id throws. Only class ids already in hand are compared.
    context(context: CheckerContext)
    private fun extendsSimpleDateFormat(constructor: FirConstructorSymbol): Boolean {
        val owner = constructor.getContainingClassSymbol() ?: return false
        return lookupSuperTypes(owner, lookupInterfaces = false, deep = true, useSiteSession = context.session)
            .any { it.lookupTag.classId in simpleDateFormatClassIds }
    }
}
