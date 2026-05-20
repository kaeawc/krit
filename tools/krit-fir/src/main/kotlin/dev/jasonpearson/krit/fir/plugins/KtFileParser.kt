package dev.jasonpearson.krit.fir.plugins

import com.intellij.openapi.util.Disposer
import org.jetbrains.kotlin.K1Deprecation
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment
import org.jetbrains.kotlin.config.CommonConfigurationKeys
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.psi.KtFile
import org.jetbrains.kotlin.psi.KtPsiFactory
import java.io.File

/**
 * Parses Kotlin source text into a [KtFile] suitable for handing to a
 * plugin rule's [`KritRule.check`] invocation. The `analyzeFile` RPC
 * accepts a `(path, source)` pair, so the daemon needs to materialize
 * PSI on demand rather than relying on a pre-parsed source cache
 * (which is what krit-types' Analysis API session provides).
 *
 * The IntelliJ [KotlinCoreEnvironment] is a process-wide singleton:
 * the first `createForProduction` call wires up an
 * [com.intellij.openapi.application.Application]; subsequent calls
 * fail because Application can only be set once per JVM. To stay
 * compatible we keep one environment alive for the daemon's lifetime
 * and reuse its `Project` for every parse — see
 * [sharedEnvironment]. Parses themselves are stateless because
 * [KtPsiFactory.createFile] does not mutate the `Project`.
 */
internal object KtFileParser {

    private val rootDisposable by lazy {
        Disposer.newDisposable("KtFileParser-root")
    }

    @OptIn(K1Deprecation::class)
    private val sharedEnvironment: KotlinCoreEnvironment by lazy {
        val config = CompilerConfiguration().apply {
            put(CommonConfigurationKeys.MESSAGE_COLLECTOR_KEY, MessageCollector.NONE)
            put(CommonConfigurationKeys.MODULE_NAME, "krit-fir-analyzeFile")
        }
        // `createForProduction` carries the @DeprecatedCompilerApi
        // marker in 2.3.x — that signals K2 migration is in flight,
        // not removal. We opt in deliberately and continue using the
        // K1 environment because the daemon only needs PSI parsing,
        // which the K2 successor doesn't yet expose as a stable
        // public surface.
        KotlinCoreEnvironment.createForProduction(
            rootDisposable,
            config,
            EnvironmentConfigFiles.JVM_CONFIG_FILES,
        )
    }

    /**
     * Parse [source] as Kotlin code. Uses the leaf of [pathHint] as
     * the synthesized file name when present so a rule that reads
     * `file.name` sees the request path.
     */
    fun parse(source: String, pathHint: String? = null): ParsedKtFile {
        val name = pathHint?.let { File(it).name }.takeUnless { it.isNullOrBlank() } ?: "Source.kt"
        val ktFile = KtPsiFactory(sharedEnvironment.project, markGenerated = false)
            .createFile(name, source)
        return ParsedKtFile(ktFile)
    }

    /**
     * Returned wrapper for parsed [KtFile]s. The environment itself
     * lives across requests, so `close` is a no-op today; callers
     * still use try-with-resources for forward-compat in case we move
     * to a per-request disposable later (e.g. when PSI references
     * accumulate enough to need pruning).
     */
    class ParsedKtFile(val ktFile: KtFile) : AutoCloseable {
        override fun close() {
            // Intentional no-op; see class doc.
        }

        /** Alias for [close] for callers that already think in disposables. */
        fun dispose() = close()
    }
}
