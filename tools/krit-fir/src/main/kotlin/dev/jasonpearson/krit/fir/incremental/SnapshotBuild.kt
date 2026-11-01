@file:OptIn(org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class, org.jetbrains.kotlin.config.CompilerConfiguration.Internals::class, org.jetbrains.kotlin.config.MessageCollectorAccess::class)
@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.isIsolatable
import org.jetbrains.kotlin.backend.common.phaser.then
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.cli.common.diagnosticsCollector
import org.jetbrains.kotlin.cli.common.fir.FirDiagnosticsCompilerResultsReporter
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.config.messageCollector
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSeverity
import org.jetbrains.kotlin.cli.common.messages.CompilerMessageSourceLocation
import org.jetbrains.kotlin.cli.common.messages.GroupingMessageCollector
import org.jetbrains.kotlin.cli.common.messages.MessageCollector
import org.jetbrains.kotlin.cli.pipeline.AbstractCliPipeline
import org.jetbrains.kotlin.cli.pipeline.ArgumentsPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.ConfigurationPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.PipelineContext
import org.jetbrains.kotlin.cli.pipeline.PipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmBackendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmConfigurationPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFir2IrPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelineArtifact
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFrontendPipelinePhase
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmWriteOutputsPhase
import org.jetbrains.kotlin.config.Services
import org.jetbrains.kotlin.diagnostics.Severity
import org.jetbrains.kotlin.diagnostics.impl.DiagnosticsCollectorImpl
import org.jetbrains.kotlin.fir.pipeline.AllModulesFrontendOutput
import org.jetbrains.kotlin.cli.pipeline.jvm.JvmFir2IrPipelineArtifact
import org.jetbrains.kotlin.ir.declarations.IrAnonymousInitializer
import org.jetbrains.kotlin.ir.IrElement
import org.jetbrains.kotlin.ir.declarations.IrClass
import org.jetbrains.kotlin.ir.IrStatement
import org.jetbrains.kotlin.ir.expressions.IrExpression
import org.jetbrains.kotlin.ir.expressions.impl.IrBlockImpl
import org.jetbrains.kotlin.ir.visitors.IrElementTransformerVoid
import org.jetbrains.kotlin.ir.visitors.IrVisitorVoid
import org.jetbrains.kotlin.ir.visitors.transformChildrenVoid
import org.jetbrains.kotlin.ir.visitors.acceptChildrenVoid
import org.jetbrains.kotlin.ir.declarations.IrConstructor
import org.jetbrains.kotlin.ir.declarations.IrParameterKind
import org.jetbrains.kotlin.ir.declarations.IrDeclaration
import org.jetbrains.kotlin.ir.declarations.IrEnumEntry
import org.jetbrains.kotlin.ir.declarations.IrField
import org.jetbrains.kotlin.ir.declarations.IrFunction
import org.jetbrains.kotlin.ir.declarations.IrProperty
import org.jetbrains.kotlin.ir.expressions.IrBlock
import org.jetbrains.kotlin.ir.expressions.IrBlockBody
import org.jetbrains.kotlin.ir.expressions.IrStatementOrigin
import org.jetbrains.kotlin.ir.expressions.IrDelegatingConstructorCall
import org.jetbrains.kotlin.ir.expressions.IrEnumConstructorCall
import org.jetbrains.kotlin.ir.expressions.IrInstanceInitializerCall
import org.jetbrains.kotlin.ir.expressions.IrMemberAccessExpression
import org.jetbrains.kotlin.ir.expressions.impl.IrConstImpl
import org.jetbrains.kotlin.ir.expressions.impl.IrThrowImpl
import org.jetbrains.kotlin.config.phaser.CompilerPhase
import org.jetbrains.kotlin.platform.jvm.JvmPlatforms
import org.jetbrains.kotlin.util.PerformanceManager
import org.jetbrains.kotlin.util.PerformanceManagerImpl
import java.util.concurrent.ConcurrentLinkedQueue

/**
 * The stock JVM frontend without its error check: the compile goes on to [FrontendDonePhase] even when the frontend
 * reported compiler errors, which decides whether the backend runs.
 */
private object ToleratingFrontendPhase :
    PipelinePhase<ConfigurationPipelineArtifact, JvmFrontendPipelineArtifact>(name = "KritFrontend") {
    override fun executePhase(input: ConfigurationPipelineArtifact): JvmFrontendPipelineArtifact? =
        JvmFrontendPipelinePhase.executePhase(input)
}

