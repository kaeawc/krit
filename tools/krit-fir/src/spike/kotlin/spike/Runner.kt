package spike

import com.intellij.openapi.Disposable
import com.intellij.openapi.util.Disposer
import com.intellij.testFramework.LightVirtualFile
import dev.jasonpearson.krit.fir.FirRuleErrorRecorder
import dev.jasonpearson.krit.fir.FirRuleErrors
import dev.jasonpearson.krit.fir.KritPluginRegistrar
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.analyze
import org.jetbrains.kotlin.analysis.api.components.KaDiagnosticCheckerFilter
import org.jetbrains.kotlin.analysis.api.components.collectDiagnostics
import org.jetbrains.kotlin.analysis.api.projectStructure.KaLibraryModule
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.analysis.api.platform.projectStructure.KotlinCompilerPluginsProvider
import org.jetbrains.kotlin.analysis.api.standalone.StandaloneAnalysisAPISession
import org.jetbrains.kotlin.analysis.api.standalone.base.declarations.KotlinStandaloneFirCompilerPluginsProvider
import org.jetbrains.kotlin.analysis.api.standalone.buildStandaloneAnalysisAPISession
import org.jetbrains.kotlin.analysis.project.structure.builder.KaModuleContainerBuilder
import org.jetbrains.kotlin.analysis.project.structure.builder.buildKtLibraryModule
import org.jetbrains.kotlin.analysis.project.structure.builder.buildKtSdkModule
import org.jetbrains.kotlin.analysis.project.structure.builder.buildKtSourceModule
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar
import org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi
import org.jetbrains.kotlin.cli.extensionsStorage
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.psi.KtFile
import java.io.File
import kotlin.io.path.Path

/** One Gradle-module-like unit: source roots (or an explicit file list) plus dependencies on earlier modules. */
class ModSpec(val name: String, val roots: List<String>, val deps: List<String> = emptyList(), val files: List<String>? = null)

/** Per-file check result in a comparable form. */
data class FileDiags(val keys: Set<String>, val errors: Int, val krit: Int) {
    val hasError get() = errors > 0
}

class Built(
    val disposable: Disposable,
    val session: StandaloneAnalysisAPISession,
    val files: Map<String, KtFile>,
    val libs: Libs?,
    val buildNanos: Long,
    val recorder: FirRuleErrorRecorder = FirRuleErrorRecorder(),
) {
    fun dispose() = Disposer.dispose(disposable)
}

/** JDK + stdlib modules that a "reuse" strategy tries to carry across sessions. */
class Libs(val jdk: org.jetbrains.kotlin.analysis.api.projectStructure.KaModule, val stdlib: KaLibraryModule)

@OptIn(KaExperimentalApi::class, ExperimentalCompilerApi::class, CompilerConfiguration.Internals::class, org.jetbrains.kotlin.analysis.api.KaPlatformInterface::class)
object Runner {
    val stdlibJar: String = System.getProperty("spike.stdlib")
    private val platform = JvmPlatforms.defaultJvmPlatform

    fun pluginConfig(): CompilerConfiguration = CompilerConfiguration().apply {
        add(CompilerPluginRegistrar.COMPILER_PLUGIN_REGISTRARS, KritPluginRegistrar())
        extensionsStorage = CompilerPluginRegistrar.ExtensionStorage()
    }

