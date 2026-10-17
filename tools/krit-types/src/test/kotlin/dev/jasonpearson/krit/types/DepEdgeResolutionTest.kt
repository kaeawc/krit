package dev.jasonpearson.krit.types

import com.intellij.openapi.util.Disposer
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.psi.KtFile
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.createTempDirectory
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Pins which source files each kind of reference makes a dependency of the
 * using file (DepEdges.kt): the dependency closure the oracle cache
 * fingerprints. Every operator convention below is declared as an extension
 * in its own file, with its receiver type in Rx.kt, so the operator's file
 * is an edge only if the reference to the operator itself was resolved.
 *
 * Dependency edges are candidate-tolerant: a reference that failed to
 * resolve still records the candidate declarations, because the declaration
 * is exactly what an edit would change into a successful resolution.
 */
@OptIn(KaExperimentalApi::class)
class DepEdgeResolutionTest {
    private fun edgesOf(useFile: String): Set<String> {
        val use = module.psiRoots.filterIsInstance<KtFile>().first { it.name == useFile }
        val tracker = DepTracker()
        assertTrue(analyzeKtFile(use, mutableMapOf(), mutableMapOf(), false, tracker))
        return tracker.depPathsByFile[use.virtualFilePath].orEmpty().map { Path.of(it).fileName.toString() }.toSet()
    }

    private fun assertEdges(useFile: String, vararg expected: String) =
        assertEquals(expected.toSet(), edgesOf(useFile), "dependency closure of $useFile")

    // --- calls ---

    @Test fun constructorCallDependsOnTheClass() = assertEdges("CtorUse.kt", "Made.kt")

    @Test fun companionInvokeDependsOnTheClassFile() = assertEdges("CompanionInvokeUse.kt", "Guarded.kt")

    // Known gap, pinned as-is: `r()` on a value reference resolves (by symbol)
    // to the variable, not to the invoke operator, so an extension `invoke`
    // in another file is not an edge. Closing it would change the cache
    // contract and is out of scope for the resolution-API migration.
    @Test fun implicitInvokeOnAReferenceDoesNotReachTheInvokeOperator() = assertEdges("InvokeUse.kt", "Rx.kt")

    // --- operators ---

    @Test fun binaryOperatorDependsOnItsOperatorFunction() = assertEdges("PlusUse.kt", "Rx.kt", "PlusOps.kt")

    @Test fun compoundAssignmentDependsOnPlusAssign() = assertEdges("AssignUse.kt", "Rx.kt", "AssignOps.kt")

    @Test fun incrementDependsOnInc() = assertEdges("IncUse.kt", "Rx.kt", "IncOps.kt")

    @Test fun unaryOperatorDependsOnItsOperatorFunction() = assertEdges("UnaryUse.kt", "Rx.kt", "UnaryOps.kt")

    @Test fun comparisonDependsOnCompareTo() = assertEdges("CompareUse.kt", "Rx.kt", "CompareOps.kt")

    @Test fun containmentDependsOnContains() = assertEdges("ContainsUse.kt", "Rx.kt", "ContainsOps.kt")

    @Test fun arrayReadDependsOnGet() = assertEdges("ArrayGetUse.kt", "Rx.kt", "GetOps.kt")

    @Test fun arrayWriteDependsOnSet() = assertEdges("ArraySetUse.kt", "Rx.kt", "SetOps.kt")

    @Test fun compoundArrayAssignmentDependsOnGetAndSet() =
        assertEdges("ArrayCompoundUse.kt", "Rx.kt", "CompoundGetOps.kt", "CompoundSetOps.kt")

    // --- iteration, delegation, destructuring ---

    @Test fun forLoopDependsOnIterator() = assertEdges("IterUse.kt", "Rx.kt", "IterOps.kt")

    @Test fun valDelegateDependsOnGetValue() = assertEdges("DelegateUse.kt", "Rx.kt", "DelegateOps.kt")

    @Test fun varDelegateDependsOnGetValueAndSetValue() = assertEdges("DelegateVarUse.kt", "Rx.kt", "DelegateVarOps.kt")

    @Test fun provideDelegateDependsOnProvideDelegate() = assertEdges("ProvideUse.kt", "Rx.kt", "ProvideOps.kt")

    @Test fun destructuringDependsOnComponentFunctions() = assertEdges("DestructureUse.kt", "Rx.kt", "DestructureOps.kt")

    // --- convention halves declared in separate files: losing either is caught ---

    @Test fun delegateGetValueSetValueAndProvideDelegateAreSeparateEdges() =
        assertEdges("SplitDelegateUse.kt", "Rx.kt", "SplitGetValueOps.kt", "SplitSetValueOps.kt", "SplitProvideOps.kt", "Provided2.kt")

    @Test fun destructuringComponentsAreSeparateEdges() =
        assertEdges("SplitDestructureUse.kt", "Rx.kt", "Component1Ops.kt", "Component2Ops.kt")

