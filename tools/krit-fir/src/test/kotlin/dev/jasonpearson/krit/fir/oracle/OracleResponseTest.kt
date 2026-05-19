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
        // either backend without branching on the source — pin the
        // exact field set so future projection work cannot accidentally
        // change the envelope contract.
        val response = OracleResponse.buildAnalyze(id = 42)

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
        val response = OracleResponse.buildAnalyze(id = 1)
        assertTrue("errors" !in response, response)
    }

    @Test
    fun errorsAreNestedInsideResult() {
        // The legacy `buildDaemonResponse` shape (used by `analyze` /
        // `analyzeAll` / `analyzeFiles`) carries `errors` *inside*
        // `result`, not as a sibling. The flat sibling shape used by
        // `analyzeWithDeps` is separate and lands with the cacheDeps
        // projection.
        val response = OracleResponse.buildAnalyze(
            id = 7,
            result = AnalyzeResult(errors = mapOf("/path/to/Foo.kt" to "parse error")),
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
        val response = OracleResponse.buildAnalyze(
            id = 1,
            result = AnalyzeResult(
                errors = mapOf("/path with \"quote\"" to "newline\nbackslash\\"),
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
        // constant per-run rather than a moving target.
        val response = OracleResponse.buildAnalyze(id = 1)
        val expectedFragment = """"kotlinVersion":"${KotlinVersion.CURRENT}""""
        assertTrue(expectedFragment in response, "$expectedFragment not in $response")
    }

    @Test
    fun idIsRenderedAsBareIntegerNotQuotedString() {
        // JSON-RPC ids on the wire are unquoted integers; the Go client
        // unmarshals into int64. Emit the id raw (not as a quoted
        // string) so the response parses on the consumer side.
        val response = OracleResponse.buildAnalyze(id = 9001)
        assertTrue(response.startsWith("""{"id":9001,"""), response)
    }

    @Test
    fun envelopeIsSingleLineForNewlineDelimitedTransport() {
        // The transport is newline-delimited JSON over stdio/TCP; a
        // response that contains a raw newline (not escaped) would
        // break the protocol mid-message. Belt-and-suspenders given
        // the per-character escape in jsonString.
        val response = OracleResponse.buildAnalyze(
            id = 1,
            result = AnalyzeResult(errors = mapOf("a" to "line1\nline2")),
        )
        assertEquals(-1, response.indexOf('\n'), "envelope contains a raw newline: $response")
    }

    @Test
    fun populatedFilePayloadSerializesPackageAndDeclarations() {
        // Seeded data pins the JSON shape produced when a non-empty
        // `FilePayload` rides on the wire — the projection layer hands
        // an `AnalyzeResult` to `buildAnalyze` and the envelope must
        // surface each populated field in the documented place.
        val response = OracleResponse.buildAnalyze(
            id = 1,
            result = AnalyzeResult(
                files = mapOf(
                    "/src/Foo.kt" to FilePayload(
                        packageName = "com.acme.foo",
                        declarations = listOf(
                            ClassPayload(
                                fqn = "com.acme.foo.Bar",
                                kind = "class",
                                supertypes = listOf("kotlin.Any"),
                                visibility = "public",
                                isOpen = true,
                            ),
                        ),
                    ),
                ),
            ),
        )
        assertTrue(""""/src/Foo.kt":{""" in response, response)
        assertTrue(""""package":"com.acme.foo"""" in response, response)
        assertTrue(""""fqn":"com.acme.foo.Bar"""" in response, response)
        assertTrue(""""kind":"class"""" in response, response)
        assertTrue(""""supertypes":["kotlin.Any"]""" in response, response)
        assertTrue(""""isOpen":true""" in response, response)
        // Modifier flags are emitted only when true — false flags must
        // not bloat the wire payload.
        assertTrue("isSealed" !in response, response)
        assertTrue("isData" !in response, response)
    }

    @Test
    fun dependenciesMapKeyedByFqn() {
        // `dependencies` is the cross-file class index — keyed by FQN
        // rather than by source path. krit-types uses this for Go-side
        // symbol resolution; the shape must round-trip identically.
        val response = OracleResponse.buildAnalyze(
            id = 1,
            result = AnalyzeResult(
                dependencies = mapOf(
                    "com.acme.foo.Bar" to ClassPayload(
                        fqn = "com.acme.foo.Bar",
                        kind = "class",
                    ),
                ),
            ),
        )
        assertTrue(""""dependencies":{"com.acme.foo.Bar":""" in response, response)
    }

    @Test
    fun analyzeWithDepsEmitsFlatEnvelopeWithCacheDeps() {
        // The `analyzeWithDeps` shape moves errors out of `result` into a
        // sibling key and always emits `cacheDeps` (even empty) so the
        // Go-side client can detect new-protocol daemons.
        val response = OracleResponse.buildAnalyzeWithDeps(
            id = 4,
            result = AnalyzeResult(
                errors = mapOf("/src/Bad.kt" to "parse error"),
            ),
        )
        assertTrue(response.startsWith("""{"id":4,"result":{"""), response)
        assertTrue(""""cacheDeps":{"version":1,"approximation":"symbol-resolved-sources","files":{},"crashed":{}}""" in response, response)
        // errors is a sibling of result, not nested inside it.
        val resultEnd = response.indexOf("}", response.indexOf(""""result":{"""))
        val errorsAt = response.indexOf(""""errors":""")
        assertTrue(
            errorsAt > resultEnd,
            "errors should be a sibling of result, not nested: $response",
        )
    }

    @Test
    fun analyzeWithDepsOmitsErrorsKeyWhenEmpty() {
        val response = OracleResponse.buildAnalyzeWithDeps(id = 5)
        assertTrue("errors" !in response, response)
        assertTrue(""""cacheDeps":{"version":1,"approximation":"symbol-resolved-sources","files":{},"crashed":{}}""" in response, response)
    }

    @Test
    fun analyzeWithDepsSerializesCacheDepsView() {
        // Populated dep tracker → cacheDeps carries per-file
        // depPaths + perFileDeps + crashed maps. Pins the wire shape
        // krit-types' `buildCacheDepsJson` produces so the Go-side
        // cache layer parses either backend's payload with one struct.
        val response = OracleResponse.buildAnalyzeWithDeps(
            id = 6,
            result = AnalyzeResult.EMPTY,
            cacheDeps = OracleResponse.CacheDepsView(
                depPathsByFile = mapOf("/src/Leaf.kt" to setOf("/src/Base.kt")),
                perFileDeps = mapOf(
                    "/src/Leaf.kt" to mapOf(
                        "com.acme.Base" to ClassPayload(fqn = "com.acme.Base", kind = "class"),
                    ),
                ),
                crashedFiles = mapOf("/src/Broken.kt" to "boom"),
            ),
        )
        assertTrue(""""cacheDeps":{"version":1,"approximation":"symbol-resolved-sources"""" in response, response)
        assertTrue(""""/src/Leaf.kt":{"depPaths":["/src/Base.kt"],"perFileDeps":{"com.acme.Base":""" in response, response)
        assertTrue(""""crashed":{"/src/Broken.kt":"boom"}""" in response, response)
    }

    @Test
    fun classAndMemberAnnotationsRideOnTheWire() {
        val response = OracleResponse.buildAnalyze(
            id = 1,
            result = AnalyzeResult(
                files = mapOf(
                    "/src/Foo.kt" to FilePayload(
                        declarations = listOf(
                            ClassPayload(
                                fqn = "com.acme.Foo",
                                kind = "class",
                                annotations = listOf("kotlin.Deprecated"),
                                members = listOf(
                                    MemberPayload(
                                        name = "doStuff",
                                        kind = "function",
                                        returnType = "kotlin.Unit",
                                        annotations = listOf("kotlin.jvm.JvmStatic"),
                                    ),
                                ),
                            ),
                        ),
                    ),
                ),
            ),
        )
        assertTrue(""""annotations":["kotlin.Deprecated"]""" in response, response)
        assertTrue(""""annotations":["kotlin.jvm.JvmStatic"]""" in response, response)
    }

    @Test
    fun diagnosticWireShapeMatchesKritTypes() {
        // krit-types emits `{factoryName, severity, message, line, col,
        // startByte?, endByte?}` for each diagnostic. Pinning the field
        // names + omit-when-empty byte range guards against accidentally
        // diverging from the consumer's parser shape.
        val response = OracleResponse.buildAnalyze(
            id = 1,
            result = AnalyzeResult(
                files = mapOf(
                    "/src/Foo.kt" to FilePayload(
                        diagnostics = listOf(
                            DiagnosticPayload(
                                factoryName = "USELESS_ELVIS",
                                severity = "WARNING",
                                message = "Elvis operator (?:) always returns the left operand of non-nullable type",
                                line = 3,
                                col = 7,
                                startByte = 10,
                                endByte = 22,
                            ),
                            DiagnosticPayload(
                                factoryName = "CAST_NEVER_SUCCEEDS",
                                severity = "WARNING",
                                message = "cast can never succeed",
                                line = 5,
                                col = 1,
                            ),
                        ),
                    ),
                ),
            ),
        )
        assertTrue(""""factoryName":"USELESS_ELVIS"""" in response, response)
        assertTrue(""""col":7""" in response, response)
        assertTrue(""""startByte":10,"endByte":22""" in response, response)
        // The zero-range diagnostic must omit startByte/endByte to keep
        // the wire payload minimal (krit-types' rule).
        assertTrue(""""factoryName":"CAST_NEVER_SUCCEEDS","severity":"WARNING"""" in response, response)
        val secondDiagFragment = response.substringAfter("CAST_NEVER_SUCCEEDS")
        assertTrue(
            "startByte" !in secondDiagFragment,
            "zero-range diagnostic must omit startByte: $response",
        )
    }

    @Test
    fun expressionPayloadOmitsZeroByteRangeAndFalseFlags() {
        // Expressions ride on every analyze response and can be dense
        // on real codebases. Emit only fields that carry information:
        // suppress the byte range when start == end, suppress flag
        // fields when false. Matches krit-types' `buildJson` shape so
        // a Go-side parity diff stays comparable.
        val response = OracleResponse.buildAnalyze(
            id = 1,
            result = AnalyzeResult(
                files = mapOf(
                    "/src/Foo.kt" to FilePayload(
                        packageName = "com.acme",
                        expressions = mapOf(
                            "12:5" to ExpressionPayload(
                                type = "kotlin.String",
                                nullable = false,
                            ),
                        ),
                    ),
                ),
            ),
        )
        assertTrue(""""12:5":{"type":"kotlin.String","nullable":false}""" in response, response)
        assertTrue("startByte" !in response, response)
        assertTrue("callTargetSuspend" !in response, response)
    }
}
