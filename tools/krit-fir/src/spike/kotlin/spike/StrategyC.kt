package spike

import com.intellij.openapi.vfs.VirtualFile
import com.intellij.openapi.vfs.VirtualFileSystem
import com.intellij.psi.PsiManager
import com.intellij.psi.impl.source.tree.CompositeElement
import com.intellij.psi.impl.source.tree.TreeElement
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.platform.modification.KaElementModificationType
import org.jetbrains.kotlin.analysis.api.platform.modification.KaSourceModificationService
import org.jetbrains.kotlin.analysis.api.platform.modification.KotlinModuleStateModificationKind
import org.jetbrains.kotlin.analysis.api.platform.modification.handleElementModification
import org.jetbrains.kotlin.analysis.api.platform.modification.publishGlobalSourceModuleStateModificationEvent
import org.jetbrains.kotlin.analysis.api.platform.modification.publishGlobalSourceOutOfBlockModificationEvent
import org.jetbrains.kotlin.analysis.api.platform.modification.publishModuleOutOfBlockModificationEvent
import org.jetbrains.kotlin.analysis.api.platform.modification.publishModuleStateModificationEvent
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtNamedFunction
import org.jetbrains.kotlin.psi.psiUtil.collectDescendantsOfType
import org.jetbrains.kotlin.psi.KtPsiFactory
import java.io.File

/**
 * Strategy C probe on the current Analysis API: can a built standalone session be told that one source file
 * changed, instead of rebuilding the session? Each probe applies the same on-disk edit, then reports whether the
 * diagnostics of the edited file match a fresh session over the edited tree.
 */
@OptIn(KaExperimentalApi::class, org.jetbrains.kotlin.analysis.api.KaPlatformInterface::class)
class StrategyC(val o: Map<String, String>) {
    private fun summarize(d: FileDiags) = "krit=${d.krit} errors=${d.errors} total=${d.keys.size}"

