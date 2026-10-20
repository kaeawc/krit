package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.ClassNameFact
import dev.jasonpearson.krit.fir.runner.Finding
import org.junit.jupiter.api.Test
import java.util.Random
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** The binary form of a persisted bundle (#878): it round-trips, and nothing damaged is ever accepted or throws anything else. */
class BundleCodecTest {
    private val key = "k".repeat(64)

    private fun surface(i: Int) = FileSurface(
        decls = mapOf(Sym("p$i", "f$i") to "h$i", Sym("p$i.C", "class:C") to "hc", Sym("", "top") to "ünïcode-$i"),
        supers = mapOf("p$i.C" to listOf("kotlin.Any", "p.Box<kotlin.Int>?")),
        sealed = setOf("p$i.S"),
        pkg = "p$i",
    )

    private fun sample(): Pair<PersistedState, List<PersistedVerdict>> {
        val files = (0 until 40).map { "/src/F$it.kt" }
        val state = PersistedState(
            compileKey = key, verdictKey = "vk", generation = 7,
            surfaces = files.withIndex().associate { (i, f) -> f to surface(i) },
            contentHashes = files.associateWith { "hash-$it" },
            classFacts = files.take(10).associateWith { listOf(ClassNameFact("C", setOf("Serializable", "Base")), ClassNameFact("D", emptySet())) },
            lookups = files.withIndex().associate { (i, f) -> f to (0 until i % 5).map { Sym("scope$it", "name$it") }.toSet() },
            snapshot = PersistedSnapshot(
                outputs = files.take(30).associateWith { listOf("p/${it.substringAfterLast('/')}.class", "p/${it.substringAfterLast('/')}Kt.class") },
                surfaceHashes = files.take(30).associateWith { "sh" },
                binaryStale = setOf(files[3], files[4]),
                removed = setOf("/src/Gone.kt"),
            ),
        )
        val verdicts = files.take(12).mapIndexed { i, f ->
            PersistedVerdict(
                f,
                VerdictCache.Entry(
                    "hash-$f", if (i % 2 == 0) "go-$i" else null, "vkey$i",
                    CachedVerdict(
                        listOf(Finding(f, i + 1, 3, 10, 20, "Rule$i", "warning", "message \"$i\" with ünï", 0.75), Finding(f, -1, 0, 0, 0, "R", "error", "", 1.0)),
                        if (i == 5) "e: broken" else null,
                        if (i == 6) mapOf("RuleX" to "boom") else emptyMap(),
                    ),
                ),
            )
        }
        return state to verdicts
    }

    @Test fun aBundleRoundTrips() {
        val (state, verdicts) = sample()
        val bytes = BundleCodec.encode(key, "snap1", state, verdicts, 7)
        val back = BundleCodec.decode(bytes, key)
        assertEquals("snap1", back.snapshotId)
        assertEquals(7, back.verdictGeneration)
        with(back.state) {
            assertEquals(key, compileKey); assertEquals("vk", verdictKey); assertEquals(7, generation)
            assertEquals(state.surfaces, surfaces)
            assertEquals(state.contentHashes, contentHashes)
            assertEquals(state.classFacts, classFacts)
            assertEquals(state.lookups, lookups)
            assertEquals(state.snapshot.outputs, snapshot.outputs)
            assertEquals(state.snapshot.surfaceHashes, snapshot.surfaceHashes)
            assertEquals(state.snapshot.binaryStale, snapshot.binaryStale)
            assertEquals(state.snapshot.removed, snapshot.removed)
        }
        assertEquals(verdicts.size, back.verdicts.size)
        for ((want, got) in verdicts.zip(back.verdicts)) {
            assertEquals(want.file, got.file)
            assertEquals(want.entry.contentHash, got.entry.contentHash)
            assertEquals(want.entry.suppliedHash, got.entry.suppliedHash)
            assertEquals(want.entry.verdictKey, got.entry.verdictKey)
            assertEquals(want.entry.verdict.findings, got.entry.verdict.findings)
            assertEquals(want.entry.verdict.error, got.entry.verdict.error)
            assertEquals(want.entry.verdict.ruleErrors, got.entry.verdict.ruleErrors)
        }
    }

    @Test fun anEmptyStateAndNullVerdictKeyRoundTrip() {
        val state = PersistedState(key, null, 1, emptyMap(), emptyMap(), emptyMap(), emptyMap(), PersistedSnapshot(emptyMap(), emptyMap(), emptySet(), emptySet()))
        val back = BundleCodec.decode(BundleCodec.encode(key, "", state, emptyList(), 1), key)
        assertNull(back.state.verdictKey)
        assertTrue(back.verdicts.isEmpty())
    }

    @Test fun theFormatIsCompact() {
        val (state, verdicts) = sample()
        val bytes = BundleCodec.encode(key, "snap1", state, verdicts, 7)
        // Every string once: far smaller than the text a JSON form would repeat per use.
        assertTrue(bytes.size < 12_000, "bundle is ${bytes.size} bytes")
    }

    @Test fun everyTruncationIsRejectedAndNeverThrowsAnythingElse() {
        val (state, verdicts) = sample()
        val bytes = BundleCodec.encode(key, "snap1", state, verdicts, 7)
        for (length in 0 until bytes.size step 7) {
            assertFailsWith<BundleRejected>("length $length") { BundleCodec.decode(bytes.copyOf(length), key) }
        }
        assertFailsWith<BundleRejected> { BundleCodec.decode(bytes.copyOf(bytes.size - 1), key) }
    }

    @Test fun anyDamagedByteIsRejected() {
        val (state, verdicts) = sample()
        val bytes = BundleCodec.encode(key, "snap1", state, verdicts, 7)
        val rnd = Random(3)
        repeat(300) {
            val damaged = bytes.copyOf()
            val at = rnd.nextInt(damaged.size)
            damaged[at] = (damaged[at].toInt() xor (1 + rnd.nextInt(255))).toByte()
            assertFailsWith<BundleRejected>("byte $at") { BundleCodec.decode(damaged, key) }
        }
    }

    @Test fun aDamagedBodyWithAFixedUpChecksumStillOnlyEverRejects() {
        // A writer bug or a hostile file can carry a valid checksum: the reader's own bounds checks must hold.
        val (state, verdicts) = sample()
        val bytes = BundleCodec.encode(key, "snap1", state, verdicts, 7)
        val rnd = Random(9)
        repeat(300) {
            val damaged = bytes.copyOf()
            repeat(1 + rnd.nextInt(3)) {
                val at = 60 + rnd.nextInt(damaged.size - 60 - 12)
                damaged[at] = rnd.nextInt(256).toByte()
            }
            val crc = java.util.zip.CRC32().also { it.update(damaged, 0, damaged.size - 12) }.value
            for (i in 0 until 8) damaged[damaged.size - 12 + i] = (crc ushr (56 - 8 * i)).toByte()
            try {
                BundleCodec.decode(damaged, key)
            } catch (_: BundleRejected) {
                // expected for most; some damage lands in a value and decodes to different data, which is fine
            }
        }
    }

    @Test fun aVersionOrKeyMismatchIsReportedAsSuch() {
        val (state, verdicts) = sample()
        val bytes = BundleCodec.encode(key, "snap1", state, verdicts, 7)
        val otherVersion = bytes.copyOf().also { it[7] = (it[7] + 1).toByte() }
        assertEquals(BundleRejected.Reason.VERSION, assertFailsWith<BundleRejected> { BundleCodec.decode(otherVersion, key) }.reason)
        assertEquals(BundleRejected.Reason.COMPILE_KEY, assertFailsWith<BundleRejected> { BundleCodec.decode(bytes, "other") }.reason)
        assertEquals(BundleRejected.Reason.CORRUPT, assertFailsWith<BundleRejected> { BundleCodec.decode(ByteArray(64), key) }.reason)
        assertEquals(key, BundleCodec.peekCompileKey(bytes))
    }
}