/**
 * Passes the frontend's output on to the backend, or ends the pipeline: [onDone] gets the compilation's
 * configuration and returns the files to leave out of the backend (canonical paths), or null when the backend must
 * not run. Files left out lose their FIR for the backend, and the backend gets a fresh diagnostics collector and
 * the message collector [backendMessages] wraps around the compile's, so the frontend's errors in those files
 * neither reach its output nor stop it.
 */
private class FrontendDonePhase(
    private val onDone: (CompilerConfiguration) -> Set<String>?,
    private val backendMessages: (MessageCollector) -> MessageCollector,
) :
    PipelinePhase<JvmFrontendPipelineArtifact, JvmFrontendPipelineArtifact>(name = "KritFrontendDone") {
    override fun executePhase(input: JvmFrontendPipelineArtifact): JvmFrontendPipelineArtifact? {
        val leftOut = onDone(input.configuration) ?: return null
        if (leftOut.isEmpty()) return input
        val configuration = input.configuration.copy()
        configuration.diagnosticsCollector = DiagnosticsCollectorImpl()
        configuration.messageCollector = backendMessages(input.configuration.messageCollector)
        val output = input.frontendOutput.outputs.map { module ->
            module.copy(fir = module.fir.filter { file -> file.sourceFile?.path?.let(::canonicalPath) !in leftOut })
        }
        return input.copy(frontendOutput = AllModulesFrontendOutput(output), configuration = configuration)
    }
}

/**
 * Replaces every function body, field initializer, default value and enum entry argument of the compile's IR
 * with `throw null` (or nothing) when [enabled] says so, which is when files were left out of the backend (#971): code that calls into a
 * left-out file (its top-level functions, its inline functions) cannot be generated without that file. A slice reads
 * the snapshot through the frontend only, which never looks at bytecode bodies: the Kotlin metadata, the signatures
 * and the constants (const initializers stay) are what it reads, and those come out of the backend unchanged. A
 * local class or anonymous object stays where it is declared, since its name can be part of a signature (an anonymous
 * object's type) and is given while its body is lowered: around it every call is stripped, and its members too.
 */
