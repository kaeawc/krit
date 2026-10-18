package spike

import com.intellij.openapi.Disposable
import com.intellij.openapi.util.Disposer
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
    fun build(specs: List<ModSpec>, extraClasspath: List<String> = emptyList(), reuse: Libs? = null, plugin: Boolean = true): Built {
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
                        if (s.files != null) addSourceRoots(s.files.map { Path(it) }) else addSourceRoots(s.roots.map { Path(it) })
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
        for ((_, list) in session.modulesWithFiles) for (f in list) if (f is KtFile) files[f.virtualFile.path] = f
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
            out[p] = checkFile(kt)
        }
        return out
    }

    fun checkFile(kt: KtFile): FileDiags {
        val keys = HashSet<String>()
        var errors = 0
        var krit = 0
        analyze(kt) {
            for (d in kt.collectDiagnostics(KaDiagnosticCheckerFilter.EXTENDED_AND_COMMON_CHECKERS)) {
                val off = d.textRanges.firstOrNull()?.startOffset ?: -1
                keys += "${d.factoryName}|${d.severity}|$off|${d.defaultMessage}"
                if (d.severity.name == "ERROR") errors++
                if (d.factoryName == "KRIT_RULE") krit++
            }
        }
        return FileDiags(keys, errors, krit)
    }
}
