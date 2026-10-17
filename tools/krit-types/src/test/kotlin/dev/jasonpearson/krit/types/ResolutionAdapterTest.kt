package dev.jasonpearson.krit.types

import com.intellij.openapi.util.Disposer
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.analyze
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtTreeVisitorVoid
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.createTempDirectory
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * The adapter's two policies differ exactly on failed calls with a sole
 * candidate: SUCCESS_ONLY must not turn that candidate into a success.
 */
@OptIn(KaExperimentalApi::class)
class ResolutionAdapterTest {
    private fun target(callee: String, policy: CallResolutionPolicy): Pair<String?, CallCertainty?> {
        val file = module.psiRoots.filterIsInstance<KtFile>().first { it.name == "Use.kt" }
        val call = mutableListOf<KtCallExpression>().also { found ->
            file.accept(object : KtTreeVisitorVoid() {
                override fun visitCallExpression(expression: KtCallExpression) {
                    super.visitCallExpression(expression)
                    if (expression.calleeExpression?.text == callee) found.add(expression)
                }
            })
        }.single()
        return analyze(file) {
            val resolved = resolveCallTarget(call, policy)
            resolved?.symbol?.callableId?.asSingleFqName()?.asString() to resolved?.certainty
        }
    }

    @Test fun validCallIsASuccessUnderBothPolicies() {
        assertEquals("p.valid" to CallCertainty.SUCCESS, target("valid", CallResolutionPolicy.SUCCESS_ONLY))
        assertEquals("p.valid" to CallCertainty.SUCCESS, target("valid", CallResolutionPolicy.CANDIDATE_TOLERANT))
    }

    @Test fun soleCandidateOfAFailedCallIsOnlyATolerantCandidate() {
        assertEquals(null to null, target("failing", CallResolutionPolicy.SUCCESS_ONLY))
        assertEquals("p.failing" to CallCertainty.CANDIDATE, target("failing", CallResolutionPolicy.CANDIDATE_TOLERANT))
    }

    @Test fun ambiguousCallHasNoTargetUnderEitherPolicy() {
        assertEquals(null to null, target("amb", CallResolutionPolicy.SUCCESS_ONLY))
        assertEquals(null to null, target("amb", CallResolutionPolicy.CANDIDATE_TOLERANT))
    }

    @Test fun unresolvedCallHasNoTargetUnderEitherPolicy() {
        assertEquals(null to null, target("nothingHere", CallResolutionPolicy.SUCCESS_ONLY))
        assertEquals(null to null, target("nothingHere", CallResolutionPolicy.CANDIDATE_TOLERANT))
    }

    companion object {
        private val sources = mapOf(
            "Decls.kt" to """
                package p
                fun valid(x: Int): Int = x
                fun failing(x: Int): Int = x
                fun amb(a: Any?, b: Int): Int = b
                fun amb(a: Int, b: Any?): Int = a
            """.trimIndent(),
            "Use.kt" to """
                package p
                fun use() {
                    valid(1)
                    failing("not an int")
                    amb(1, 1)
                    nothingHere(1)
                }
            """.trimIndent(),
        )

        private val module: KaSourceModule by lazy {
            val root = createTempDirectory("krit-kaa-resolution-adapter-")
            root.toFile().deleteOnExit()
            for ((name, body) in sources) Files.writeString(root.resolve(name), body + "\n")
            val stdlib = Path.of(KotlinVersion::class.java.protectionDomain.codeSource.location.toURI()).toString()
            buildSession(
                Disposer.newDisposable("resolution-adapter-test"),
                ParsedArgs(sourceDirs = listOf(root.toString()), classpath = listOf(stdlib), jdkHome = null, output = null),
            )
        }
    }
}