private class StripBodiesPhase(private val enabled: () -> Boolean) :
    PipelinePhase<JvmFir2IrPipelineArtifact, JvmFir2IrPipelineArtifact>(name = "KritStripBodies") {
    override fun executePhase(input: JvmFir2IrPipelineArtifact): JvmFir2IrPipelineArtifact {
        if (!enabled()) return input
        val builtIns = input.result.irBuiltIns
        val factory = builtIns.irFactory
        fun throwNull(start: Int, end: Int) =
            IrThrowImpl(start, end, builtIns.nothingType, IrConstImpl.constNull(start, end, builtIns.nothingNType))
        fun declaresLocalClass(element: IrElement): Boolean {
            var found = false
            element.acceptChildrenVoid(object : IrVisitorVoid() {
                override fun visitElement(element: IrElement) { if (!found) element.acceptChildrenVoid(this) }
                override fun visitClass(declaration: IrClass) { found = true }
            })
            return found
        }
        // An argument that holds a local class keeps it, through [aroundClasses]; the others become `throw null`.
        fun stripArguments(call: IrMemberAccessExpression<*>, aroundClasses: ((IrExpression) -> IrExpression)?) {
            // The dispatch receiver of an inner class's call stays: it is the outer instance, not user code.
            for (i in call.arguments.indices) {
                val parameter = call.symbol.owner.let { (it as? IrFunction)?.parameters?.getOrNull(i) }
                if (parameter?.kind == IrParameterKind.DispatchReceiver) continue
                val argument = call.arguments[i] ?: continue
                call.arguments[i] = if (aroundClasses != null && declaresLocalClass(argument)) aroundClasses(argument)
                else throwNull(argument.startOffset, argument.endOffset)
            }
        }
        lateinit var stripClass: (IrClass) -> Unit
        // Strips the code around the local classes of [element]: each class is stripped like any class, and every
        // call is replaced by `throw null`, after the arguments that hold a local class, which stay.
        fun stripAroundLocalClasses(element: IrElement) {
            if (element is IrClass) { stripClass(element); return }
            val transformer = object : IrElementTransformerVoid() {
                override fun visitClass(declaration: IrClass): IrStatement { stripClass(declaration); return declaration }
                // An object expression is the object's class and the call of its constructor, which lowerings expect.
                override fun visitBlock(expression: IrBlock): IrExpression {
                    if (expression.origin != IrStatementOrigin.OBJECT_LITERAL) return super.visitBlock(expression)
                    for (statement in expression.statements) {
                        when (statement) {
                            is IrClass -> stripClass(statement)
                            is IrMemberAccessExpression<*> -> stripArguments(statement) { it.transform(this, null) as IrExpression }
                        }
                    }
                    return expression
                }
                override fun visitMemberAccess(expression: IrMemberAccessExpression<*>): IrExpression {
                    val start = expression.startOffset
                    val end = expression.endOffset
                    // A constructor's delegating call stays (lowerings rewrite it); its arguments are stripped.
                    if (expression is IrDelegatingConstructorCall || expression is IrEnumConstructorCall) {
                        stripArguments(expression) { it.transform(this, null) as IrExpression }
                        return expression
                    }
                    if (!declaresLocalClass(expression)) return throwNull(start, end)
                    val kept = expression.arguments.filterNotNull().filter(::declaresLocalClass).map { it.transform(this, null) as IrExpression }
                    return IrBlockImpl(start, end, builtIns.nothingType, null, kept + throwNull(start, end))
                }
            }
            // The element itself goes through the transformer too when it can (an object expression), else its children.
            if (element is IrExpression) element.transform(transformer, null) else element.transformChildrenVoid(transformer)
        }
        fun strip(declaration: IrDeclaration) {
            when (declaration) {
                is IrClass -> declaration.declarations.toList().forEach(::strip)
                is IrProperty -> {
                    declaration.getter?.let(::strip); declaration.setter?.let(::strip); declaration.backingField?.let(::strip)
                }
                is IrField -> if (declaration.correspondingPropertySymbol?.owner?.isConst != true) {
                    val initializer = declaration.initializer
                    if (initializer != null && declaresLocalClass(initializer)) stripAroundLocalClasses(initializer)
                    else declaration.initializer = null
                }
                is IrEnumEntry -> {
                    // The entry's constructor call stays (the enum lowering needs it); its arguments go.
                    declaration.initializerExpression?.let { init ->
                        if (declaresLocalClass(init)) stripAroundLocalClasses(init)
                        else (init.expression as? IrMemberAccessExpression<*>)?.let { stripArguments(it, null) }
                    }
                    declaration.correspondingClass?.let(::strip)
                }
                is IrAnonymousInitializer -> if (declaresLocalClass(declaration.body)) stripAroundLocalClasses(declaration.body) else declaration.body = factory.createBlockBody(declaration.startOffset, declaration.endOffset)
                is IrFunction -> {
                    for (parameter in declaration.parameters) parameter.defaultValue?.let { default ->
                        if (declaresLocalClass(default)) { stripAroundLocalClasses(default); return@let }
                        parameter.defaultValue = factory.createExpressionBody(default.startOffset, default.endOffset, throwNull(default.startOffset, default.endOffset))
                    }
                    val body = declaration.body ?: return
                    if (declaresLocalClass(body)) { stripAroundLocalClasses(body); return }
                    // A constructor keeps its delegating call (lowerings such as the inner-class one rewrite it),
                    // with its arguments stripped, and the instance initializer call; nothing else.
                    val kept = if (declaration is IrConstructor) (body as? IrBlockBody)?.statements.orEmpty().filter {
                        it is IrDelegatingConstructorCall || it is IrInstanceInitializerCall
                    } else emptyList()
                    for (call in kept) if (call is IrDelegatingConstructorCall) stripArguments(call, null)
                    declaration.body = factory.createBlockBody(body.startOffset, body.endOffset).apply {
                        statements += kept.ifEmpty { listOf(throwNull(body.startOffset, body.endOffset)) }
                    }
                }
                else -> {}
            }
        }
        stripClass = { strip(it) }
        for (file in input.result.irModuleFragment.files) file.declarations.toList().forEach(::strip)
        return input
    }
}