    @Test fun iteratorHasNextAndNextAreSeparateEdges() =
        assertEdges("SplitIterUse.kt", "Rx.kt", "SplitIteratorOps.kt", "HasNextOps.kt", "NextOps.kt")

    @Test fun ambiguousPlusAndPlusAssignKeepBothEdges() =
        assertEdges("BothUse.kt", "Rx.kt", "BothPlusOps.kt", "BothAssignOps.kt")

    // --- candidate edges (failed resolution) ---

    @Test fun failedCallWithOneCandidateKeepsTheCandidateEdge() = assertEdges("FailedUse.kt", "OnlyOne.kt")

    @Test fun ambiguousCallKeepsEveryCandidateEdge() = assertEdges("AmbiguousUse.kt", "AmbA.kt", "AmbB.kt")

    @Test fun failedOperatorKeepsTheCandidateOperatorEdge() = assertEdges("FailedOpUse.kt", "Rx.kt", "PlusOps.kt")

    @Test fun incompleteEditorCodeKeepsTheCandidateEdge() = assertEdges("IncompleteUse.kt", "OnlyTwo.kt")

    @Test fun unresolvedReferenceHasNoEdges() = assertEdges("UnresolvedUse.kt")

    companion object {
        private val sources = mapOf(
            "Rx.kt" to """
                package p
                class RInvoke
                class RPlus
                class RAssign
                class RInc
                class RUnary
                class RCompare
                class RContains
                class RGet
                class RSet
                class RCompound
                class RIter
                class RDelegate
                class RDelegateVar
                class RProvide
                class RDestructure
                class RSplitDelegate
                class RSplitProvide
                class RSplitDestructure
                class RSplitIter
                class Cursor
                class RBoth
            """.trimIndent(),
            "Made.kt" to "package p\nclass Made(val v: Int)",
            "Guarded.kt" to "package p\nclass Guarded private constructor() { companion object { operator fun invoke(): Guarded = Guarded() } }",
            "InvokeOps.kt" to "package p\noperator fun RInvoke.invoke(): Int = 1",
            "PlusOps.kt" to "package p\noperator fun RPlus.plus(o: RPlus): RPlus = o",
            "AssignOps.kt" to "package p\noperator fun RAssign.plusAssign(o: RAssign) {}",
            "IncOps.kt" to "package p\noperator fun RInc.inc(): RInc = this",
            "UnaryOps.kt" to "package p\noperator fun RUnary.unaryMinus(): RUnary = this",
            "CompareOps.kt" to "package p\noperator fun RCompare.compareTo(o: RCompare): Int = 0",
            "ContainsOps.kt" to "package p\noperator fun RContains.contains(x: Int): Boolean = true",
            "GetOps.kt" to "package p\noperator fun RGet.get(i: Int): Int = i",
            "SetOps.kt" to "package p\noperator fun RSet.set(i: Int, v: Int) {}",
            "CompoundGetOps.kt" to "package p\noperator fun RCompound.get(i: Int): Int = i",
            "CompoundSetOps.kt" to "package p\noperator fun RCompound.set(i: Int, v: Int) {}",
            "IterOps.kt" to "package p\noperator fun RIter.iterator(): Iterator<Int> = listOf(1).iterator()",
            "DelegateOps.kt" to "package p\noperator fun RDelegate.getValue(t: Any?, p: kotlin.reflect.KProperty<*>): Int = 1",
            "DelegateVarOps.kt" to """
                package p
                operator fun RDelegateVar.getValue(t: Any?, p: kotlin.reflect.KProperty<*>): Int = 1
                operator fun RDelegateVar.setValue(t: Any?, p: kotlin.reflect.KProperty<*>, v: Int) {}
            """.trimIndent(),
            "ProvideOps.kt" to """
                package p
                class Provided { operator fun getValue(t: Any?, p: kotlin.reflect.KProperty<*>): Int = 1 }
                operator fun RProvide.provideDelegate(t: Any?, p: kotlin.reflect.KProperty<*>): Provided = Provided()
            """.trimIndent(),
            "DestructureOps.kt" to """
                package p
                operator fun RDestructure.component1(): Int = 1
                operator fun RDestructure.component2(): Int = 2
            """.trimIndent(),
            "SplitGetValueOps.kt" to "package p\noperator fun RSplitDelegate.getValue(t: Any?, p: kotlin.reflect.KProperty<*>): Int = 1",
            "SplitSetValueOps.kt" to "package p\noperator fun RSplitDelegate.setValue(t: Any?, p: kotlin.reflect.KProperty<*>, v: Int) {}",
            "Provided2.kt" to "package p\nclass Provided2 { operator fun getValue(t: Any?, p: kotlin.reflect.KProperty<*>): Int = 1 }",
            "SplitProvideOps.kt" to "package p\noperator fun RSplitProvide.provideDelegate(t: Any?, p: kotlin.reflect.KProperty<*>): Provided2 = Provided2()",
            "Component1Ops.kt" to "package p\noperator fun RSplitDestructure.component1(): Int = 1",
            "Component2Ops.kt" to "package p\noperator fun RSplitDestructure.component2(): Int = 2",
            "SplitIteratorOps.kt" to "package p\noperator fun RSplitIter.iterator(): Cursor = Cursor()",
            "HasNextOps.kt" to "package p\noperator fun Cursor.hasNext(): Boolean = false",
            "NextOps.kt" to "package p\noperator fun Cursor.next(): Int = 1",
            "BothPlusOps.kt" to "package p\noperator fun RBoth.plus(o: RBoth): RBoth = o",
            "BothAssignOps.kt" to "package p\noperator fun RBoth.plusAssign(o: RBoth) {}",
            "OnlyOne.kt" to "package p\nfun onlyOne(x: Int) {}",
            "AmbA.kt" to "package p\nfun ambig(a: Any?, b: Int) {}",
            "AmbB.kt" to "package p\nfun ambig(a: Int, b: Any?) {}",
            "OnlyTwo.kt" to "package p\nfun onlyTwo(x: Int) {}",

            "CtorUse.kt" to "package p\nfun useCtor() { Made(1) }",
            "CompanionInvokeUse.kt" to "package p\nfun useCompanion() { Guarded() }",
            "InvokeUse.kt" to "package p\nfun useInvoke(r: RInvoke) { r() }",
            "PlusUse.kt" to "package p\nfun usePlus(a: RPlus, b: RPlus) { a + b }",
            "AssignUse.kt" to "package p\nfun useAssign(a: RAssign, b: RAssign) { a += b }",
            "IncUse.kt" to "package p\nfun useInc(a: RInc) { var x = a; x++ }",
            "UnaryUse.kt" to "package p\nfun useUnary(a: RUnary) { -a }",
            "CompareUse.kt" to "package p\nfun useCompare(a: RCompare, b: RCompare) { a < b }",
            "ContainsUse.kt" to "package p\nfun useContains(a: RContains) { 1 in a }",
            "ArrayGetUse.kt" to "package p\nfun useGet(a: RGet) { a[0] }",
            "ArraySetUse.kt" to "package p\nfun useSet(a: RSet) { a[0] = 1 }",
            "ArrayCompoundUse.kt" to "package p\nfun useArrayCompound(a: RCompound) { a[0] += 1 }",
            "IterUse.kt" to "package p\nfun useIter(a: RIter) { for (x in a) {} }",
            "DelegateUse.kt" to "package p\nfun useDelegate(a: RDelegate) { val v: Int by a }",
            "DelegateVarUse.kt" to "package p\nfun useDelegateVar(a: RDelegateVar) { var v: Int by a }",
            "ProvideUse.kt" to "package p\nfun useProvide(a: RProvide) { val v: Int by a }",
            "DestructureUse.kt" to "package p\nfun useDestructure(a: RDestructure) { val (x, y) = a }",
            "SplitDelegateUse.kt" to "package p\nfun useSplitDelegate(a: RSplitDelegate, b: RSplitProvide) { var v: Int by a; val w: Int by b }",
            "SplitDestructureUse.kt" to "package p\nfun useSplitDestructure(a: RSplitDestructure) { val (x, y) = a }",
            "SplitIterUse.kt" to "package p\nfun useSplitIter(a: RSplitIter) { for (x in a) {} }",
            "BothUse.kt" to "package p\nfun useBoth(a: RBoth, b: RBoth) { var c = a; c += b }",
            "FailedUse.kt" to "package p\nfun useFailed() { onlyOne(\"not an int\") }",
            "AmbiguousUse.kt" to "package p\nfun useAmbiguous() { ambig(1, 1) }",
            "FailedOpUse.kt" to "package p\nfun useFailedOp(a: RPlus) { a + 1 }",
            "IncompleteUse.kt" to "package p\nfun useIncomplete() {\n    onlyTwo(\n}",
            "UnresolvedUse.kt" to "package p\nfun useUnresolved() { nowhere(1) }",
        )

        private val module: KaSourceModule by lazy {
            val root = createTempDirectory("krit-kaa-dep-resolution-")
            root.toFile().deleteOnExit()
            for ((name, body) in sources) {
                val path = root.resolve(name)
                Files.createDirectories(path.parent)
                Files.writeString(path, body + "\n")
            }
            val stdlib = Path.of(KotlinVersion::class.java.protectionDomain.codeSource.location.toURI()).toString()
            buildSession(
                Disposer.newDisposable("dep-resolution-test"),
                ParsedArgs(sourceDirs = listOf(root.toString()), classpath = listOf(stdlib), jdkHome = null, output = null),
            )
        }
    }
}