    fun run() {
        val n = 200
        val dir = File("/tmp/spike740/stratc").also { it.deleteRecursively() }
        val g = Gen(dir, n); g.generate()
        val specs = listOf(ModSpec("lib", listOf(g.libDir.path)), ModSpec("app", listOf(g.appDir.path), listOf("lib")))
        val path = g.appPath(10)
        val original = File(path).readText()
        val bodyEdit = original.replace("fun body(): Int { val t = 3; return t * 3 }", "fun body(): Int { println(\"x\"); return 9 }")
        val sigEdit = original.replace("    fun body()", "    fun added(): Int { println(\"y\"); return 1 }\n    fun body()")
        check(bodyEdit != original && sigEdit != original)

        for ((label, edited) in listOf("body-edit" to bodyEdit, "member-added" to sigEdit)) {
            println("=== $label")
            File(path).writeText(original)
            val b = Runner.build(specs)
            val project = b.session.project
            val mod = b.session.modulesWithFiles.keys.first { it.name == "app" }
            val kt = b.files.getValue(path)
            val base = Runner.checkFile(kt)
            println("baseline: ${summarize(base)}")
            File(path).writeText(edited)
            val fb = Runner.build(specs)
            val want = Runner.checkFile(fb.files.getValue(path))
            fb.dispose()
            println("fresh-session expectation after edit: ${summarize(want)}")

            fun probe(name: String, f: () -> Unit) {
                try {
                    f()
                    val got = Runner.checkFile(b.files.getValue(path))
                    println("$name -> ${summarize(got)} matchesFresh=${got.keys == want.keys}")
                } catch (t: Throwable) {
                    println("$name -> THROWS ${t.javaClass.simpleName}: ${t.message?.lineSequence()?.firstOrNull()}")
                    if (o["trace"] == "true") t.stackTrace.take(14).forEach { println("      at $it") }
                }
            }
            // P1: events only (supported API). PSI still holds the old text.
            probe("P1 events only (module OOB + module state UPDATE + global source OOB)") {
                mod.publishModuleOutOfBlockModificationEvent()
                mod.publishModuleStateModificationEvent(KotlinModuleStateModificationKind.UPDATE)
                project.publishGlobalSourceOutOfBlockModificationEvent()
            }
            // P2: re-resolve the PSI from the file system; does the session see the new text?
            probe("P2 re-read PSI via PsiManager.findFile(virtualFile) (does the session own the new tree?)") {
                val vf = kt.virtualFile
                val again = PsiManager.getInstance(project).findFile(vf) as KtFile
                println("   same PSI instance=${again === kt}; text contains edit=${again.text.contains(if (label == "body-edit") "println" else "added")}")
            }
            // P3: in-place raw AST replacement + KaSourceModificationService, no session rebuild.
            probe("P3 raw AST replace + handleElementModification(Unknown) + module OOB event") {
                val fresh = KtPsiFactory(project, false).createFile(File(path).name, edited)
                val node = kt.node as CompositeElement
                node.rawRemoveAllChildren()
                var c = fresh.node.firstChildNode
                val chain = ArrayList<TreeElement>()
                while (c != null) { chain += c as TreeElement; c = c.treeNext }
                for (e in chain) { e.rawRemove(); node.rawAddChildren(e) }
                KaSourceModificationService.getInstance(project).handleElementModification(kt, KaElementModificationType.Unknown)
                mod.publishModuleOutOfBlockModificationEvent()
            }
            probe("P3w WritableVf hack + whole-file raw replace + modification service + module OOB event") {
                makeWritable(kt)
                val fresh = KtPsiFactory(project, false).createFile(File(path).name, edited)
                val node = kt.node as CompositeElement
                node.rawRemoveAllChildren()
                var c = fresh.node.firstChildNode
                val chain = ArrayList<TreeElement>()
                while (c != null) { chain += c as TreeElement; c = c.treeNext }
                for (e in chain) { e.rawRemove(); node.rawAddChildren(e) }
                KaSourceModificationService.getInstance(project).handleElementModification(kt, KaElementModificationType.Unknown)
                mod.publishModuleOutOfBlockModificationEvent()
            }
            probe("P4 + global module-state invalidation") {
                project.publishGlobalSourceModuleStateModificationEvent()
            }
            // P5: supported route for unsaved content: an in-memory LightVirtualFile in a rebuilt (slice) session.
            run {
                File(path).writeText(original) // disk holds the OLD text; only the overlay carries the edit
                val slice = listOf(path, g.libPath(1), g.libPath(2), g.libPath(3)).distinct()
                val all = (g.texts.keys).toList()
                val m = Runner.build(listOf(ModSpec("lib", emptyList(), files = all.filter { it.startsWith(g.libDir.path) }), ModSpec("app", emptyList(), listOf("lib"), files = all.filter { it.startsWith(g.appDir.path) })), overlay = mapOf(path to edited))
                val got = Runner.checkFile(m.files.getValue(path))
                println("P5 LightVirtualFile overlay (unsaved buffer) in rebuilt session -> ${summarize(got)} matchesFresh=${got.keys == want.keys}")
                m.dispose()
            }
            b.dispose()
        }
        dir.deleteRecursively()
    }
}

/** Delegating VirtualFile that reports itself writable so raw PSI mutation passes PsiFileImpl.checkWritable. */
class WritableVf(val d: VirtualFile) : VirtualFile() {
    override fun getName(): String = d.name
    override fun getFileSystem(): VirtualFileSystem = d.fileSystem
    override fun getPath(): String = d.path
    override fun isWritable(): Boolean = true
    override fun isDirectory(): Boolean = d.isDirectory
    override fun isValid(): Boolean = d.isValid
    override fun getParent(): VirtualFile? = d.parent
    override fun getChildren(): Array<VirtualFile> = d.children
    override fun getOutputStream(requestor: Any?, newModificationStamp: Long, newTimeStamp: Long) = d.getOutputStream(requestor, newModificationStamp, newTimeStamp)
    override fun contentsToByteArray(): ByteArray = d.contentsToByteArray()
    override fun getTimeStamp(): Long = d.timeStamp
    override fun getLength(): Long = d.length
    override fun refresh(asynchronous: Boolean, recursive: Boolean, postRunnable: Runnable?) = d.refresh(asynchronous, recursive, postRunnable)
    override fun getInputStream() = d.inputStream
    override fun equals(other: Any?): Boolean = d == (if (other is WritableVf) other.d else other)
    override fun hashCode(): Int = d.hashCode()
}

/** Reflection hack: make the file's view provider hold a writable virtual file. Unsupported API surface. */
fun makeWritable(kt: KtFile) {
    val vp = kt.viewProvider
    var c: Class<*>? = vp.javaClass
    while (c != null && c.simpleName != "AbstractFileViewProvider") c = c.superclass
    val f = c!!.getDeclaredField("myVirtualFile").apply { isAccessible = true }
    val cur = f.get(vp) as VirtualFile
    if (cur !is WritableVf) f.set(vp, WritableVf(cur))
}
