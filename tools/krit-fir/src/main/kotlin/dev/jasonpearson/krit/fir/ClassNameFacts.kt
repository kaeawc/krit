package dev.jasonpearson.krit.fir

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.types.FirTypeRef
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens

/**
 * One class of a source file as Go's name-based resolver sees it: the class's simple name and the identifiers
 * written in its supertype references. Used by SerialVersionUIDInSerializableClass, and recorded per file by the
 * incremental state so a slice compile can supply the facts of the files it does not compile from source.
 */
data class ClassNameFact(val name: String, val supertypeNames: Set<String>)

/**
 * The [ClassNameFact] of every class in [file], nested and local classes included, in source order. This is the
 * single extraction both the rule (on the files it compiles) and the recorder use, so the two cannot disagree.
 */
fun classNameFacts(file: FirFile): List<ClassNameFact> {
    val out = ArrayList<ClassNameFact>()
    file.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            element.acceptChildren(this)
        }

        override fun visitRegularClass(regularClass: FirRegularClass) {
            val names = HashSet<String>()
            for (ref in regularClass.superTypeRefs) supertypeNames(ref, names)
            out += ClassNameFact(regularClass.name.asString(), names)
            regularClass.acceptChildren(this)
        }
    })
    return out
}

/**
 * The identifiers written in a supertype reference, backticks removed. An implicit supertype (`Any`, an enum's
 * `Enum`) has no written name.
 */
fun supertypeNames(ref: FirTypeRef, out: MutableSet<String>) {
    val source = ref.source ?: return
    if (source.kind !is KtRealSourceElementKind) return
    fun walk(node: LighterASTNode) {
        if (node.tokenType == KtTokens.IDENTIFIER) {
            out += source.treeStructure.toString(node).toString().removeSurrounding("`")
            return
        }
        for (child in lightChildren(source, node)) walk(child)
    }
    walk(source.lighterASTNode)
}

/**
 * The simple names that reach `Serializable` by supertype names: `Serializable` itself, and every class whose
 * supertypes name a name already reached, to a fixed point. Order- and file-independent.
 */
fun serializableReachedNames(facts: Iterable<ClassNameFact>): Set<String> {
    val bySupertype = HashMap<String, MutableList<String>>()
    for (fact in facts) for (s in fact.supertypeNames) bySupertype.getOrPut(s) { ArrayList() }.add(fact.name)
    val reached = hashSetOf(SERIALIZABLE_NAME)
    val queue = ArrayDeque(listOf(SERIALIZABLE_NAME))
    while (queue.isNotEmpty()) {
        for (sub in bySupertype[queue.removeFirst()].orEmpty()) if (reached.add(sub)) queue.addLast(sub)
    }
    return reached
}

const val SERIALIZABLE_NAME = "Serializable"