/**
 * The stock K2 JVM phases (configuration, frontend, fir2ir, backend, output), with a marker after the frontend.
 * The frontend is the same phase [dev.jasonpearson.krit.fir.runner.FrontendOnlyJvmPipeline] runs, so everything up
 * to the marker reports exactly what a frontend-only check reports.
 *
 * Two phases of the compiler's own JVM chain are left out. `FrontendFilesForPluginsGenerationPipelinePhase` lets
 * compiler plugins generate source files for the frontend; krit's plugin generates none, and the frontend-only
 * pipeline that is today's check does not run it either. `JvmSerializeCommonMetadataPipelinePhase` writes common
 * metadata for multiplatform compilations, which never get a snapshot (`expect`/`actual` falls back).
 */
private class SnapshotJvmPipeline(
    private val backendMessages: (MessageCollector) -> MessageCollector,
    private val stripBodies: () -> Boolean,
    private val frontendDone: (CompilerConfiguration) -> Set<String>?,
) : AbstractCliPipeline<K2JVMCompilerArguments>() {
    override val defaultPerformanceManager: PerformanceManager =
        PerformanceManagerImpl(JvmPlatforms.defaultJvmPlatform, "Kotlin to JVM Compiler (snapshot)")

    override fun createCompoundPhase(
        arguments: K2JVMCompilerArguments,
    ): CompilerPhase<PipelineContext, ArgumentsPipelineArtifact<K2JVMCompilerArguments>, *> =
        JvmConfigurationPipelinePhase then ToleratingFrontendPhase then FrontendDonePhase(frontendDone, backendMessages) then
            JvmFir2IrPipelinePhase then StripBodiesPhase(stripBodies) then JvmBackendPipelinePhase then JvmWriteOutputsPhase
}

/**
 * One whole-module check compile that also writes the module's binary snapshot: the stock phases with the backend,
 * `-Xreport-output-files`, into [snapshot]'s directory.
 *
 * It is a check first. The messages and the lookup recording see the frontend exactly as in a frontend-only compile
 * and nothing after it: once the frontend is done, [run] hands over every message the frontend produced, then drops
 * every later one (a backend diagnostic must not turn a clean verdict into an error) and freezes the lookups. The
 * compile's verdicts are therefore the verdicts of a frontend-only check, whatever the backend does. A backend
 * failure after the frontend only costs the snapshot.
 *
 * A module whose compiler errors all sit in files (#971) still gets a snapshot, of its other files: the files with
 * an error ([leftOut]) are left out of the backend, so they have no binary and every slice compiles them from source.
 * The backend only writes the class files a slice reads for their metadata; a clean file that refers to a left-out
 * file's declaration compiles to a reference by name, which is all a slice needs. No snapshot is built when more than
 * [maxLeftOut] files have an error (slices that carry that many files cost what a full check costs), or when the
 * compile has an error not tied to a file.
 *
 * The compiler keeps the frontend's diagnostics (every krit finding is a warning) in its diagnostics collector and
 * hands them to the message collector only when the pipeline ends, which here is after the backend. The marker
 * therefore reports and flushes them itself, before the backend runs; the end-of-pipeline repeat is dropped.
 */
