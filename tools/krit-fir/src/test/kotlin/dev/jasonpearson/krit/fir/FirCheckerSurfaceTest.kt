package dev.jasonpearson.krit.fir

import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.type.TypeCheckers
import org.jetbrains.kotlin.fir.analysis.extensions.FirAdditionalCheckersExtension
import java.lang.reflect.Modifier
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class FirCheckerSurfaceTest {
    private fun checkerGetters(base: Class<*>): List<String> =
        base.methods.filter {
            it.declaringClass == base && it.parameterCount == 0 && !Modifier.isStatic(it.modifiers) &&
                !Modifier.isFinal(it.modifiers) && it.name.startsWith("get") && it.name.endsWith("Checkers") &&
                !it.name.startsWith("getAll")
        }.map { it.name }.sorted()

    private fun assertOverridesAll(base: Class<*>, merged: Any) {
        val getters = checkerGetters(base)
        assertTrue(getters.isNotEmpty(), "no checker getters found on ${base.name}")
        val missing = getters.filter { merged.javaClass.getMethod(it).declaringClass == base }
        assertEquals(emptyList(), missing, "merged ${base.simpleName} does not override")
    }

    // Enumerated from the K2 API rather than hardcoded, so a compiler bump that
    // adds a checker-set property fails here instead of silently never merging.
    @Test fun mergedCheckerSetsOverrideEveryK2CheckerKind() {
        val merged = mergeFirRules(emptyList())
        assertOverridesAll(ExpressionCheckers::class.java, merged.expression)
        assertOverridesAll(DeclarationCheckers::class.java, merged.declaration)
        assertOverridesAll(TypeCheckers::class.java, merged.type)
    }

    @Test fun pluginExtensionContributesEveryPerSourceCheckerFamily() {
        // Language-version-settings checkers validate compiler arguments, not
        // source, so no FirRule can contribute one.
        val notPerSource = setOf("getLanguageVersionSettingsCheckers")
        val missing = checkerGetters(FirAdditionalCheckersExtension::class.java)
            .filter { it !in notPerSource }
            .filter { KritFirCheckers::class.java.getMethod(it).declaringClass != KritFirCheckers::class.java }
        assertEquals(emptyList(), missing, "KritFirCheckers does not override")
    }
}
