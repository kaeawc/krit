// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package test.testwithoutassertion.visibilitylookalike

import org.junit.Test

class VisibilityLookalike {
    private fun isBannerDisplayed() = true
    @Test fun <!TestWithoutAssertion!>banner<!>() { isBannerDisplayed() }
}
