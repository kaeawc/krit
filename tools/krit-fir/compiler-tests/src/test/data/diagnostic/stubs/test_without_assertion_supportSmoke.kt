package stubs

import test.testwithoutassertion.support.displayLogin
import test.testwithoutassertion.support.verifyLoginShown

fun supportSmoke(rule: androidx.compose.ui.test.junit4.ComposeContentTestRule, base: test.testwithoutassertion.support.BenchmarkBase, subject: test.testwithoutassertion.support.CustomSubject, listener: test.testwithoutassertion.support.Listener) {
    rule.displayLogin()
    rule.verifyLoginShown()
    base.benchmarkRule.measureRepeated { subject.matchesValue(1) }
    subject.matchesValue(1)
    listener.onChange = { it.hashCode() }
    test.testwithoutassertion.support.Repository().load()
}
