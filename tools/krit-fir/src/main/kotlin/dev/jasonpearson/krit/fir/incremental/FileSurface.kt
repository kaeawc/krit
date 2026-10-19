@file:OptIn(org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess::class)

package dev.jasonpearson.krit.fir.incremental

import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirCallableDeclaration
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirDeclarationStatus
import org.jetbrains.kotlin.fir.declarations.FirEnumEntry
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirMemberDeclaration
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirTypeAlias
import org.jetbrains.kotlin.fir.declarations.FirTypeParameterRef
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.declarations.getSealedClassInheritors
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.render
import org.jetbrains.kotlin.fir.types.FirTypeRef
import org.jetbrains.kotlin.fir.types.coneTypeOrNull
import org.jetbrains.kotlin.fir.types.renderReadableWithFqNames
import dev.jasonpearson.krit.fir.isIsolatable
import java.security.MessageDigest

/**
 * The declaration surface of one file as resolved FIR sees it: every non-private declaration keyed by the lookup symbol
 * other files would use to reach it, with its resolved signature (inferred types included). Inline function bodies and
 * const initializers are part of the surface, as for IC. Constructors are folded into the class header.
 *
 * [decls] maps each lookup symbol to a hash of its declarations; a class is keyed `Sym(scope, "class:Name")`.
 * [supers] maps the fully qualified name of every non-private class declared in the file (nested ones included)
 * to its rendered supertypes, as written by `renderReadableWithFqNames` (type arguments and `?` included).
 */
internal data class FileSurface(
    val decls: Map<Sym, String>,
    val supers: Map<String, List<String>>,
    /** Fully qualified names of the sealed classes and interfaces declared in the file. */
    val sealed: Set<String> = emptySet(),
) {
    companion object {
        /** Changed lookup symbols between [old] and [new]; class-header changes also return the class scope. */
        fun diff(old: FileSurface?, new: FileSurface?): Pair<Set<Sym>, Set<String>> {
            val o = old?.decls.orEmpty(); val n = new?.decls.orEmpty()
            val syms = HashSet<Sym>()
            for (k in o.keys + n.keys) if (o[k] != n[k]) syms += k
            // a class whose header (supertypes, modality, constructors, sealed inheritors...) changed: every lookup in its scope
            val scopes = syms.filter { it.name.startsWith(CLASS_MARK) }.map { "${it.scope.let { s -> if (s.isEmpty()) "" else "$s." }}${it.name.removePrefix(CLASS_MARK)}" }.toSet()
            val plain = syms.map { if (it.name.startsWith(CLASS_MARK)) Sym(it.scope, it.name.removePrefix(CLASS_MARK)) else it }.toSet()
            return plain to scopes
        }
        const val CLASS_MARK = "class:"
    }
}

internal object Surface {
    fun file(f: FirFile, session: FirSession): FileSurface {
        val acc = HashMap<Sym, MutableList<String>>()
        val supers = HashMap<String, List<String>>()
        val sealed = HashSet<String>()
        val pkg = f.packageDirective.packageFqName.asString()
        for (d in f.declarations) visit(d, pkg, acc, supers, sealed, session)
        return FileSurface(acc.mapValues { (_, v) -> hash(v.sorted().joinToString("\n")) }, supers, sealed)
    }

    private fun visible(d: FirMemberDeclaration) = d.status.visibility != Visibilities.Private

    private fun visit(d: FirDeclaration, scope: String, acc: MutableMap<Sym, MutableList<String>>, supers: MutableMap<String, List<String>>, sealed: MutableSet<String>, session: FirSession) {
        when (d) {
            is FirRegularClass -> {
                if (!visible(d)) return
                val fq = d.symbol.classId.asSingleFqName().asString()
                supers[fq] = d.superTypeRefs.map { t(it) }
                if (d.status.modality == org.jetbrains.kotlin.descriptors.Modality.SEALED) sealed += fq
                val header = buildString {
                    append(d.classKind).append(' ').append(st(d.status)).append(tps(d.typeParameters)).append(" : ")
                    append(d.superTypeRefs.joinToString { t(it) }).append(anns(d.annotations))
                    if (d.status.modality == org.jetbrains.kotlin.descriptors.Modality.SEALED) append(" sealed=").append(d.getSealedClassInheritors(session).map { it.asString() }.sorted())
                    for (c in d.declarations.filterIsInstance<FirConstructor>()) if (visible(c)) append("\n  ctor ").append(fn(c))
                }
                acc.getOrPut(Sym(scope, FileSurface.CLASS_MARK + d.name.asString())) { ArrayList() }.add(header)
                for (m in d.declarations) if (m !is FirConstructor) visit(m, fq, acc, supers, sealed, session)
            }
            is FirNamedFunction -> if (visible(d)) acc.getOrPut(Sym(scope, d.name.asString())) { ArrayList() }.add(fn(d))
            is FirProperty -> if (visible(d)) acc.getOrPut(Sym(scope, d.name.asString())) { ArrayList() }.add(prop(d))
            is FirTypeAlias -> if (visible(d)) acc.getOrPut(Sym(scope, d.name.asString())) { ArrayList() }.add("typealias ${st(d.status)}${tps(d.typeParameters)} = ${t(d.expandedTypeRef)}${anns(d.annotations)}")
            is FirEnumEntry -> acc.getOrPut(Sym(scope, d.name.asString())) { ArrayList() }.add("entry${anns(d.annotations)}")
            else -> {}
        }
    }

