package dev.jasonpearson.krit.intellij

import java.io.File
import java.nio.file.Files
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNull

class KritRuleCatalogTest {
    private lateinit var dir: File
    private lateinit var binary: File
    private val calls = mutableListOf<File>()
    private var output: String? = ruleJson("A", "B")

    private fun ruleJson(vararg ids: String): String =
        KritRuleList.toJson(ids.map { KritRuleInfo(id = it, category = "style", severity = "warning") })

    private fun catalog(cacheDir: File? = File(dir, "cache")) = KritRuleCatalog(cacheDir) { bin ->
        calls += bin
        output
    }

    @BeforeTest
    fun setUp() {
        dir = Files.createTempDirectory("krit-rule-catalog").toFile()
        binary = File(dir, "krit").apply {
            writeText("v1")
            setLastModified(1_700_000_000_000)
        }
    }

    @AfterTest
    fun tearDown() {
        dir.deleteRecursively()
    }

    @Test
    fun `runs krit once per binary and serves repeats from memory`() {
        val catalog = catalog()

        assertEquals(listOf("A", "B"), catalog.rulesFor(binary)!!.map { it.id })
        assertEquals(listOf("A", "B"), catalog.rulesFor(binary)!!.map { it.id })

        assertEquals(1, calls.size)
    }

    @Test
    fun `a new catalog reads the disk cache instead of running krit`() {
        catalog().rulesFor(binary)
        calls.clear()
        output = null // krit would fail now; the cache must answer

        val rules = catalog().rulesFor(binary)

        assertEquals(listOf("A", "B"), rules!!.map { it.id })
        assertEquals(0, calls.size)
        assertEquals(listOf("A", "B"), catalog().cachedRulesFor(binary)!!.map { it.id })
    }

    @Test
    fun `replacing the binary invalidates the cached list`() {
        val catalog = catalog()
        catalog.rulesFor(binary)

        binary.writeText("v2, a longer binary")
        binary.setLastModified(1_800_000_000_000)
        output = ruleJson("A", "B", "C")

        assertEquals(listOf("A", "B", "C"), catalog.rulesFor(binary)!!.map { it.id })
        assertEquals(2, calls.size)
        // And a fresh catalog (an IDE restart) doesn't resurrect the old list.
        assertEquals(listOf("A", "B", "C"), catalog().rulesFor(binary)!!.map { it.id })
        assertEquals(2, calls.size)
    }

    @Test
    fun `a touched binary with the same size is refetched`() {
        val catalog = catalog()
        catalog.rulesFor(binary)
        binary.setLastModified(1_700_000_100_000)

        catalog.rulesFor(binary)

        assertEquals(2, calls.size)
    }

    @Test
    fun `different binary paths are cached separately`() {
        val other = File(dir, "other-krit").apply {
            writeText("v1")
            setLastModified(binary.lastModified())
        }
        val catalog = catalog()
        catalog.rulesFor(binary)
        catalog.rulesFor(other)

        assertEquals(listOf(binary, other), calls)
        assertNotEquals(KritRuleCatalog.Key.of(binary).fileName(), KritRuleCatalog.Key.of(other).fileName())
    }

    @Test
    fun `refresh drops the cached list and runs krit again`() {
        val catalog = catalog()
        catalog.rulesFor(binary)
        output = ruleJson("Z")

        assertEquals(listOf("Z"), catalog.refresh(binary)!!.map { it.id })
        assertEquals(listOf("Z"), catalog().rulesFor(binary)!!.map { it.id })
        assertEquals(2, calls.size)
    }

    @Test
    fun `a failed or non-JSON listing is not cached`() {
        output = "Available rules:\n"
        val catalog = catalog()

        assertNull(catalog.rulesFor(binary))
        output = ruleJson("A")
        assertEquals(listOf("A"), catalog.rulesFor(binary)!!.map { it.id })
        assertEquals(2, calls.size)
    }

    @Test
    fun `a corrupt or mismatched cache file is ignored`() {
        val cacheDir = File(dir, "cache")
        catalog(cacheDir).rulesFor(binary)
        val file = File(cacheDir, KritRuleCatalog.Key.of(binary).fileName())
        file.writeText("{not json")
        calls.clear()

        assertEquals(listOf("A", "B"), catalog(cacheDir).rulesFor(binary)!!.map { it.id })
        assertEquals(1, calls.size)

        // A file whose recorded key doesn't match (a hash collision, a
        // hand-copied cache) is not trusted either.
        file.writeText(file.readText().replace(binary.absolutePath.replace("\\", "\\\\"), "/elsewhere/krit"))
        calls.clear()
        catalog(cacheDir).rulesFor(binary)
        assertEquals(1, calls.size)
    }

    @Test
    fun `works without a disk cache`() {
        val catalog = catalog(cacheDir = null)
        catalog.rulesFor(binary)
        catalog.rulesFor(binary)
        assertEquals(1, calls.size)
        assertNull(catalog(cacheDir = null).cachedRulesFor(binary))
    }
}
