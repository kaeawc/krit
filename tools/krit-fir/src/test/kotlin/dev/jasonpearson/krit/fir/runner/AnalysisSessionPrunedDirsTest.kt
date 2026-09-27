package dev.jasonpearson.krit.fir.runner

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals

class AnalysisSessionPrunedDirsTest {
    @TempDir lateinit var root: Path

    @Test
    fun trackedIdeaTemplateIsNotDiscoveredForCompilation() {
        val source = root.resolve("src/App.kt").toFile()
        val template = root.resolve(".idea/fileTemplates/Template.kt").toFile()
        for (file in listOf(source, template)) {
            file.parentFile.mkdirs()
            file.writeText("class Example")
        }
        val session = AnalysisSession(listOf(root.toString()), emptyList())
        assertEquals(listOf(source.path), session.currentSourceFiles())
        session.dispose()
    }
}
