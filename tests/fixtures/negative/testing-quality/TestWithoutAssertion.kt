package test

import com.autonomousapps.kit.GradleBuilder.build
import org.junit.Ignore
import org.junit.Test
import org.junit.Assert.assertEquals
import org.thoughtcrime.securesms.testing.assertIsSize

class TestWithoutAssertionNegative {
    @Test
    fun loads() {
        val x = 42
        assertEquals(42, x)
    }

    @Test
    fun signalStyleInfixAssertion() {
        val values = listOf(1, 2)
        values assertIsSize 2
    }

    @Test(expected = IllegalArgumentException::class)
    fun expectedException() {
        parse("")
    }

    @Test
    fun visualSnapshot() {
        paparazzi.snapshot(Unit)
    }

    @Test
    fun macrobenchmark() {
        benchmarkRule.measureRepeated()
    }
}

class CompilerHarnessNegative : MetroCompilerTest() {
    @Test
    fun generatedCodeCompiles() {
        compile(source("class Example"))
    }
}

class GradleBuilderBuildNegative {
    @Test
    fun projectBuilds() {
        build(project.rootDir, "compileKotlin")
    }
}

class IncrementalBuildNegative : BaseIncrementalCompilationTest() {
    @Test
    fun projectFailsAfterChange() {
        project.compileKotlinAndFail()
    }
}

@Ignore("manual preview")
class TestWithoutAssertionIgnoredNegative {
    @Test
    fun preview() {
        println("manual")
    }
}
