package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * Pins the krit-fir one-shot CLI argv parser. The parser is small
 * but it lives on the hot path `oracle.InvokeWithFilesWithOptions`
 * shells out through, so a regression that drops the `--classpath`
 * flag would silently break type resolution in every project that
 * relies on a non-default classpath.
 */
class OneShotCliTest {

    @Test
    fun extractCliValueReturnsFirstMatchingFlag() {
        val args = arrayOf(
            "--sources", "/a,/b",
            "--output", "/tmp/out.json",
            "--classpath", "/jars/x.jar:/jars/y.jar",
        )
        assertEquals("/a,/b", extractCliValue(args, "--sources"))
        assertEquals("/tmp/out.json", extractCliValue(args, "--output", "-o"))
        assertEquals("/jars/x.jar:/jars/y.jar", extractCliValue(args, "--classpath", "-cp"))
    }

    @Test
    fun extractCliValueAcceptsAliasFlags() {
        // Short alias matches just like the long form so the parser
        // is symmetric with krit-types' `--output` / `-o` and
        // `--classpath` / `-cp`.
        val args = arrayOf("-o", "/tmp/out.json", "-cp", "/jars/single.jar")
        assertEquals("/tmp/out.json", extractCliValue(args, "--output", "-o"))
        assertEquals("/jars/single.jar", extractCliValue(args, "--classpath", "-cp"))
    }

    @Test
    fun extractCliValueReturnsNullWhenFlagMissing() {
        val args = arrayOf("--sources", "/a")
        assertNull(extractCliValue(args, "--classpath"))
    }

    @Test
    fun extractCliValueReturnsNullWhenFlagHasNoValue() {
        // A trailing flag with no value reads past the end of args
        // — the parser must not pick up an out-of-bounds value.
        val args = arrayOf("--sources", "/a", "--classpath")
        assertNull(extractCliValue(args, "--classpath"))
    }
}
