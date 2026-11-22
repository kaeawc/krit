package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import dev.jasonpearson.krit.fir.SdkLevels
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class UnspecifiedImmutableFlagFilesTest {
    private fun source(packageName: String) = """
        package $packageName
        import android.app.PendingIntent
        import android.content.Context
        import android.content.Intent
        fun pending(context: Context, intent: Intent) = PendingIntent.getActivity(context, 0, intent, 0)
    """.trimIndent()

    @Test fun sdkIsLeftToGoAndTestExemptions() {
        val levels = mapOf(
            "Below.kt" to SdkLevels(targetSdk = 22),
            "Warning.kt" to SdkLevels(targetSdk = 23),
            "PreS.kt" to SdkLevels(targetSdk = 30),
            "S.kt" to SdkLevels(targetSdk = 31),
            "MinBelow.kt" to SdkLevels(minSdk = 22),
            "MinS.kt" to SdkLevels(minSdk = 31),
            "TargetWins.kt" to SdkLevels(minSdk = 31, targetSdk = 22),
            "Unknown.kt" to SdkLevels(),
            "Listed.kt" to SdkLevels(targetSdk = 35),
        )
        val result = KritFirProbe.compile(
            levels.keys.associateWith { source(it.removeSuffix(".kt").lowercase()) },
            FirRuleCompileContext(enabledRuleIds = setOf("UnspecifiedImmutableFlag")),
            sdkLevels = levels,
            testFiles = setOf("Listed.kt"),
        )
        assertTrue(result.clean, result.problems())
        assertEquals(
            levels.keys - "Listed.kt",
            result.diags.filter { it.name == "UnspecifiedImmutableFlag" }.map { it.file }.toSet(),
        )
        assertTrue(result.diags.all { it.message.contains("Missing `PendingIntent` mutability flag") })
    }

    @Test fun sameFileStableValsAreKnown() {
        val result = KritFirProbe.compile(
            mapOf("Stable.kt" to """
                package pendingflag.stable
                import android.app.PendingIntent
                import android.content.Context
                import android.content.Intent
                private val NONE = 0
                class N {
                    private val flags = PendingIntent.FLAG_ONE_SHOT
                    fun pending(context: Context, intent: Intent) {
                        PendingIntent.getBroadcast(context, 0, intent, flags)
                        PendingIntent.getActivity(context, 0, intent, NONE)
                    }
                }
            """.trimIndent()),
            FirRuleCompileContext(enabledRuleIds = setOf("UnspecifiedImmutableFlag")),
        )
        assertTrue(result.clean, result.problems())
        assertEquals(listOf(9, 10), result.diags.filter { it.name == "UnspecifiedImmutableFlag" }.map { it.line })
    }

    @Test fun otherFileNonConstIsOpaqueButConstIsKnown() {
        val sources = mapOf(
            "Constants.kt" to "package pendingflag.external; const val ZERO = 0; val opaque = 0",
            "Consumer.kt" to """
                package pendingflag.external
                import android.app.PendingIntent
                import android.content.Context
                import android.content.Intent
                fun pending(context: Context, intent: Intent) {
                    PendingIntent.getActivity(context, 0, intent, ZERO)
                    PendingIntent.getActivity(context, 0, intent, opaque)
                }
            """.trimIndent(),
        )
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("UnspecifiedImmutableFlag")),
        )
        assertTrue(result.clean, result.problems())
        assertEquals(listOf(6), result.diags.filter { it.name == "UnspecifiedImmutableFlag" }.map { it.line })
    }
}
