package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.references.FirResolvedNamedReference
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirEnumEntrySymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeFlexibleType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.FirResolvedTypeRef
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId

// Flags a private class, interface or enum that nothing in its file refers to, with the Go rule's scope
// and message (`Private class C is unused.`, on the class's name).
//
// Go looks for the name anywhere else in the file, so a same-named class of another package, a member
// of that name, a comment or a string keeps it quiet. This checker reads resolution instead: the class
// is used when some resolved reference in the file (a type, a constructor call, a qualifier such as
// `C.Companion` or `C::class`, a supertype, an annotation, an enum entry) names its class id. A private
// class is visible only in its file, so the file holds every use. A mention inside the class's own text
// does not count, like Go. Objects and companion objects are separate declarations in Go and not judged.
internal object UnusedPrivateClass : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnusedPrivateClass"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(UnusedPrivateClass)
    }

    private val judgedKinds = setOf(ClassKind.CLASS, ClassKind.INTERFACE, ClassKind.ENUM_CLASS, ClassKind.ANNOTATION_CLASS)

    private class Candidate(val name: String, val classId: ClassId, val anchor: KtSourceElement)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        val candidates = ArrayList<Candidate>()
        val referenced = HashSet<ClassId>()
        // The classes whose own text is being walked: Go ignores a name inside the declaration itself.
        val inside = HashSet<ClassId>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    is FirResolvedTypeRef -> addType(element.coneType, inside, referenced)
                    is FirResolvedQualifier -> element.qualifierSymbol?.classId?.let { addClass(it, inside, referenced) }
                    is FirResolvedNamedReference -> addSymbol(element, inside, referenced)
                    is FirRegularClass -> {
                        val candidate = candidateOf(element)
                        if (candidate != null) {
                            candidates += candidate
                            inside += candidate.classId
                            element.acceptChildren(this)
                            inside -= candidate.classId
                            return
                        }
                    }
                }
                element.acceptChildren(this)
            }
        })
        for (candidate in candidates) {
            if (candidate.classId !in referenced) report(candidate.anchor, "Private class ${candidate.name} is unused.")
        }
    }

    // A nested class is reached through its enclosing classes (`Outer.Inner()`), so they count as used too.
    private fun addClass(classId: ClassId, inside: Set<ClassId>, referenced: MutableSet<ClassId>) {
        var current: ClassId? = classId
        while (current != null) {
            if (current !in inside) referenced += current
            current = current.outerClassId
        }
    }

    // A constructor call or an enum entry names its class even when the class is not spelled at the use.
    private fun addSymbol(reference: FirResolvedNamedReference, inside: Set<ClassId>, referenced: MutableSet<ClassId>) {
        val symbol = reference.resolvedSymbol
        val owner = when (symbol) {
            is FirConstructorSymbol -> symbol.callableId.classId
            is FirEnumEntrySymbol -> symbol.callableId.classId
            else -> null
        }
        if (owner != null) addClass(owner, inside, referenced)
    }

    private fun addType(type: ConeKotlinType, inside: Set<ClassId>, referenced: MutableSet<ClassId>) {
        when (type) {
            is ConeClassLikeType -> {
                addClass(type.lookupTag.classId, inside, referenced)
                type.typeArguments.forEach { argument -> argument.type?.let { addType(it, inside, referenced) } }
            }
            is ConeFlexibleType -> {
                addType(type.lowerBound, inside, referenced)
                addType(type.upperBound, inside, referenced)
            }
            is ConeDefinitelyNotNullType -> addType(type.original, inside, referenced)
            is ConeIntersectionType -> type.intersectedTypes.forEach { addType(it, inside, referenced) }
            else -> Unit
        }
    }

    private fun candidateOf(declaration: FirRegularClass): Candidate? {
        if (declaration.visibility != Visibilities.Private || declaration.classKind !in judgedKinds) return null
        if (declaration.symbol.classId.isLocal) return null
        val source = declaration.source ?: return null
        if (source.kind !is KtRealSourceElementKind || source.elementType != KtNodeTypes.CLASS) return null
        val identifier = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        val name = lightText(source, identifier).trim('`')
        if (name.isEmpty()) return null
        return Candidate(name, declaration.symbol.classId, lightSourceOf(identifier, source))
    }
}
