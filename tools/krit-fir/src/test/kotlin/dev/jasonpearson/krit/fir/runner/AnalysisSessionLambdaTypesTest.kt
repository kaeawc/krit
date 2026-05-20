package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Assumptions.assumeTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertTrue

/**
 * End-to-end coverage for the lambda-suspend + call-expression-type
 * data the oracle pass collects for the FIR-backed [Resolver]. The
 * tests skip when kotlin-stdlib isn't reachable on the test JVM
 * (suspend resolution needs `kotlin.coroutines.CoroutineScope` etc.
 * to be on the classpath).
 */
class AnalysisSessionLambdaTypesTest {

    @TempDir
    lateinit var tmp: Path

    private lateinit var stdlibClasspath: List<String>

    @BeforeEach
    fun resolveStdlib() {
        val stdlib = findKotlinStdlib()
        assumeTrue(
            stdlib != null,
            "kotlin-stdlib jar not found in Gradle cache; required for suspend-conversion + " +
                "type resolution. Set KOTLIN_STDLIB_JAR or populate the Gradle cache by running " +
                "`./gradlew :test` at the repo root.",
        )
        stdlibClasspath = listOfNotNull(stdlib)
    }

    @Test
    fun lambdaPassedToSuspendParamSurfacesAsSuspendInResolver() {
        val path = writeKt(
            "SuspendLambda.kt",
            """
            package com.acme.suspendlambda

            suspend fun runSuspend(block: suspend () -> Unit) {
                block()
            }

            suspend fun caller() {
                runSuspend {
                    println("inside suspend block")
                }
            }
            """.trimIndent(),
        )

        val lambdaSuspend = analyze(path).lambdaSuspendByLineCol
        assertTrue(
            lambdaSuspend.values.any { it },
            "expected at least one lambda flagged suspend, got $lambdaSuspend",
        )
    }

    @Test
    fun lambdaPassedToPlainBlockParamSurfacesAsNotSuspend() {
        val path = writeKt(
            "PlainLambda.kt",
            """
            package com.acme.plainlambda

            fun runPlain(block: () -> Unit) {
                block()
            }

            fun caller() {
                runPlain {
                    println("inside plain block")
                }
            }
            """.trimIndent(),
        )

        val lambdaSuspend = analyze(path).lambdaSuspendByLineCol
        // No lambda passed to a `() -> Unit` parameter should carry
        // the suspend flag. The map may also be empty for lambdas the
        // oracle didn't visit; the contract is "nothing flagged
        // suspend", and the resolver's missing-key default is false.
        assertTrue(
            lambdaSuspend.values.none { it },
            "no lambda should be suspend in plain-block source, got $lambdaSuspend",
        )
    }

    @Test
    fun propertyAccessTypeFqnLandsOnNonCallPayload() {
        // OracleQualifiedAccessChecker captures property reads. The
        // payload it writes has a populated `type` and
        // `callTargetResolved=false` so the resolver's call-specific
        // lookups still ignore it; `expressionType(propertyRef)`
        // returns the real FQN.
        val path = writeKt(
            "PropAccess.kt",
            """
            package com.acme.propaccess

            class Box(val label: String)

            fun caller(box: Box): String = box.label
            """.trimIndent(),
        )

        val expressions = analyze(path).expressions
        val nonCallStringEntries = expressions.values.filter {
            it.type == "kotlin.String" && !it.callTargetResolved
        }
        assertTrue(
            nonCallStringEntries.isNotEmpty(),
            "expected at least one non-call property-access payload with type=kotlin.String, got " +
                "${expressions.values.map { it.type to it.callTargetResolved }}",
        )
    }

    @Test
    fun callExpressionTypeFqnLandsOnExpressionPayload() {
        val path = writeKt(
            "CallType.kt",
            """
            package com.acme.calltype

            fun produceString(): String = "hi"

            fun caller(): String = produceString()
            """.trimIndent(),
        )

        val expressions = analyze(path).expressions
        // The call to `produceString()` returns kotlin.String; the
        // populated ExpressionPayload.type drives the resolver's
        // expressionType() lookup.
        assertTrue(
            expressions.values.any { it.type == "kotlin.String" },
            "expected at least one call payload with type=kotlin.String, got " +
                "${expressions.values.map { it.type }}",
        )
    }

    private fun analyze(path: String) = AnalysisSession(
        sourceDirs = listOf(tmp.toFile().absolutePath),
        classpath = stdlibClasspath,
    ).analyze(emptyList()).files[path] ?: error("no file payload for $path")

    private fun writeKt(name: String, source: String): String {
        val file = tmp.resolve(name).toFile()
        file.writeText(source)
        return file.canonicalPath
    }

    private fun findKotlinStdlib(): String? {
        System.getenv("KOTLIN_STDLIB_JAR")?.let { override ->
            if (File(override).isFile) return override
        }
        val home = System.getProperty("user.home") ?: return null
        val cacheRoot = File(home, ".gradle/caches/modules-2/files-2.1/org.jetbrains.kotlin/kotlin-stdlib")
        if (!cacheRoot.isDirectory) return null
        val matches = cacheRoot.walkTopDown()
            .filter { it.isFile && it.name.startsWith("kotlin-stdlib-") && it.name.endsWith(".jar") }
            .filter { !it.name.contains("sources") && !it.name.contains("javadoc") }
            .toList()
            .sortedByDescending { it.name }
        return matches.firstOrNull()?.absolutePath
    }
}
