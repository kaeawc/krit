package dev.jasonpearson.krit.fir.checkers.androidlint

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.firstModifierAnchor
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.analysis.getChild
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isInner
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go HandlerLeak rule (Android Lint `HandlerLeak`), Kotlin side:
 * - an `inner` class that is an `android.os.Handler`, reported on the
 *   declaration's first line (its first annotation or modifier), like Go,
 *   unless its primary constructor takes a `Looper`-typed parameter that it
 *   passes to the `Handler(...)` superclass constructor;
 * - an anonymous object (`object : Handler() { ... }`) that is an
 *   `android.os.Handler`, reported on its `object` keyword, like Go. As in
 *   Go, every anonymous Handler is reported, whatever it is constructed with
 *   and wherever it is declared.
 *
 * As in Go, the Looper exemption reads the source: a primary constructor
 * parameter counts when a type named `Looper` is written anywhere in it, the
 * superclass call counts when it is written as `Handler(...)` (last name
 * segment), and the parameter is passed when an identifier spelled like it
 * appears in that call's arguments (outside a short `$name` template
 * entry). A class without a primary constructor takes the Looper parameters
 * of every primary constructor in the file, because Go walks the whole file
 * when it finds none. Secondary constructors are not considered.
 *
 * A class or object is a Handler when `android.os.Handler` is in its
 * superclass closure.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`HandlerLeak*.kt`):
 * - With `import android.os.*`, Go treats every supertype name it cannot
 *   match to an explicit import as `android.os.Handler`, so it reports inner
 *   classes and anonymous objects that only implement `Runnable` or extend a
 *   class declared in the file. Those are not Handlers and are not reported here.
 * - Go resolves only the supertype's last name segment through the file's
 *   imports, so it misses a fully qualified `android.os.Handler` supertype, a
 *   type alias, and a subclass of a Handler base class (top-level, nested, or
 *   local). Those are inner or anonymous Handlers and are reported here.
 * - Go resolves the supertype's last name segment through an explicit
 *   `import android.os.Handler` even when a nested class named `Handler`
 *   shadows the import or the supertype is qualified (`Foo.Handler()`).
 *   Those supertypes are not android.os.Handler and are not reported here.
 */
