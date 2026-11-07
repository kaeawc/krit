package dev.jasonpearson.krit.fir

import org.jetbrains.kotlin.config.KotlinCompilerVersion
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * krit-fir ships its own copy of the compiler's FirProviderImpl (it indexes callable names by package;
 * the original rescans every callable per package). The copy is the 2.4.21 source plus that one change, so
 * a different compiler version needs it re-vendored from that version's source before this pin moves.
 */
class FirProviderImplPinTest {
    @Test
    fun `the vendored FirProviderImpl matches the bundled compiler version`() {
        assertEquals(
            "2.4.21", KotlinCompilerVersion.VERSION,
            "Re-vendor org/jetbrains/kotlin/fir/resolve/providers/impl/FirProviderImpl.kt from the new Kotlin " +
                "version (keep the callableNamesInPackage index), then update this pin.",
        )
    }
}
