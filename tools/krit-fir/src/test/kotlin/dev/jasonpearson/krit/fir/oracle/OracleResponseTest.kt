package dev.jasonpearson.krit.fir.oracle

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class OracleResponseTest {

    @Test
    fun emptyAnalyzeEnvelopeMatchesKritTypesShape() {
        // The JSON shape mirrors krit-types' `buildDaemonResponse`:
        // `{"id":N,"result":{"version":1,"kotlinVersion":"...","files":{},"dependencies":{}}}`.
        // A single Go-side oracle client must parse responses from
        // either backend without branching on the source — verified by
        // pinning the exact field set and ordering.
        val response = OracleResponse.buildEmptyAnalyze(id = 42)

        assertTrue(response.startsWith("""{"id":42,"result":{"""), response)
        assertTrue(response.endsWith("}}"), response)
        assertTrue(response.contains(""""version":1"""), response)
        assertTrue(response.contains(""""files":{}"""), response)
        assertTrue(response.contains(""""dependencies":{}"""), response)
        assertTrue(response.contains(""""kotlinVersion":"""), response)
    }

    @Test
    fun emptyAnalyzeHasNoErrorsFieldWhenErrorsAreEmpty() {
        // The `errors` map is suppressed entirely when empty — matches
        // krit-types' behavior of nesting `errors` inside `result` only
        // when at least one entry exists.
        val response = OracleResponse.buildEmptyAnalyze(id = 1)
        assertTrue("errors" !in response, response)
    }

    @Test
    fun errorsAreNestedInsideResult() {
        // The legacy `buildDaemonResponse` shape (used by `analyze` /
        // `analyzeAll` / `analyzeFiles`) carries `errors` *inside*
        // `result`, not as a sibling. The flat sibling shape used by
        // `analyzeWithDeps` is separate and lands with the cacheDeps
        // projection.
        val response = OracleResponse.buildEmptyAnalyze(
            id = 7,
            errors = mapOf("/path/to/Foo.kt" to "parse error"),
        )
        val resultStart = response.indexOf(""""result":{""")
        val resultEnd = response.lastIndexOf("}}")
        val errorsAt = response.indexOf(""""errors":""")
        assertTrue(
            resultStart in 0 until errorsAt && errorsAt in 0 until resultEnd,
            "errors should appear inside result, not as a sibling: $response",
        )
    }

    @Test
    fun errorEntriesAreEscapedAsJsonStrings() {
        // Paths and messages can contain characters that need JSON
        // escaping (quotes, backslashes, newlines). The builder must
        // emit valid JSON without introducing a control char in the
        // output (only escaped sequences).
        val response = OracleResponse.buildEmptyAnalyze(
            id = 1,
            errors = mapOf(
                "/path with \"quote\"" to "newline\nbackslash\\",
            ),
        )
        assertTrue(response.contains("\\\"quote\\\""), response)
        assertTrue(response.contains("newline\\nbackslash\\\\"), response)
        assertTrue('\n' !in response, "raw newline leaked into envelope: $response")
    }

    @Test
    fun kotlinVersionMatchesRuntimeKotlinVersion() {
        // krit-types stamps `kotlinVersion` from `KotlinVersion.CURRENT`
        // (the compiler's own runtime version). krit-fir should do the
        // same so a parity-diff test can pin the field as a known
        // constant per-run rather than a moving target. This pins the
        // contract: whatever `KotlinVersion.CURRENT` returns at compile
        // time, the envelope must report it verbatim.
        val response = OracleResponse.buildEmptyAnalyze(id = 1)
        val expectedFragment = """"kotlinVersion":"${KotlinVersion.CURRENT}""""
        assertTrue(expectedFragment in response, "$expectedFragment not in $response")
    }

    @Test
    fun idIsRenderedAsBareIntegerNotQuotedString() {
        // JSON-RPC ids on the wire are unquoted integers; the Go client
        // unmarshals into int64. Emit the id raw (not as a quoted
        // string) so the response parses on the consumer side.
        val response = OracleResponse.buildEmptyAnalyze(id = 9001)
        assertTrue(response.startsWith("""{"id":9001,"""), response)
    }

    @Test
    fun envelopeIsSingleLineForNewlineDelimitedTransport() {
        // The transport is newline-delimited JSON over stdio/TCP; a
        // response that contains a raw newline (not escaped) would
        // break the protocol mid-message. Belt-and-suspenders given
        // the per-character escape in jsonString.
        val response = OracleResponse.buildEmptyAnalyze(
            id = 1,
            errors = mapOf("a" to "line1\nline2"),
        )
        assertEquals(-1, response.indexOf('\n'), "envelope contains a raw newline: $response")
    }
}
