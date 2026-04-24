package dev.krit.fir.runner

import dev.krit.fir.checkers.ComposeRememberWithoutKey
import dev.krit.fir.checkers.FlowCollectInOnCreate
import dev.krit.fir.checkers.UnsafeCastWhenNullable
import dev.krit.fir.rules.SmokeChecker
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar
import org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.extensions.FirAdditionalCheckersExtension
import org.jetbrains.kotlin.fir.extensions.FirExtensionRegistrar
import org.jetbrains.kotlin.fir.extensions.FirExtensionRegistrarAdapter

// CompilerPluginRegistrar for the krit-fir runner. Registered programmatically via Services
// (not via META-INF/services) so the runner can filter which checkers are active per request.
@OptIn(ExperimentalCompilerApi::class)
class CheckerRegistrar(private val enabledRules: Set<String>) : CompilerPluginRegistrar() {
    override val pluginId = "dev.krit.fir.runner"
    override val supportsK2 = true

    override fun ExtensionStorage.registerExtensions(configuration: CompilerConfiguration) {
        FirExtensionRegistrarAdapter.registerExtension(RunnerFirExtensionRegistrar(enabledRules))
    }
}

private class RunnerFirExtensionRegistrar(private val enabledRules: Set<String>) : FirExtensionRegistrar() {
    override fun ExtensionRegistrarContext.configurePlugin() {
        +{ session: FirSession -> RunnerFirCheckers(session, enabledRules) }
    }
}

private class RunnerFirCheckers(
    session: FirSession,
    enabledRules: Set<String>,
) : FirAdditionalCheckersExtension(session) {
    // Empty set means "all rules enabled".
    private val all = enabledRules.isEmpty()

    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = buildSet {
            if (all || "FlowCollectInOnCreate" in enabledRules) add(FlowCollectInOnCreate)
            if (all || "ComposeRememberWithoutKey" in enabledRules) add(ComposeRememberWithoutKey)
        }
        override val typeOperatorCallCheckers = buildSet {
            if (all || "UnsafeCastWhenNullable" in enabledRules) add(UnsafeCastWhenNullable)
        }
    }

    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = buildSet {
            if (all || "SmokeChecker" in enabledRules) add(SmokeChecker)
        }
    }
}
