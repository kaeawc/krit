package spike

import com.intellij.psi.impl.source.PsiFileImpl
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtPsiFactory
import java.io.File

/**
 * Strategy E: keep ONE long-lived session over the module and analyze an edited file as a dangling copy
 * (KtFile.originalFile = the session's PSI) instead of rebuilding anything. Valid while the edit does not change
 * the file's declaration surface, so nothing else in the session is stale.
 */
@OptIn(KaExperimentalApi::class, org.jetbrains.kotlin.analysis.api.KaPlatformInterface::class)
class Dangling(val o: Map<String, String>) {
    fun danglingCopy(b: Built, path: String, text: String): KtFile {
        val orig = b.files.getValue(path)
        val copy = KtPsiFactory(b.session.project, false).createFile(File(path).name, text)
        (copy as PsiFileImpl).originalFile = orig
        return copy
    }

    fun run() {
        val n = (o["files"] ?: "2000").toInt()
        val edits = (o["edits"] ?: "60").toInt()
        val warm = (o["warmup"] ?: "10").toInt()
        val g = makeProj(o, java.io.File(o["dir"] ?: "/tmp/spike740/dang$n"), n, 17L)
        println("# ${g.describe()}")
        val b = Runner.build(g.specs(null), g.classpath)
        // warm the long-lived session the way a daemon would have: check a sample of files once
        val sample = b.files.keys.sorted().filterIndexed { i, _ -> i % 40 == 0 }
        var t = System.nanoTime(); Runner.check(b, sample)
        println("session warm-up: checked ${sample.size} files in ${ms(System.nanoTime() - t)} ms")
        // edit loop: body-only edits (kinds body / literal)
        val rows = ArrayList<Triple<Long, Long, Boolean>>()
        var mism = 0
        val kinds = if (g is Gen) listOf("body") else listOf("literal")
        for (i in 1..(warm + edits)) {
            val e = g.nextEdit(kinds)
            val p = e.changed.first()
            val text = File(p).readText()
            t = System.nanoTime(); val cpu0 = cpuNanos()
            val copy = danglingCopy(b, p, text)
            val got = Runner.checkFile(copy)
            val dt = System.nanoTime() - t
            val fresh = if (o["oracle"] != "false") {
                val fb = Runner.build(g.specs(null), g.classpath); val f = Runner.checkFile(fb.files.getValue(p)); fb.dispose(); f
            } else null
            val ok = fresh == null || fresh.keys == got.keys
            if (!ok) { mism++; println("  MISMATCH e$i ${File(p).name}: want=${fresh!!.keys.size} got=${got.keys.size} ${(fresh.keys - got.keys).take(1)} / ${(got.keys - fresh.keys).take(1)}") }
            if (i > warm) rows += Triple(dt, cpuNanos() - cpu0, ok)
            println("e$i body ${File(p).name} dangling-check=${ms(dt)} ms cpu=${ms(cpuNanos() - cpu0)} krit=${got.krit} errors=${got.errors} match=$ok load=${load1()}")
        }
        val tot = rows.map { it.first }
        println("## dangling steady-state n=${rows.size} p50=${ms(pct(tot, .5))} p95=${ms(pct(tot, .95))} max=${ms(tot.max())} ms; cpu p50=${ms(pct(rows.map { it.second }, .5))}; mismatches=$mism")
        println("## heap after GC: ${memSnap(true)}")
        b.dispose()
        if (o["keep"] != "true") java.io.File(o["dir"] ?: "/tmp/spike740/dang$n").deleteRecursively()
    }
}