internal class SnapshotBuild(
    val snapshot: Snapshot, private val lookups: CompileLookups?, private val maxLeftOut: Int = 0,
) {
    private val outputMessages = ConcurrentLinkedQueue<String>()

    /** The frontend finished without a compiler error outside [leftOut], so the backend ran (or failed trying). */
    @Volatile var frontendClean = false
        private set

    /** The files with a compiler error that the backend left out, canonical paths: they have no binary in [snapshot]. */
    @Volatile var leftOut: Set<String> = emptySet()
        private set

    /** Why the backend run failed after the frontend; null when it did not. */
    @Volatile var failure: Throwable? = null
        private set

    /** True when [snapshot] now holds the class files of the module's files outside [leftOut]. */
    @Volatile var built = false
        private set

    // Messages pass through until the frontend's are delivered; after that they are the backend's and are dropped.
    @Volatile private var dropping = false

    fun run(args: K2JVMCompilerArguments, services: Services, messages: MessageCollector): ExitCode {
        // The backend's first error, for the failure: it reaches either collector (a crash goes to the compile's).
        var backendError: String? = null
        fun noteBackend(severity: CompilerMessageSeverity, message: String) {
            if (severity.isError && backendError == null) backendError = message.lineSequence().take(3).joinToString(" ")
        }
        val gate = object : MessageCollector {
            override fun clear() = messages.clear()
            override fun hasErrors() = messages.hasErrors()
            override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) {
                if (severity == CompilerMessageSeverity.OUTPUT) { outputMessages += message; return }
                if (!dropping) messages.report(severity, message, location) else noteBackend(severity, message)
            }
        }
        var frontendErrors = false
        // The backend's own errors, once files were left out: the compile's collector already holds the frontend's.
        var backendErrors = false
        val backendMessages = { compile: MessageCollector ->
            object : MessageCollector {
                override fun clear() = compile.clear()
                override fun hasErrors() = backendErrors
                override fun report(severity: CompilerMessageSeverity, message: String, location: CompilerMessageSourceLocation?) {
                    if (severity.isError) backendErrors = true
                    noteBackend(severity, message)
                    compile.report(severity, message, location)
                }
            }
        }
        val pipeline = SnapshotJvmPipeline(backendMessages, { leftOut.isNotEmpty() }) pipeline@{ configuration ->
            // The frontend's errors: in files, which the backend can leave out, or not tied to one, which it cannot.
            val diagnostics = configuration.diagnosticsCollector
            val byFile = diagnostics.diagnosticsByFile.entries.filter { (_, list) -> list.any { it.severity == Severity.ERROR } }
            val errored = byFile.mapTo(LinkedHashSet()) { (file, _) -> file?.path?.let(::canonicalPath) }
            frontendErrors = diagnostics.hasErrors || configuration.messageCollector.hasErrors()
            if (frontendErrors) {
                val inFiles = byFile.sumOf { (_, list) -> list.count { it.severity == Severity.ERROR } }
                val untied = null in errored || configuration.messageCollector.hasErrors() ||
                    diagnostics.diagnostics.count { it.severity == Severity.ERROR } > inFiles
                if (untied || errored.isEmpty() || errored.size > maxLeftOut) return@pipeline null
            }
            // Without the buffer to flush the frontend's messages cannot be told from the backend's: fail closed,
            // no backend and no snapshot, instead of letting backend messages reach the verdict.
            val buffer = (configuration.messageCollector as? GroupingMessageCollector)
                ?.takeUnless { IncrementalTestHook.hideMessageBuffer }
            if (buffer == null) {
                failure = IllegalStateException("the compiler's message buffer is not available; no snapshot")
                return@pipeline null
            }
            frontendClean = true
            leftOut = if (frontendErrors) errored.filterNotNullTo(LinkedHashSet()) else emptySet()
            FirDiagnosticsCompilerResultsReporter.reportToMessageCollector(diagnostics, configuration)
            buffer.flush()
            dropping = true
            lookups?.freeze()
            IncrementalTestHook.onStage?.invoke(IncrementalStage.SNAPSHOT_BACKEND)
            leftOut
        }
        val exit = try {
            pipeline.execute(args, services, gate)
        } catch (t: Throwable) {
            // Before the marker this is today's failure and propagates as before.
            if (!frontendClean || !isIsolatable(t)) throw t
            failure = t
            ExitCode.OK
        }
        if (!frontendClean) {
            // Ended at the marker without a backend: the frontend's messages went out normally at the end, and the
            // compile's exit code is the frontend's.
            if (frontendErrors && failure == null) return ExitCode.COMPILATION_ERROR
            return if (failure != null) ExitCode.OK else exit
        }
        // With files left out the compile ends in COMPILATION_ERROR for the frontend's errors; only the backend's count.
        val backendFailed = if (leftOut.isEmpty()) exit != ExitCode.OK else exit == ExitCode.INTERNAL_ERROR || backendErrors
        if (failure == null && backendFailed) {
            failure = IllegalStateException("snapshot backend exited with $exit${backendError?.let { ": $it" }.orEmpty()}")
        }
        if (failure == null) {
            try {
                snapshot.adopt(Snapshot.parseOutputs(outputMessages))
                built = true
            } catch (t: Throwable) {
                if (!isIsolatable(t)) throw t
                failure = t
            }
        }
        // As far as the check goes the compile ended with the frontend: an error there is the compile's.
        return if (leftOut.isNotEmpty()) ExitCode.COMPILATION_ERROR else ExitCode.OK
    }
}
