package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirAnnotationContainer
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.getContainingClassSymbol
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

/**
 * Flags a `RecyclerView.Adapter` subclass that calls `notifyDataSetChanged()` and never mentions
 * `DiffUtil` or `ListAdapter`, reported on the first such call that is not suppressed with
 * `@SuppressLint("NotifyDataSetChanged")`.
 *
 * Mirrors the Go rule: a class (or, here, an object) whose superclass closure holds
 * `RecyclerView.Adapter`, with no `ListAdapter` in it, is searched as a whole, nested classes and
 * lambdas included. The call is the first one in source order. Any written reference to a class
 * named `DiffUtil` or `ListAdapter` inside the class (a type, a type argument, a qualifier such as
 * `DiffUtil.calculateDiff`) clears it. Test files are skipped. Java classes stay with Go.
 *
 * Deliberate differences from Go, each pinned in the golden data (`RecyclerAdapterWithoutDiffUtil*.kt`):
 * - Go needs a direct supertype whose name ends in `Adapter`, or the resolver's hierarchy for such a
 *   name. The closure here also sees an adapter reached through a base class of any name, and an
 *   adapter that is an `object`, which Go does not visit.
 * - Go matches the call by name. Here it must resolve to a member of `RecyclerView.Adapter`, so a
 *   `notifyDataSetChanged` on another adapter type (`BaseAdapter` for a `ListView`) or a same-named
 *   function is not reported.
 * - Go needs the direct supertype to be named `ListAdapter`. Here a subclass of a class that extends
 *   `ListAdapter` is exempt too.
 */
internal object RecyclerAdapterWithoutDiffUtil : FirClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "RecyclerAdapterWithoutDiffUtil"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(RecyclerAdapterWithoutDiffUtil)
    }

    private val notifyDataSetChanged = Name.identifier("notifyDataSetChanged")
    private val adapterClassIds = setOf(
        classIdOf("androidx.recyclerview.widget", "RecyclerView").createNestedClassId(Name.identifier("Adapter")),
        classIdOf("android.support.v7.widget", "RecyclerView").createNestedClassId(Name.identifier("Adapter")),
    )

    // Go matches these two names anywhere in the class, whatever the package.
    private val clearingNames = setOf("DiffUtil", "ListAdapter")
    private const val LIST_ADAPTER = "ListAdapter"
    private const val SUPPRESSED_ID = "\"NotifyDataSetChanged\""

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirClass) {
        if (declaration !is FirRegularClass) return
        if (declaration.classKind != ClassKind.CLASS && declaration.classKind != ClassKind.OBJECT) return
        if (declaration.source?.kind !is KtRealSourceElementKind) return
        val session = context.session
        val closure = superClosure(declaration.symbol, session)
        if (closure.none { it in adapterClassIds }) return
        if (closure.any { it.shortClassName.asString() == LIST_ADAPTER }) return
        if (isInTestFile()) return
        if (suppresses(declaration) || context.containingElements.any(::suppresses)) return
        val scan = Scan(session)
        declaration.acceptChildren(scan)
        if (scan.mentionsClearingName) return
        val callee = scan.firstCall?.calleeReference?.source ?: return
        report(
            callee,
            "'${declaration.name.asString()}' uses notifyDataSetChanged() without DiffUtil; " +
                "use ListAdapter or DiffUtil.calculateDiff() for efficient updates.",
        )
    }

    // Class ids of every superclass, read from lookup tags so a local class is safe.
    private fun superClosure(symbol: FirClassSymbol<*>, session: FirSession): List<ClassId> =
        lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = session)
            .map { it.lookupTag.classId }

    private fun suppresses(element: FirElement): Boolean {
        val annotations = (element as? FirAnnotationContainer)?.annotations ?: return false
        return annotations.any { annotation ->
            val text = annotation.source?.text?.toString() ?: return@any false
            "Suppress" in text && SUPPRESSED_ID in text
        }
    }

    private fun namesClearingClass(classId: ClassId?): Boolean =
        classId?.outermostClassId?.shortClassName?.asString() in clearingNames

    private fun namesClearingClass(type: ConeKotlinType, session: FirSession): Boolean {
        val expanded = type.fullyExpandedType(session) as? ConeClassLikeType ?: return false
        return namesClearingClass(expanded.lookupTag.classId) ||
            expanded.typeArguments.any { argument -> argument.type?.let { namesClearingClass(it, session) } == true }
    }

    // One walk over the class body: the first live call and whether a clearing name is written.
    private class Scan(private val session: FirSession) : FirVisitorVoid() {
        var firstCall: FirFunctionCall? = null
        var mentionsClearingName = false
        private var suppressedDepth = 0

        override fun visitElement(element: FirElement) {
            val suppressed = suppresses(element)
            if (suppressed) suppressedDepth++
            inspect(element)
            element.acceptChildren(this)
            if (suppressed) suppressedDepth--
        }

        private fun inspect(element: FirElement) {
            when (element) {
                is FirFunctionCall -> inspectCall(element)
                is FirResolvedQualifier -> if (namesClearingClass(element.qualifierSymbol?.classId)) mentionsClearingName = true
                is FirResolvedTypeRef ->
                    if (element.source?.kind is KtRealSourceElementKind && namesClearingClass(element.coneType, session)) {
                        mentionsClearingName = true
                    }
            }
            if (element is FirQualifiedAccessExpression && !mentionsClearingName) {
                val owner = element.calleeReference.toResolvedCallableSymbol()?.callableId?.classId
                if (namesClearingClass(owner)) mentionsClearingName = true
            }
        }

        private fun inspectCall(call: FirFunctionCall) {
            if (suppressedDepth > 0 || call.calleeReference.name != notifyDataSetChanged) return
            val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
            val owner = callee.getContainingClassSymbol() as? FirClassSymbol<*> ?: return
            if (owner.classId !in adapterClassIds && superClosure(owner, session).none { it in adapterClassIds }) return
            val start = call.calleeReference.source?.startOffset ?: return
            val current = firstCall?.calleeReference?.source?.startOffset
            if (current == null || start < current) firstCall = call
        }
    }
}
