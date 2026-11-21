// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package androidx.test.espresso

import org.junit.Test
import androidx.test.espresso.*

class EspressoVisibility {
    private fun isBannerDisplayed() = true
    private fun renderBanner() { isBannerDisplayed() }
    @Test fun banner() { isBannerDisplayed() }
    @Test fun delegatedBanner() { renderBanner() }
}
