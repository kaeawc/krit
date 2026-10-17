package dev.jasonpearson.krit.types

import com.intellij.openapi.util.Disposer
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtLambdaExpression
import org.jetbrains.kotlin.psi.KtTreeVisitorVoid
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.createTempDirectory
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * Characterizes how krit-types turns Analysis API call resolution into
 * call targets, so a change of resolution API cannot change behaviour
 * unnoticed.
 *
 * Both call-target consumers (`analyzeKtFile`'s expression export and the
 * plugin `Resolver`) are candidate-tolerant: a call that failed to resolve
 * but has exactly one candidate still yields that candidate's callable.
 * The expectations below pin that contract for valid, failed, ambiguous
 * and unresolved calls.
 */
@OptIn(KaExperimentalApi::class)
class CallResolutionCharacterizationTest {
    private data class Site(val fqn: String?, val resolved: Boolean, val suspend: Boolean, val annotations: List<String>)

    private val exported = mutableMapOf<String, Map<String, ExpressionResult>>()

    private fun ktFile(name: String): KtFile = module.psiRoots.filterIsInstance<KtFile>().first { it.name == name }

    private fun calls(file: KtFile, callee: String): List<KtCallExpression> {
        val found = mutableListOf<KtCallExpression>()
        file.accept(object : KtTreeVisitorVoid() {
            override fun visitCallExpression(expression: KtCallExpression) {
                super.visitCallExpression(expression)
                if (expression.calleeExpression?.text == callee) found.add(expression)
            }
        })
        return found.sortedBy { it.textRange.startOffset }
    }

    /** The exported call target of the [nth] call to [callee] in [fileName]. */
    private fun exportedSite(fileName: String, callee: String, nth: Int = 0): Site {
        val file = ktFile(fileName)
        val expressions = exported.getOrPut(fileName) {
            val files = mutableMapOf<String, FileResult>()
            assertTrue(analyzeKtFile(file, files, mutableMapOf(), true))
            files.getValue(file.virtualFilePath).expressions
        }
        val call = calls(file, callee)[nth]
        val document = assertNotNull(file.viewProvider.document)
        val line = document.getLineNumber(call.textRange.startOffset) + 1
        val col = call.textRange.startOffset - document.getLineStartOffset(line - 1) + 1
        val result = assertNotNull(expressions["$line:$col"], "no export for $callee in $fileName; keys=${expressions.keys}")
        return Site(result.callTarget, result.callTargetResolved, result.callTargetSuspend, result.annotations)
    }

    private fun resolverSite(fileName: String, callee: String, nth: Int = 0): Pair<String?, Boolean> {
        val file = ktFile(fileName)
        val resolver = AnalysisApiResolver(file)
        val call = calls(file, callee)[nth]
        return resolver.resolvedCallFqName(call) to resolver.isSuspendCall(call)
    }

    private fun assertSite(
        fileName: String,
        callee: String,
        fqn: String?,
        resolved: Boolean,
        suspend: Boolean = false,
        annotations: List<String> = emptyList(),
        nth: Int = 0,
    ) {
        assertEquals(Site(fqn, resolved, suspend, annotations), exportedSite(fileName, callee, nth), "export of $callee in $fileName")
        // The plugin Resolver agrees with the export on resolved targets and
        // answers null where the export falls back to lexical text.
        val (resolverFqn, resolverSuspend) = resolverSite(fileName, callee, nth)
        assertEquals(if (resolved) fqn else null, resolverFqn, "resolver fqn of $callee in $fileName")
        assertEquals(resolved && suspend, resolverSuspend, "resolver suspend of $callee in $fileName")
    }

    // --- success ---

    @Test fun validTopLevelCall() = assertSite("CallsValid.kt", "valid", "p.valid", true)

    @Test fun validStdlibCall() = assertSite("CallsValid.kt", "listOf", "kotlin.collections.listOf", true)

    @Test fun suspendCallIsMarked() = assertSite("CallsValid.kt", "susp", "p.susp", true, suspend = true)

    @Test fun annotationsOfTheCallTargetAreExported() =
        assertSite("CallsValid.kt", "tagged", "p.tagged", true, annotations = listOf("p.Tag"))

    @Test fun memberCallResolvesToTheDeclaringClass() = assertSite("CallsValid.kt", "member", "p.Holder.member", true)

    // A constructor symbol has no callable id, so the export keeps the lexical
    // callee and reports the call as not resolved. Downstream rules rely on
    // constructors staying "unresolved" here.
    @Test fun constructorCallHasNoCallableIdAndFallsBackToLexicalText() =
        assertSite("CallsCtor.kt", "Plain", "Plain", false)

    @Test fun companionInvokeWinsOverPrivateConstructor() =
        assertSite("CallsCtor.kt", "Guarded", "p.Guarded.Companion.invoke", true)

    @Test fun implicitInvokeOnAnObjectResolvesToInvoke() = assertSite("CallsCtor.kt", "i", "p.Invokable.invoke", true)

    @Test fun implicitInvokeOnAFunctionTypeResolvesToFunctionInvoke() =
        assertSite("CallsCtor.kt", "f", "kotlin.Function0.invoke", true)

    @Test fun suspendFunctionTypeInvokeIsMarked() =
        assertSite("CallsCtor.kt", "sf", "kotlin.coroutines.SuspendFunction0.invoke", true, suspend = true)