    private fun t(r: FirTypeRef?): String = r?.coneTypeOrNull?.renderReadableWithFqNames() ?: "?"
    private fun tps(ps: List<FirTypeParameterRef>) = if (ps.isEmpty()) "" else ps.joinToString(prefix = "<", postfix = ">") { p ->
        p.symbol.name.asString() + ":" + p.symbol.resolvedBounds.joinToString("&") { it.coneType.renderReadableWithFqNames() } + (if (p.symbol.isReified) " reified" else "") + " " + p.symbol.variance
    }
    private fun anns(a: List<FirAnnotation>) = if (a.isEmpty()) "" else a.joinToString(prefix = " @[", postfix = "]") { it.render().trim() }
    private fun vp(p: FirValueParameter) = "${p.name}:${t(p.returnTypeRef)}${if (p.defaultValue != null) "=" else ""}${if (p.isVararg) "..." else ""}${anns(p.annotations)}"
    private fun st(s: FirDeclarationStatus) = buildString {
        append(s.visibility.name).append(' ').append(s.modality?.name ?: "")
        for ((flag, name) in listOf(
            s.isExpect to "expect", s.isActual to "actual", s.isOverride to "override", s.isOperator to "operator", s.isInfix to "infix",
            s.isInline to "inline", s.isValue to "value", s.isTailRec to "tailrec", s.isExternal to "external", s.isConst to "const",
            s.isLateInit to "lateinit", s.isInner to "inner", s.isCompanion to "companion", s.isData to "data", s.isSuspend to "suspend",
            s.isFun to "fun",
        )) if (flag) append(' ').append(name)
    }
    private fun callable(d: FirCallableDeclaration) =
        "${st(d.status)}${tps(d.typeParameters)} ctx(${d.contextParameters.joinToString { vp(it) }}) recv(${t(d.receiverParameter?.typeRef)})"

    private fun fn(d: FirCallableDeclaration): String = buildString {
        append("fun ").append(callable(d))
        val params = when (d) { is FirNamedFunction -> d.valueParameters; is FirConstructor -> d.valueParameters; else -> emptyList() }
        append('(').append(params.joinToString { vp(it) }).append("): ").append(t(d.returnTypeRef)).append(anns(d.annotations))
        // inline bodies are inlined into call sites: part of the ABI for IC and for checkers that see inlined code
        if (d.status.isInline && d is FirNamedFunction) append(" body=").append(hash(d.body?.render() ?: ""))
        // contracts live in the body but are serialized into metadata and drive callers' smart casts
        if (d is FirNamedFunction) (d.contractDescription as? org.jetbrains.kotlin.fir.contracts.FirResolvedContractDescription)?.let { append(" contract=").append(hash(it.render())) }
    }

    private fun prop(d: FirProperty): String = buildString {
        append(if (d.isVar) "var " else "val ").append(callable(d)).append(": ").append(t(d.returnTypeRef)).append(anns(d.annotations))
        append(" get=").append(d.getter?.status?.visibility?.name).append(d.getter?.status?.isInline)
        append(" set=").append(d.setter?.status?.visibility?.name).append(d.setter?.status?.isInline)
        if (d.delegate != null) append(" delegated")
        if (d.status.isConst) append(" const=").append(d.initializer?.render())
        if (d.getter?.status?.isInline == true) append(" getterBody=").append(hash(d.getter?.body?.render() ?: ""))
    }

    fun hash(s: String): String {
        val md = MessageDigest.getInstance("SHA-1").digest(s.toByteArray())
        return hex(md, 10)
    }
}

/**
 * Records the [Surface] of every compiled file into [recording]. Registered only while incremental recording is
 * on; it reports nothing, so it never changes a diagnostic or a finding.
 */
internal class SurfaceFileChecker(private val recording: CompileRecording) : FirFileChecker(MppCheckerKind.Common) {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        // Best-effort: a failure here must never reach the compiler, where it would fail the file's real checkers.
        try {
            val path = declaration.sourceFile?.path ?: return
            SurfaceTestHook.beforeFile?.invoke(path)
            recording.surfaces[canonicalPath(path)] = Surface.file(declaration, context.session)
        } catch (e: Throwable) {
            if (!isIsolatable(e)) throw e
            recording.fail(e)
        }
    }
}

/** Lets tests make the surface checker fail on chosen files; null in production. */
internal object SurfaceTestHook {
    @Volatile var beforeFile: ((String) -> Unit)? = null
}
