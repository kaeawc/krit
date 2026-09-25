package dev.jasonpearson.krit.types

import java.nio.file.Files
import kotlin.io.path.createTempDirectory
import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * A daemon session parses each source file once. Before it checked the
 * sources on disk, a request made after Lib.kt changed was answered from
 * Lib.kt's old content, so Use.kt's re-analysis still saw `helper(): R?`.
 */
class DaemonSourceRefreshTest {
    @Test
    fun requestAfterASourceEditSeesTheNewContent() {
        val root = createTempDirectory("krit-kaa-daemon-refresh-")
        val lib = root.resolve("Lib.kt")
        val use = root.resolve("Use.kt")
        Files.writeString(lib, "package p\n\nclass R\nfun helper(): R? = R()\n")
        Files.writeString(use, "package p\n\nfun use() = helper()!!\n")
        val parsed = ParsedArgs(sourceDirs = listOf(root.toString()), classpath = emptyList(), jdkHome = null, output = null, daemon = true)
        var session = DaemonSession.build(parsed)
        try {
            fun analyzeUse(id: Int): RequestResult {
                val request = """{"id":$id,"method":"analyzeWithDeps","files":["$use"]}"""
                val result = handleRequestLine(request, session, parsed, System.currentTimeMillis()) {}
                if (result is RequestResult.SessionRebuilt) session = result.newSession
                return result
            }
            fun json(result: RequestResult): String = when (result) {
                is RequestResult.Response -> result.json
                is RequestResult.SessionRebuilt -> result.json
                else -> error("unexpected $result")
            }

            val first = analyzeUse(1)
            assertIs<RequestResult.Response>(first)
            assertFalse("UNNECESSARY_NOT_NULL_ASSERTION" in json(first), json(first))
            assertFalse(session.sourcesChanged())

            Files.writeString(lib, "package p\n\nclass R\nfun helper(): R = R()\n")
            assertTrue(session.sourcesChanged())
            val second = analyzeUse(2)
            assertTrue("UNNECESSARY_NOT_NULL_ASSERTION" in json(second), "stale Lib.kt: ${json(second)}")
            assertIs<RequestResult.SessionRebuilt>(second)

            // The rebuilt session is current again: no second rebuild.
            assertIs<RequestResult.Response>(analyzeUse(3))
        } finally {
            session.dispose()
            root.toFile().deleteRecursively()
        }
    }
}
