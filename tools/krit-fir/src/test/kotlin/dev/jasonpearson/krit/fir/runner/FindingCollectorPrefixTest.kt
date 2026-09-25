package dev.jasonpearson.krit.fir.runner

import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageLocation
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals

class FindingCollectorPrefixTest {
    @TempDir lateinit var tmp: Path
    @Test fun onlyLeadingRulePrefixIsCaptured() {
        val path = tmp.resolve("Probe.kt").toString()
        val collector = FindingCollector(mapOf(java.io.File(path).canonicalPath to path))
        val at = CompilerMessageLocation.create(path, 1, 1, null)
        collector.report(CompilerMessageSeverity.WARNING, "call to [SomeApi] is deprecated", at)
        collector.report(CompilerMessageSeverity.WARNING, "[ProtocolProbe] configured", at)
        assertEquals(listOf("ProtocolProbe"), collector.findings.map { it.rule })
        assertEquals("configured", collector.findings.single().message)
    }
}
