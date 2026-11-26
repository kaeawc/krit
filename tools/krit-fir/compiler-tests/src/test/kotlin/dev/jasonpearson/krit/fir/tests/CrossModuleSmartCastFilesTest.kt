package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * The check compiles a scan's Gradle modules as one, where a property of another module is stable and smart-casts.
 * That module's own build rejects the cast, so a local inferred non-null through it (`val p = a.team.profile` after
 * `a.team.profile != null`) stays nullable there, and the redundant-null-safety rules must not call its `?.` or `?:`
 * unnecessary (#1672). The same code within one module is still reported.
 */
class CrossModuleSmartCastFilesTest {
    private val model = """
        package demo.lib

        class Profile(val fields: List<String>)
        class Team(val profile: Profile?, val names: List<String>?)
        class Account(val team: Team)
    """.trimIndent()

    private fun use(pkg: String) = """
        package $pkg

        import demo.lib.Account

        fun safeCall(account: Account?): Int {
            if (account != null && account.team.profile != null) {
                val profile = account.team.profile
                return profile?.fields?.size ?: 0
            }
            return 0
        }

        fun elvis(account: Account): Int {
            if (account.team.profile == null) return 0
            val profile = account.team.profile
            val known = profile ?: return 0
            return known.fields.size
        }

        fun notNull(account: Account): Int {
            if (account.team.profile == null) return 0
            val profile = account.team.profile
            return profile!!.fields.size
        }

        fun orEmpty(account: Account): Int {
            if (account.team.names == null) return 0
            val names = account.team.names
            return names.orEmpty().size
        }

        fun direct(account: Account): Int {
            if (account.team.profile == null) return 0
            return account.team.profile?.fields?.size ?: 0
        }
    """.trimIndent()

    private val rules = setOf("UnnecessarySafeCall", "UselessElvisOnNonNull", "UnnecessaryNotNullOperator", "UselessCallOnNotNull")

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(sources, FirRuleCompileContext(enabledRuleIds = rules))
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name in rules }.map { it.name to it.line }.sortedBy { it.second }
    }

    @Test fun aLocalInferredNonNullThroughAnotherModulesPropertyIsNotRedundant() {
        val sources = mapOf(
            "lib/src/main/kotlin/demo/lib/Account.kt" to model,
            "src/main/kotlin/demo/Profiles.kt" to use("demo"),
        )
        assertEquals(emptyList(), findings(sources))
    }

    // UnnecessaryNotNullOperator and UselessCallOnNotNull read the declared type through smart casts, so `profile!!`
    // and `names.orEmpty()` stay silent in both layouts; they are kept here so a change to that is noticed.
    @Test fun theSameCodeWithinOneModuleIsReported() {
        val sources = mapOf(
            "lib/src/main/kotlin/demo/lib/Account.kt" to model,
            "lib/src/main/kotlin/demo/lib/Profiles.kt" to use("demo.lib.use"),
        )
        assertEquals(
            listOf("UnnecessarySafeCall" to 8, "UselessElvisOnNonNull" to 16, "UnnecessarySafeCall" to 34),
            findings(sources),
        )
    }
}