    // --- candidate-tolerant: failed call with exactly one candidate ---

    @Test fun failedCallWithOneCandidateStillReportsTheCandidate() =
        assertSite("CallsFailed.kt", "valid", "p.valid", true)

    @Test fun failedCallWithOneSuspendCandidateKeepsTheSuspendMarker() =
        assertSite("CallsFailed.kt", "susp1", "p.susp1", true, suspend = true)

    @Test fun failedCallWithOneAnnotatedCandidateKeepsAnnotations() =
        assertSite("CallsFailed.kt", "tagged1", "p.tagged1", true, annotations = listOf("p.Tag"))

    // --- unknown: no unique candidate ---

    @Test fun ambiguousOverloadsFallBackToLexicalText() = assertSite("CallsFailed.kt", "amb", "amb", false)

    @Test fun unresolvedCallFallsBackToLexicalText() = assertSite("CallsFailed.kt", "nothingHere", "nothingHere", false)

    @Test fun unresolvedReceiverCallFallsBackToLexicalText() = assertSite("CallsFailed.kt", "ghost", "ghost", false)

    @Test fun incompleteEditorCodeStillResolvesTheCallee() = assertSite("CallsIncomplete.kt", "valid", "p.valid", true)

    @Test fun incompleteCodeWithoutAnyCandidateFallsBackToLexicalText() =
        assertSite("CallsIncomplete.kt", "missing", "missing", false)

    // --- isLambdaSuspend (candidate-tolerant, uses the sole function candidate) ---

    private fun lambdaSuspend(fileName: String, nth: Int): Boolean {
        val file = ktFile(fileName)
        val lambdas = mutableListOf<KtLambdaExpression>()
        file.accept(object : KtTreeVisitorVoid() {
            override fun visitLambdaExpression(expression: KtLambdaExpression) {
                super.visitLambdaExpression(expression)
                lambdas.add(expression)
            }
        })
        return AnalysisApiResolver(file).isLambdaSuspend(lambdas.sortedBy { it.textRange.startOffset }[nth])
    }

    @Test fun lambdaPassedToASuspendParameterIsSuspend() = assertTrue(lambdaSuspend("LambdaCalls.kt", 0))

    @Test fun lambdaPassedToAPlainParameterIsNotSuspend() = assertFalse(lambdaSuspend("LambdaCalls.kt", 1))

    @Test fun lambdaOfAFailedCallWithOneCandidateUsesTheCandidate() = assertTrue(lambdaSuspend("LambdaCalls.kt", 2))

    @Test fun lambdaOfAnUnresolvedCallIsNotSuspend() = assertFalse(lambdaSuspend("LambdaCalls.kt", 3))

    companion object {
        private val sources = mapOf(
            "Decls.kt" to """
                package p
                annotation class Tag
                @Tag fun tagged(): Int = 1
                @Tag fun tagged1(x: Int): Int = x
                suspend fun susp(): Int = 1
                suspend fun susp1(x: Int): Int = x
                fun valid(x: Int): Int = x
                fun amb(a: Any?, b: Int): Int = b
                fun amb(a: Int, b: Any?): Int = a
                class Holder { fun member(): Int = 1 }
                class Plain(val x: Int)
                class Guarded private constructor() { companion object { operator fun invoke(): Guarded = Guarded() } }
                class Invokable { operator fun invoke(): Int = 1 }
                fun runSuspend(block: suspend () -> Unit) {}
                fun runPlain(block: () -> Unit) {}
                fun runSuspend2(n: Int, block: suspend () -> Unit) {}
            """.trimIndent(),
            "CallsValid.kt" to """
                package p
                suspend fun callsValid(h: Holder) {
                    valid(1)
                    listOf(1)
                    susp()
                    tagged()
                    h.member()
                }
            """.trimIndent(),
            "CallsCtor.kt" to """
                package p
                suspend fun callsCtor(i: Invokable, f: () -> Int, sf: suspend () -> Int) {
                    Plain(1)
                    Guarded()
                    i()
                    f()
                    sf()
                }
            """.trimIndent(),
            "CallsFailed.kt" to """
                package p
                fun callsFailed() {
                    valid("not an int")
                    susp1("x")
                    tagged1("x")
                    amb(1, 1)
                    nothingHere(1)
                    "s".ghost()
                }
            """.trimIndent(),
            "CallsIncomplete.kt" to """
                package p
                fun callsIncomplete() {
                    valid(
                    missing(
                }
            """.trimIndent(),
            "LambdaCalls.kt" to """
                package p
                suspend fun callsLambdas() {
                    runSuspend { susp() }
                    runPlain { 1 }
                    runSuspend2("x") { susp() }
                    unresolvedRunner { 1 }
                }
            """.trimIndent(),
        )

        private val module: KaSourceModule by lazy {
            val root = createTempDirectory("krit-kaa-call-resolution-")
            root.toFile().deleteOnExit()
            for ((name, body) in sources) {
                val path = root.resolve(name)
                Files.createDirectories(path.parent)
                Files.writeString(path, body + "\n")
            }
            val stdlib = Path.of(KotlinVersion::class.java.protectionDomain.codeSource.location.toURI()).toString()
            buildSession(
                Disposer.newDisposable("call-resolution-test"),
                ParsedArgs(sourceDirs = listOf(root.toString()), classpath = listOf(stdlib), jdkHome = null, output = null),
            )
        }
    }
}
