// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test.testwithoutassertion.benchmarkwildcard

import org.junit.Test
import androidx.benchmark.junit4.*

class StartupBenchmark {
    @Test fun warmup() { println("warm") }
}
