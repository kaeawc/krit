package dev.jasonpearson.krit.intellij

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class KritSuppressTest {
    @Test
    fun `mergeArguments appends to empty list`() {
        assertEquals(listOf("HardcodedColor"), KritSuppress.mergeArguments(emptyList(), "HardcodedColor"))
    }

    @Test
    fun `mergeArguments preserves existing args and appends new one`() {
        assertEquals(
            listOf("UNCHECKED_CAST", "HardcodedColor"),
            KritSuppress.mergeArguments(listOf("UNCHECKED_CAST"), "HardcodedColor"),
        )
    }

    @Test
    fun `mergeArguments returns null when rule id already present`() {
        // Caller skips the edit so the file is not touched. Avoids
        // duplicate args and unnecessary undo entries.
        assertNull(KritSuppress.mergeArguments(listOf("HardcodedColor"), "HardcodedColor"))
        assertNull(
            KritSuppress.mergeArguments(listOf("UNCHECKED_CAST", "HardcodedColor"), "HardcodedColor"),
        )
    }

    @Test
    fun `titleFor uses the rule id`() {
        assertEquals("Suppress Krit 'HardcodedColor' on this declaration", KritSuppress.titleFor("HardcodedColor"))
    }

    @Test
    fun `formatJavaSuppressValue emits bare string for single arg`() {
        assertEquals("\"HardcodedColor\"", KritSuppress.formatJavaSuppressValue(listOf("HardcodedColor")))
    }

    @Test
    fun `formatJavaSuppressValue emits brace array for multiple args`() {
        // Matches javac's expected SuppressWarnings literal form so the
        // file remains compilable after the rewrite.
        assertEquals(
            "{\"unchecked\", \"HardcodedColor\"}",
            KritSuppress.formatJavaSuppressValue(listOf("unchecked", "HardcodedColor")),
        )
    }

    @Test
    fun `stripQuotes strips matching double quotes only`() {
        assertEquals("HardcodedColor", KritSuppress.stripQuotes("\"HardcodedColor\""))
        assertEquals("HardcodedColor", KritSuppress.stripQuotes("HardcodedColor"))
        assertEquals("\"unbalanced", KritSuppress.stripQuotes("\"unbalanced"))
    }
}
