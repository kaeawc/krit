package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

class ScopeOnParameterizedClassMessagesTest {
    @Test
    fun sharedAndCdiMessages() {
        val source = """
            package jakarta.enterprise.context

            import org.koin.core.annotation.Singleton
            import org.koin.core.annotation.Single

            @NormalScope
            annotation class ApplicationScoped

            @Singleton
            class SharedCache<T>

            @ApplicationScoped
            class CdiCache<T>

            @NormalScope
            annotation class ProjectNormalScope

            @ProjectNormalScope
            class ProjectCdiCache<T>

            @Single
            class SingleCache<T>
        """.trimIndent()
        val messages = KritFirProbe.diagnose(
            mapOf("Main.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("ScopeOnParameterizedClass")),
        )
            .filter { it.file == "Main.kt" && it.name == "ScopeOnParameterizedClass" }
            .map { it.message.removePrefix("[ScopeOnParameterizedClass] ") }
        assertEquals(
            listOf(
                "@Singleton on generic class 'SharedCache' shares one instance across all type arguments " +
                    "because the type parameter is erased at runtime.",
                "@ApplicationScoped on generic class 'CdiCache': CDI allows only @Dependent on a bean " +
                    "with type parameters, so this is a definition error.",
                "@ProjectNormalScope on generic class 'ProjectCdiCache': CDI allows only @Dependent on a bean " +
                    "with type parameters, so this is a definition error.",
                "@Single on generic class 'SingleCache' shares one instance across all type arguments " +
                    "because the type parameter is erased at runtime.",
            ),
            messages,
        )
    }
}