internal object HandlerLeak : FirClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "HandlerLeak"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(HandlerLeak)
    }

    private const val INNER_MESSAGE =
        "This Handler class should be static or leaks might occur. Use a WeakReference to the outer class."
    private const val ANONYMOUS_MESSAGE =
        "Anonymous Handler may leak the enclosing class. Use a static inner class with a WeakReference."

    private val handlerClassId = ClassId(FqName("android.os"), Name.identifier("Handler"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirClass) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        when (declaration) {
            is FirAnonymousObject -> {
                if (!isHandler(declaration, context.session)) return
                report(source.getChild(KtTokens.OBJECT_KEYWORD, depth = 1) ?: source, ANONYMOUS_MESSAGE)
            }
            is FirRegularClass -> {
                if (declaration.classKind != ClassKind.CLASS || !declaration.isInner) return
                if (!isHandler(declaration, context.session)) return
                if (passesLooperToHandler(source)) return
                // An inner class always has a modifier list (`inner`); Go
                // reports its first line, where the first annotation or
                // modifier stands.
                report(firstModifierAnchor(source) ?: source, INNER_MESSAGE)
            }
        }
    }

    // Supertypes are read from their own lookup tags, so no class id is
    // resolved from a symbol that may be local.
    private fun isHandler(declaration: FirClass, session: FirSession): Boolean {
        for (ref in declaration.superTypeRefs) {
            val type = ref.coneType.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: continue
            if (type.lookupTag.classId == handlerClassId) return true
            val symbol = type.lookupTag.toClassSymbol(session) ?: continue
            val extendsHandler = lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = session)
                .any { it.lookupTag.classId == handlerClassId }
            if (extendsHandler) return true
        }
        return false
    }

    // Go's exemption, read from the source as Go reads it: a primary
    // constructor parameter whose text holds a type named `Looper`, whose
    // name appears among the arguments of a superclass call written as
    // `Handler(...)` (its last type name segment).
    private fun passesLooperToHandler(source: KtSourceElement): Boolean {
        val root = source.lighterASTNode
        val looperParams = HashSet<String>()
        val ctor = lightChildren(source, root).firstOrNull { it.tokenType == KtNodeTypes.PRIMARY_CONSTRUCTOR }
        if (ctor != null) {
            collectLooperParams(source, ctor, looperParams)
        } else {
            // Go looks the primary constructor up as a child of the class and
            // walks from the file root when there is none, so a class without
            // one takes the Looper parameters of every primary constructor in
            // the file (an outer class's `looper: Looper` included).
            collectFileLooperParams(source, source.treeStructure.root, looperParams)
        }
        if (looperParams.isEmpty()) return false
        val superTypes = lightChildren(source, root).firstOrNull { it.tokenType == KtNodeTypes.SUPER_TYPE_LIST }
            ?: return false
        return lightChildren(source, superTypes).any { entry ->
            entry.tokenType == KtNodeTypes.SUPER_TYPE_CALL_ENTRY &&
                superCallNamesHandler(source, entry) &&
                lightChildren(source, entry).any {
                    it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST && usesAnyName(source, it, looperParams)
                }
        }
    }

    // The Looper-typed parameters of the primary constructor [ctor].
    private fun collectLooperParams(source: KtSourceElement, ctor: LighterASTNode, into: MutableSet<String>) {
        val params = lightChildren(source, ctor).firstOrNull { it.tokenType == KtNodeTypes.VALUE_PARAMETER_LIST } ?: return
        for (param in lightChildren(source, params)) {
            if (param.tokenType != KtNodeTypes.VALUE_PARAMETER) continue
            val name = lightChildren(source, param).firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: continue
            if (containsLooperType(source, param)) into += source.treeStructure.toString(name).toString()
        }
    }

    // The Looper-typed parameters of every primary constructor under [node].
    private fun collectFileLooperParams(source: KtSourceElement, node: LighterASTNode, into: MutableSet<String>) {
        if (node.tokenType == KtNodeTypes.PRIMARY_CONSTRUCTOR) collectLooperParams(source, node, into)
        for (child in lightChildren(source, node)) collectFileLooperParams(source, child, into)
    }

    // A type name segment `Looper` anywhere in [node]'s subtree.
    private fun containsLooperType(source: KtSourceElement, node: LighterASTNode): Boolean {
        if (node.tokenType == KtNodeTypes.USER_TYPE) {
            val segment = lightChildren(source, node).lastOrNull { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION }
            if (segment != null && source.treeStructure.toString(segment).toString() == "Looper") return true
        }
        return lightChildren(source, node).any { containsLooperType(source, it) }
    }

    // The superclass call's type as written ends in the segment `Handler`.
    private fun superCallNamesHandler(source: KtSourceElement, entry: LighterASTNode): Boolean {
        val callee = lightChildren(source, entry).firstOrNull { it.tokenType == KtNodeTypes.CONSTRUCTOR_CALLEE }
            ?: return false
        val typeRef = lightChildren(source, callee).firstOrNull { it.tokenType == KtNodeTypes.TYPE_REFERENCE }
            ?: return false
        val userType = lightChildren(source, typeRef).firstOrNull { it.tokenType == KtNodeTypes.USER_TYPE }
            ?: return false
        val segment = lightChildren(source, userType).lastOrNull { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION }
            ?: return false
        return source.treeStructure.toString(segment).toString() == "Handler"
    }

    // An identifier spelled like one of [names] anywhere in [node]'s subtree.
    // Go's tree-sitter reads a short template entry (`"$looper"`) as an
    // interpolated identifier, which it does not count; `"${looper}"` holds an
    // ordinary identifier and counts in both.
    private fun usesAnyName(source: KtSourceElement, node: LighterASTNode, names: Set<String>): Boolean {
        if (node.tokenType == KtTokens.IDENTIFIER) return source.treeStructure.toString(node).toString() in names
        if (node.tokenType == KtNodeTypes.SHORT_STRING_TEMPLATE_ENTRY) return false
        return lightChildren(source, node).any { usesAnyName(source, it, names) }
    }
}
