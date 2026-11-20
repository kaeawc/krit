package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.assignedAccess
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.firstLineAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.inheritsFromAny
import dev.jasonpearson.krit.fir.support.inheritsFrom
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirAnonymousInitializer
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.processAllDeclarations
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassIdSafe
import org.jetbrains.kotlin.fir.declarations.utils.isAbstract
import org.jetbrains.kotlin.fir.declarations.utils.isLateInit
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId

/**
 * Reports singleton state when resolved symbols establish unprotected mutable state.
 *
 * Deliberate Go differences: aliases resolve, non-scope lookalikes do not; project
 * scopes named Singleton/ApplicationScoped require a recognized Scope/NormalScope meta-annotation.
 * Collection vals require a known mutable implementation or Kotlin factory, not
 * merely a MutableCollection interface (which includes concurrent implementations).
 * Mutable interfaces and unknown project subclasses alone are insufficient evidence.
 * GuardedBy deliberately uses
 * any package, matching Go. Collection vals share the var protection checks; mutex,
 * atomicfu and read/write lock wrappers also protect accesses. Unwritten and
 * initialization-only vars are silent when private, privately set or lateinit,
 * matching Go's field-injection exemption. Outside writers of other public/internal/
 * protected vars are not visible here. Constructor parameters use Go's main/UI
 * name-only exemption; arbitrary qualifiers do not prove confinement.
 * Thread-safe own types exempt only vars that are not reassigned;
 * concurrent collection exemptions do not extend to other concurrent-package types.
 * Go's atomic/mutex/hot-flow text markers still skip reassigned references; FIR
 * reports those reference writes consistently with other thread-safe wrappers.
 *
 * Lock/thread context is conservatively retained in lambdas: resetting it for an
 * executor/launch/thread callback could introduce findings without proving that
 * the callback actually escapes or executes on a different thread.
 */
