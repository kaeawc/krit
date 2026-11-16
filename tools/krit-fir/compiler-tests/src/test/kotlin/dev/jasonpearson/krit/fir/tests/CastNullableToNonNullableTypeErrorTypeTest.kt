package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// An operand whose type comes from a class the compiler cannot resolve is an error type, which reads as nullable.
// The error sits in another file, so the file with the cast still compiles and keeps the verdict.
class CastNullableToNonNullableTypeErrorTypeTest {

    private val holder = """
        package errortype

        class Holder {
            val view = MissingView()
        }
    """.trimIndent()

    private val use = """
        package errortype

        fun fromErrorType(holder: Holder): String = holder.view.child as String

        fun fromNullable(input: String?): CharSequence = input as CharSequence
    """.trimIndent()

    @Test fun errorTypedOperandIsNotReportedButNullableOneIs() {
        val result = KritFirProbe.compile(
            mapOf("Holder.kt" to holder, "Use.kt" to use),
            FirRuleCompileContext(enabledRuleIds = setOf("CastNullableToNonNullableType")),
            // The check request turns this on, so a file without errors keeps its verdict next to one with errors.
            configure = { it.reportAllWarnings = true },
        )
        assertTrue(result.compileErrors.all { it.startsWith("Holder.kt") }, result.problems())
        val lines = result.diags.filter { it.name == "CastNullableToNonNullableType" && it.file == "Use.kt" }.map { it.line }
        assertEquals(listOf(5), lines, result.diags.toString() + result.problems())
    }
}
