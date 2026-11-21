// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 54, 56, 83, 84, 89
package test.testwithoutassertion.reviewer

import org.junit.Test
import org.junit.Rule
import org.junit.Assert.assertEquals
import org.junit.rules.ExpectedException
import org.junit.rules.ErrorCollector
import assertk.assertThat
import assertk.assertions.isEqualTo
import strikt.api.expectThat
import org.amshove.kluent.shouldBeEqualTo
import androidx.test.ext.truth.content.IntentSubject
import org.awaitility.Awaitility.await
import org.skyscreamer.jsonassert.JSONAssert
import com.dropbox.dropshots.Dropshots
import com.lemonappdev.konsist.api.KonsistScope
import com.android.tools.lint.checks.infrastructure.lint
import org.mockito.Mockito.verifyZeroInteractions
import androidx.compose.ui.test.junit4.ComposeContentTestRule
import androidx.compose.ui.test.hasText
import kotlinx.coroutines.test.runTest
import com.slack.circuit.test.Presenter
import test.testwithoutassertion.support.*
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

class ReviewerLibraries(
    private val rule: ComposeContentTestRule,
    private val collector: ErrorCollector,
    private val dropshots: Dropshots,
    private val konsist: KonsistScope,
    private val presenter: Presenter,
    private val subject: CustomSubject,
) : BenchmarkBase() {
    @get:Rule val thrown: ExpectedException = ExpectedException.none()
    @Test fun rejectsNonNumber() { thrown.expect(NumberFormatException::class.java); "x".toInt() }
    @Test fun collectorCheck() { collector.checkThat(1, Any()) }
    @Test fun assertkCheck() { assertThat(1 + 1).isEqualTo(2) }
    @Test fun striktCheck() { expectThat(2).isEqualTo(2) }
    @Test fun kluentCheck() { 2 shouldBeEqualTo 2 }
    @Test fun intentTruthCheck() { IntentSubject.assertThat(Any()).hasAction("x") }
    @Test fun awaitilityCheck() { val done = AtomicBoolean(); await().until { done.get() } }
    @Test fun jsonCheck() { JSONAssert.assertEquals("{}", "{}", true) }
    @Test fun dropshotsCheck() { dropshots.assertSnapshot(Any()) }
    @Test fun konsistCheck() { konsist.assertTrue { true } }
    @Test fun lintCheck() { lint().files("source").run().expectClean() }
    @Test fun lintCountCheck() { lint().files("source").run().expectErrorCount(0) }
    @Test fun mockitoCheck() { verifyZeroInteractions(Any()) }
    @Test fun composeCountCheck() { rule.waitUntilNodeCount(hasText("x"), 2) }
    @Test fun inheritedBenchmark() { benchmarkRule.measureRepeated { println(1) } }
    @Test fun circuitSubtype() = runTest { presenter.test { awaitItem() } }
    @Test fun customTruthSubtype() { subject.matchesValue(1) } // FIR abstains: external custom subject is not allowlisted.
    @Test fun crossModuleNamedHelper() { rule.verifyLoginShown() }
    @Test fun crossSourceSetHelper() { rule.displayLogin() } // FIR abstains: cross-file/source-set helper body is unknown.
}

class ReviewerCallbacks(private val listener: Listener) {
    @Test fun anonymousRunnable() {
        Executors.newSingleThreadExecutor().submit(object : Runnable {
            override fun run() { assertEquals(1, 1) }
        }).get()
    }
    @Test fun escapedLambda() {
        val task: () -> Unit = { assertEquals(1, 1) }
        Executors.newSingleThreadExecutor().submit(task).get()
    }
    @Test fun assignedListener() { listener.onChange = { assertEquals(1, it) } }
}

class ReviewerConstruction {
    @org.junit.jupiter.params.ParameterizedTest
    fun rejectsInvalid(value: String) { org.junit.jupiter.api.Assertions.assertThrows(IllegalArgumentException::class.java) { Email(value) } }
    @Test fun acceptsValid() { Email("a@b.c") }
}

private class Email(value: String)

class ReviewerConservativeEvidence {
    private val values = mapOf("a" to "ready")
    private fun loaded(): String = values["a"] ?: error("not loaded")
    @Test fun loads() { loaded() } // FIR abstains: helper uses an Elvis failure and unknown member lookup.
    @Test fun productionFileOperation() { Repository().load() } // FIR abstains: cross-file production body and constructor are unknown.
    @Test fun trailingPropertyCast() {
        val value: Any = "ready"
        val text = value as String
    }
    @Test fun externalPlatformCallback() { java.util.concurrent.CompletableFuture.runAsync { println(1) }.get() } // FIR abstains: external platform callback is not allowlisted.
}