internal object SingletonOnMutableClass :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "SingletonOnMutableClass"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(SingletonOnMutableClass)
    }

    private val singletonScopes = setOf(
        classIdOf("javax.inject", "Singleton"),
        classIdOf("jakarta.inject", "Singleton"),
        classIdOf("com.google.inject", "Singleton"),
        classIdOf("javax.ejb", "Singleton"),
        classIdOf("jakarta.ejb", "Singleton"),
        classIdOf("org.koin.core.annotation", "Singleton"),
    )
    private val applicationScopes = setOf(
        classIdOf("javax.enterprise.context", "ApplicationScoped"),
        classIdOf("jakarta.enterprise.context", "ApplicationScoped"),
        classIdOf("javax.faces.bean", "ApplicationScoped"),
        classIdOf("jakarta.faces.bean", "ApplicationScoped"),
    )
    private val threadAnnotations = setOf("androidx.annotation", "android.support.annotation")
        .flatMap { pkg -> listOf("MainThread", "UiThread", "WorkerThread").map { classIdOf(pkg, it) } }
        .toSet()
    private val protectionAnnotations = setOf(
        classIdOf("kotlin.jvm", "Volatile"),
        classIdOf("kotlin.concurrent", "Volatile"),
        classIdOf("kotlin.jvm", "Synchronized"),
        classIdOf("androidx.annotation", "GuardedBy"),
        classIdOf("android.support.annotation", "GuardedBy"),
        classIdOf("javax.annotation.concurrent", "GuardedBy"),
        classIdOf("net.jcip.annotations", "GuardedBy"),
        classIdOf("com.google.errorprone.annotations.concurrent", "GuardedBy"),
    )
    private val synchronizedAnnotation = setOf(classIdOf("kotlin.jvm", "Synchronized"))
    private val scopeMetaAnnotations = setOf(
        classIdOf("javax.inject", "Scope"), classIdOf("jakarta.inject", "Scope"),
        classIdOf("com.google.inject", "ScopeAnnotation"),
        classIdOf("javax.enterprise.context", "NormalScope"), classIdOf("jakarta.enterprise.context", "NormalScope"),
        classIdOf("me.tatarka.inject.annotations", "Scope"), classIdOf("dev.zacsweers.metro", "Scope"),
    )
    private val confinementNames = setOf("mainThreadHandler", "mainHandler", "uiHandler",
        "mainDispatcher", "uiDispatcher", "mainThreadDispatcher")
    private val mutableImplementations = listOf("ArrayList", "HashMap", "HashSet", "LinkedHashMap", "LinkedHashSet")
        .flatMap { name -> listOf(classIdOf("java.util", name), classIdOf("kotlin.collections", name)) }.toSet()
    private val mutableFactories = setOf("mutableListOf", "mutableSetOf", "mutableMapOf", "arrayListOf",
        "hashMapOf", "hashSetOf", "linkedMapOf", "linkedSetOf")
    private val confinementTypes = setOf(
        classIdOf("android.os", "Handler"),
        classIdOf("kotlinx.coroutines", "MainCoroutineDispatcher"),
    )
    private val protectedTypes = setOf(
        classIdOf("kotlinx.coroutines.sync", "Mutex"),
        classIdOf("kotlinx.coroutines.flow", "MutableStateFlow"),
        classIdOf("kotlinx.coroutines.flow", "MutableSharedFlow"),
        classIdOf("kotlinx.atomicfu.locks", "SynchronizedObject"),
    )
    private val atomicLockTypes = listOf("Lock", "ReadWriteLock")
        .map { classIdOf("java.util.concurrent.locks", it) }.toSet() +
        listOf("AtomicBoolean", "AtomicInteger", "AtomicLong", "AtomicReference", "AtomicIntegerArray",
        "AtomicLongArray", "AtomicReferenceArray", "AtomicMarkableReference", "AtomicStampedReference")
        .map { classIdOf("java.util.concurrent.atomic", it) } +
        listOf("AtomicBoolean", "AtomicInt", "AtomicLong", "AtomicRef")
            .map { classIdOf("kotlinx.atomicfu", it) } +
        listOf("AtomicBoolean", "AtomicInt", "AtomicLong", "AtomicReference", "AtomicIntArray",
            "AtomicLongArray", "AtomicArray").map { classIdOf("kotlin.concurrent.atomics", it) }

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        if (declaration.source?.kind !is KtRealSourceElementKind ||
            declaration.classKind == ClassKind.INTERFACE || declaration.classKind == ClassKind.OBJECT
        ) return
        val scopes = declaration.annotations.mapNotNull { scopeName(it, context.session) }
        val scope = listOf("Singleton", "ApplicationScoped").firstOrNull { it in scopes } ?: return
        if (hasAnnotation(declaration.annotations, threadAnnotations, context.session)) return
        var primary: FirConstructor? = null
        val members = mutableListOf<FirProperty>()
        declaration.processAllDeclarations(context.session) { symbol ->
            when (val member = symbol.fir) {
                is FirConstructor -> if (member.isPrimary) primary = member
                is FirProperty -> if (member.source?.kind is KtRealSourceElementKind && !member.isAbstract) {
                    members += member
                }
                else -> Unit
            }
        }
        if (primary?.valueParameters?.any {
                matchesType(it.returnTypeRef.coneType, confinementTypes) ||
                    it.name.asString() in confinementNames
            } == true
        ) return
        members.sortBy { it.source?.startOffset }
        val scan = StateScan(context.session)
        declaration.accept(scan)
        val findings = members.mapNotNull { member ->
            val source = member.source ?: return@mapNotNull null
            val annotations = member.annotations + member.backingField?.annotations.orEmpty() +
                member.getter?.annotations.orEmpty() + member.setter?.annotations.orEmpty()
            if (hasAnnotation(annotations, protectionAnnotations, context.session) ||
                annotations.any { it.toAnnotationClassIdSafe(context.session)?.shortClassName?.asString() == "GuardedBy" } ||
                (member.symbol !in scan.assigned && (
                    matchesType(member.returnTypeRef.coneType, protectedTypes + atomicLockTypes) ||
                    isThreadSafeCollection(member.returnTypeRef.coneType, member.initializer as? FirFunctionCall) ||
                    member.initializer?.let { matchesType(it.resolvedType, protectedTypes) } == true
                )) ||
                member.delegate?.let { matchesType(it.resolvedType, protectedTypes + atomicLockTypes) } == true
            ) return@mapNotNull null
            val references = scan.references[member.symbol].orEmpty()
            if (references.isNotEmpty() && references.all { it.threadAnnotated }) return@mapNotNull null
            if ((!member.isVar || member.visibility == Visibilities.Private || member.setter?.visibility == Visibilities.Private) &&
                references.isNotEmpty() && references.all { scan.isSynchronized(it) }
            ) return@mapNotNull null
            if (member.visibility != Visibilities.Private && member.setter?.visibility == Visibilities.Private &&
                scan.writes[member.symbol].orEmpty().all { scan.isSynchronized(it) }
            ) return@mapNotNull null
            val reason = if (member.isVar) {
                if (member.symbol !in scan.assigned &&
                    (member.visibility == Visibilities.Private || member.setter?.visibility == Visibilities.Private || member.isLateInit)
                ) return@mapNotNull null
                "var property"
            } else {
                val initializer = member.initializer ?: return@mapNotNull null
                val call = initializer as? FirFunctionCall ?: return@mapNotNull null
                if (!isKnownMutableCollection(call)) return@mapNotNull null
                val callSource = call.source ?: return@mapNotNull null
                val callNode = selectorCallExpression(callSource) ?: return@mapNotNull null
                val callee = significantChildren(callSource, callNode).firstOrNull() ?: return@mapNotNull null
                "val initialised by ${lightText(callSource, callee)}()"
            }
            source to "'${identifierText(source) ?: member.name.asString()}' ($reason)"
        }
        if (findings.isEmpty()) return
        val name = declaration.source?.let(::identifierText) ?: declaration.name.asString()
        val source = findings.first().first
        report(
            firstLineAnchor(source, KtTokens.VAR_KEYWORD, KtTokens.VAL_KEYWORD) ?: source,
            "@$scope class '$name' holds unprotected mutable state in ${findings.joinToString(", ") { it.second }}; " +
                "singleton-scoped state should be thread-safe.",
        )
    }

    context(context: CheckerContext)
    private fun isKnownMutableCollection(call: FirFunctionCall): Boolean {
        val symbol = call.calleeReference.toResolvedCallableSymbol() ?: return false
        val callableId = symbol.callableId ?: return false
        return (symbol is FirConstructorSymbol && callableId.classId in mutableImplementations) ||
            (callableId.packageName.asString() == "kotlin.collections" && callableId.classId == null &&
                callableId.callableName.asString() in mutableFactories)
    }

    context(context: CheckerContext)
    private fun isThreadSafeCollection(type: ConeKotlinType, call: FirFunctionCall?): Boolean {
        val types = listOfNotNull(type.expandedClassType(), call?.resolvedType?.expandedClassType())
        if (types.any {
                isThreadSafeCollectionClass(it.lookupTag.classId) ||
                    it.lookupTag.toClassSymbol(context.session)?.let { symbol ->
                        inheritsFrom(symbol, context.session, lookupInterfaces = true, ::isThreadSafeCollectionClass)
                    } == true
            }
        ) return true
        val callableId = call?.calleeReference?.toResolvedCallableSymbol()?.callableId ?: return false
        return callableId.classId == classIdOf("java.util", "Collections") &&
            (callableId.callableName.asString().startsWith("synchronized") || callableId.callableName.asString().startsWith("unmodifiable"))
    }

    private fun isThreadSafeCollectionClass(classId: ClassId): Boolean =
        (classId.packageFqName.asString() == "java.util.concurrent" && classId.shortClassName.asString() in setOf(
            "ConcurrentHashMap", "CopyOnWriteArrayList", "CopyOnWriteArraySet", "ConcurrentLinkedQueue",
            "ConcurrentLinkedDeque", "LinkedBlockingQueue", "LinkedBlockingDeque", "ArrayBlockingQueue",
            "PriorityBlockingQueue", "LinkedTransferQueue", "SynchronousQueue", "ConcurrentSkipListMap",
            "ConcurrentSkipListSet", "DelayQueue",
        )) ||
            classId in setOf(classIdOf("java.util", "Vector"), classIdOf("java.util", "Hashtable")) ||
            classId in setOf(classIdOf("androidx.compose.runtime.snapshots", "SnapshotStateList"),
                classIdOf("androidx.compose.runtime.snapshots", "SnapshotStateMap"))

    private fun scopeName(annotation: FirAnnotation, session: FirSession): String? {
        val classId = annotation.toAnnotationClassIdSafe(session) ?: return null
        if (classId in singletonScopes) return "Singleton"
        if (classId in applicationScopes) return "ApplicationScoped"
        val name = classId.shortClassName.asString()
        return name.takeIf { it in setOf("Singleton", "ApplicationScoped") && hasMetaAnnotation(annotation, scopeMetaAnnotations, session) }
    }

    private fun hasMetaAnnotation(annotation: FirAnnotation, targets: Set<ClassId>, session: FirSession): Boolean {
        val type = annotation.annotationTypeRef.coneType.fullyExpandedType(session) as? ConeClassLikeType ?: return false
        val symbol = type.lookupTag.toClassSymbol(session) ?: return false
        return symbol.resolvedAnnotationsWithClassIds.any { it.toAnnotationClassIdSafe(session) in targets }
    }

    context(context: CheckerContext)
    private fun matchesType(type: ConeKotlinType, targets: Set<ClassId>): Boolean {
        val expanded = type.expandedClassType() ?: return false
        if (expanded.lookupTag.classId in targets) return true
        val symbol = expanded.lookupTag.toClassSymbol(context.session) ?: return false
        return inheritsFromAny(symbol, targets, context.session, lookupInterfaces = true)
    }

    private fun hasAnnotation(annotations: List<FirAnnotation>, targets: Set<ClassId>, session: FirSession): Boolean =
        annotations.any { it.toAnnotationClassIdSafe(session) in targets }

    private data class Reference(
        val function: FirCallableSymbol<*>?,
        val locked: Boolean,
        val threadAnnotated: Boolean,
    )

    private class StateScan(private val session: FirSession) : FirVisitorVoid() {
        val references = mutableMapOf<FirPropertySymbol, MutableList<Reference>>()
        val writes = mutableMapOf<FirCallableSymbol<*>, MutableList<Reference>>()
        val assigned = mutableSetOf<FirCallableSymbol<*>>()
        private val functions = mutableMapOf<FirCallableSymbol<*>, FirNamedFunction>()
        private val calls = mutableMapOf<FirCallableSymbol<*>, MutableList<Reference>>()
        private var enclosingFunction: FirNamedFunction? = null
        private var enclosingProperty: FirProperty? = null
        private var locked = false
        private var initializing = false

        override fun visitElement(element: FirElement) {
            val previousFunction = enclosingFunction
            val previousProperty = enclosingProperty
            val previousLock = locked
            val previousInitializing = initializing
            when (element) {
                is FirConstructor, is FirAnonymousInitializer -> initializing = true
                is FirNamedFunction -> {
                    enclosingFunction = element
                    functions[element.symbol] = element
                }
                is FirProperty -> enclosingProperty = element
                is FirVariableAssignment -> {
                    val target = assignedAccess(element)
                    val receiver = unwrapSmartCasts(target?.explicitReceiver)
                    if (receiver == null || receiver is FirThisReceiverExpression) {
                        if (!initializing) target?.calleeReference?.toResolvedCallableSymbol()?.let {
                            assigned += it
                            writes.getOrPut(it) { mutableListOf() } += reference()
                        }
                    }
                }
            }
            if (element is FirFunctionCall) {
                val symbol = element.calleeReference.toResolvedCallableSymbol()
                if (symbol != null) calls.getOrPut(symbol) { mutableListOf() } += reference()
                val callableId = symbol?.callableId
                if (callableId?.classId == null) {
                    val pkg = callableId?.packageName?.asString()
                    val name = callableId?.callableName?.asString()
                    if ((name == "synchronized" && pkg in setOf("kotlin", "kotlinx.atomicfu.locks")) ||
                        (name == "withLock" && pkg in setOf("kotlin.concurrent", "kotlinx.coroutines.sync", "kotlinx.atomicfu.locks")) ||
                        (name in setOf("read", "write") && pkg == "kotlin.concurrent")
                    ) locked = true
                }
            }
            if (element is FirQualifiedAccessExpression) {
                val symbol = element.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol
                if (symbol != null && symbol != enclosingProperty?.symbol) {
                    references.getOrPut(symbol) { mutableListOf() } += reference()
                }
            }
            element.acceptChildren(this)
            enclosingFunction = previousFunction
            enclosingProperty = previousProperty
            locked = previousLock
            initializing = previousInitializing
        }

        private fun reference() = Reference(
            enclosingFunction?.symbol,
            locked,
            enclosingFunction?.let { hasAnnotation(it.annotations, threadAnnotations, session) } == true,
        )

        fun isSynchronized(reference: Reference, visiting: Set<FirCallableSymbol<*>> = emptySet()): Boolean {
            if (reference.locked) return true
            val symbol = reference.function ?: return false
            val member = functions[symbol] ?: return false
            if (hasAnnotation(member.annotations, synchronizedAnnotation, session)) return true
            if (member.visibility != Visibilities.Private || symbol in visiting) return false
            val callers = calls[symbol].orEmpty()
            return callers.isNotEmpty() && callers.all { isSynchronized(it, visiting + symbol) }
        }
    }
}