    /**
     * Build a fresh standalone session. [reuse] carries the JDK/stdlib modules of an earlier session into this one
     * (strategy B); null builds them fresh (strategy A).
     */
    fun build(specs: List<ModSpec>, extraClasspath: List<String> = emptyList(), reuse: Libs? = null, plugin: Boolean = true, overlay: Map<String, String> = emptyMap()): Built {
        val t0 = System.nanoTime()
        val disposable = Disposer.newDisposable("spike")
        var libsOut: Libs? = null
        lateinit var mods: Map<String, KaSourceModule>
        val session = buildStandaloneAnalysisAPISession(disposable) {
            if (plugin) registerProjectService(KotlinCompilerPluginsProvider::class.java, KotlinStandaloneFirCompilerPluginsProvider(pluginConfig()))
            buildKtModuleProvider {
                this.platform = Runner.platform
                val libs = reuse ?: makeLibs(extraClasspath)
                libsOut = libs
                val built = LinkedHashMap<String, KaSourceModule>()
                for (s in specs) {
                    val m = buildKtSourceModule {
                        this.platform = Runner.platform
                        moduleName = s.name
                        val own = s.files ?: emptyList()
                        if (s.files != null) {
                            addSourceRoots(own.filter { it !in overlay }.map { Path(it) })
                            for (p in own) overlay[p]?.let { addSourceVirtualFile(LightVirtualFile(File(p).name, org.jetbrains.kotlin.idea.KotlinFileType.INSTANCE, it)) }
                        } else addSourceRoots(s.roots.map { Path(it) })
                        addRegularDependency(libs.jdk)
                        addRegularDependency(libs.stdlib)
                        for (d in s.deps) addRegularDependency(built.getValue(d))
                    }
                    built[s.name] = m
                    addModule(m)
                }
                mods = built
            }
        }
        val files = HashMap<String, KtFile>()
        val byName = overlay.keys.associateBy { File(it).name }
        for ((_, list) in session.modulesWithFiles) for (f in list) if (f is KtFile) {
            val vf = f.virtualFile
            files[if (vf is LightVirtualFile) byName[vf.name] ?: vf.path else vf.path] = f
        }
        return Built(disposable, session, files, libsOut, System.nanoTime() - t0)
    }

    private fun KaModuleContainerBuilder.makeLibs(extra: List<String>): Libs {
        val jdk = buildKtSdkModule {
            addBinaryRootsFromJdkHome(Path(System.getProperty("java.home")), isJre = true)
            this.platform = Runner.platform
            libraryName = "jdk"
        }
        val std = buildKtLibraryModule {
            this.platform = Runner.platform
            addBinaryRoots((listOf(stdlibJar) + extra).map { Path(it) })
            libraryName = "stdlib"
        }
        return Libs(jdk, std)
    }

    /** Collect diagnostics of [paths] (all files when null). */
    fun check(b: Built, paths: Collection<String>? = null): Map<String, FileDiags> {
        val out = HashMap<String, FileDiags>()
        for (p in paths ?: b.files.keys) {
            val kt = b.files[p] ?: continue
            out[p] = checkFile(kt, b.recorder, p)
        }
        return out
    }

    @Volatile var filter: KaDiagnosticCheckerFilter = KaDiagnosticCheckerFilter.EXTENDED_AND_COMMON_CHECKERS

    fun checkFile(kt: KtFile, rec: FirRuleErrorRecorder = FirRuleErrorRecorder(), path: String = kt.virtualFile.path): FileDiags {
        // Rule isolation: the recorder is captured when the FIR session builds its checker extension (on first use),
        // so it must be current on this thread before the first analyze() of a session.
        FirRuleErrors.begin(rec)
        val keys = HashSet<String>()
        var errors = 0
        var krit = 0
        analyze(kt) {
            for (d in kt.collectDiagnostics(filter)) {
                val off = d.textRanges.firstOrNull()?.startOffset ?: -1
                keys += "${d.factoryName}|${d.severity}|$off|${d.defaultMessage}"
                if (d.severity.name == "ERROR") errors++
                if (d.factoryName == "KRIT_RULE") krit++
            }
        }
        val canon = try { File(path).canonicalPath } catch (_: Exception) { path }
        for ((rule, byPath) in rec.snapshot()) for (p in byPath.keys) {
            val cp = try { File(p).canonicalPath } catch (_: Exception) { p }
            if (p == path || cp == canon || (File(p).parent == "/" && File(p).name == File(path).name)) keys += "RULEERR|$rule"
        }
        return FileDiags(keys, errors, krit)
    }
}
