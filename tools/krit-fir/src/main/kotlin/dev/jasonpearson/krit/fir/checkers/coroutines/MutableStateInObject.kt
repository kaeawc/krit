package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.assignedAccess
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.inheritsFromAny
import dev.jasonpearson.krit.fir.support.nestedClassIdOf
import dev.jasonpearson.krit.fir.support.walkCheckedFileWithParents
import dev.jasonpearson.krit.fir.support.walkWithParents
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirPropertyAccessor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isCompanion
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.FirBackingFieldReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

internal object MutableStateInObject : FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "MutableStateInObject"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(MutableStateInObject)
    }

    private val synchronizedAnnotation = setOf(classIdOf("kotlin.jvm", "Synchronized"))
    private val synchronizedCalls = setOf(
        CallableId(FqName("kotlin"), Name.identifier("synchronized")),
        CallableId(FqName("kotlinx.atomicfu.locks"), Name.identifier("synchronized")),
        CallableId(FqName("kotlinx.atomicfu.locks"), Name.identifier("withLock")),
        CallableId(FqName("kotlin.concurrent"), Name.identifier("withLock")),
    )
    private val lifecycle = buildSet {
        for (name in listOf("After", "Before", "AfterClass", "BeforeClass")) {
            add(classIdOf("org.junit", name))
        }
        for (name in listOf("AfterTest", "BeforeTest")) {
            add(classIdOf("kotlin.test", name))
        }
        for (name in listOf("AfterEach", "BeforeEach", "AfterAll", "BeforeAll")) {
            add(classIdOf("org.junit.jupiter.api", name))
        }
        for (name in listOf("AfterMethod", "AfterClass", "BeforeMethod", "BeforeClass")) {
            add(classIdOf("org.testng.annotations", name))
        }
    }

    @OptIn(DirectDeclarationsAccess::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        if (!isRelevantObject(declaration)) return
        val candidates = declaration.declarations.filterIsInstance<FirProperty>()
            .filter { MutableStateInObjectCandidates.isCandidate(it) }
        if (candidates.isEmpty()) return
        val sites = collectSites(declaration, candidates)
        for (candidate in candidates) {
            if (shouldReport(candidate, declaration, sites)) reportCandidate(candidate)
        }
    }

    context(context: CheckerContext)
    private fun isRelevantObject(declaration: FirRegularClass): Boolean {
        val isObject = declaration.classKind == ClassKind.OBJECT && !declaration.isCompanion
        if (!isObject || isInTestFile()) return false
        val path = containingScanPath()?.replace('\\', '/')?.lowercase().orEmpty()
        if (listOf("/sample/", "/samples/", "/playground/").any { it in path }) return false
        return (context.containingElements.filterIsInstance<FirRegularClass>() + declaration)
            .none { isTestDouble(it.name.asString()) }
    }

    private data class Sites(
        val references: Map<FirPropertySymbol, ArrayList<Site>>,
        val writes: Map<FirPropertySymbol, ArrayList<Site>>,
        val calls: Map<FirCallableSymbol<*>, MutableList<Site>>,
    )

    context(context: CheckerContext)
    private fun collectSites(declaration: FirRegularClass, candidates: List<FirProperty>): Sites {
        val references = candidates.associate { it.symbol to ArrayList<Site>() }
        val writes = candidates.associate { it.symbol to ArrayList<Site>() }
        val calls = HashMap<FirCallableSymbol<*>, MutableList<Site>>()
        walkCheckedFileWithParents { element, parents ->
            if (element is FirQualifiedAccessExpression && declaration in parents) {
                val target = element.calleeReference.toResolvedCallableSymbol()
                    ?.unwrapFakeOverrides() as? FirPropertySymbol
                if (parents.none { it is FirProperty && it.symbol == target }) {
                    references[target]?.add(Site(parents.toList()))
                }
            }
            if (element is FirVariableAssignment) {
                val target = assignedAccess(element)?.calleeReference?.toResolvedCallableSymbol()
                    ?.unwrapFakeOverrides() as? FirPropertySymbol
                if (parents.none { it is FirProperty && it.symbol == target }) {
                    writes[target]?.add(Site(parents.toList()))
                }
            }
            if (element is FirFunctionCall && declaration in parents) {
                val target = element.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()
                if (target != null) calls.getOrPut(target) { ArrayList() }.add(Site(parents.toList()))
            }
        }
        return Sites(references, writes, calls)
    }

    context(context: CheckerContext)
    private fun shouldReport(candidate: FirProperty, declaration: FirRegularClass, sites: Sites): Boolean {
        val propertyWrites = sites.writes.getValue(candidate.symbol)
        val hasAccessibleWrite = propertyWrites.isNotEmpty() ||
            candidate.visibility != Visibilities.Private && candidate.setter?.visibility != Visibilities.Private
        val hasResetWrite = propertyWrites.any {
            it.inFinally() || it.function()?.hasResolvedAnnotation(lifecycle) == true
        }
        return hasAccessibleWrite && !hasResetWrite &&
            !isIgnoredPrivateProperty(candidate, declaration, propertyWrites, sites)
    }

    context(context: CheckerContext)
    private fun isIgnoredPrivateProperty(
        candidate: FirProperty,
        declaration: FirRegularClass,
        propertyWrites: List<Site>,
        sites: Sites,
    ): Boolean {
        if (candidate.visibility != Visibilities.Private) return false
        val ownWrites = propertyWrites.filter { declaration in it.parents }
        val hasOnlyTestHookWrites = ownWrites.isNotEmpty() && ownWrites.all { it.function()?.isTestHook() == true }
        return hasOnlyTestHookWrites || sites.references.getValue(candidate.symbol).let { propertyReferences ->
            propertyReferences.isNotEmpty() &&
                propertyReferences.all { it.isSynchronized(sites.calls, HashSet()) }
        }
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun reportCandidate(candidate: FirProperty) {
        val source = candidate.source ?: return
        val anchor = identifierAnchor(source)
        val name = anchor?.text?.toString() ?: candidate.name.asString()
        val message = "Mutable 'var $name' in object declaration. " +
            "Shared mutable state without synchronization is a race condition."
        report(
            anchor ?: source,
            message,
        )
    }

    context(context: CheckerContext)
    private fun FirDeclaration.hasResolvedAnnotation(ids: Set<ClassId>): Boolean =
        annotations.any { it.annotationTypeRef.coneType.expandedClassType()?.lookupTag?.classId in ids }

    private fun isTestDouble(name: String): Boolean {
        val prefixes = listOf("Fake", "Stub", "Test")
        val hasTestPrefix = prefixes.any { prefix ->
            name.startsWith(prefix) && name.length > prefix.length && name[prefix.length] in 'A'..'Z'
        }
        return hasTestPrefix || name.endsWith("Fake") || name.endsWith("Test")
    }

    context(context: CheckerContext)
    private fun FirNamedFunction.isTestHook(): Boolean {
        val isVisibleForTesting = annotations.any {
            it.annotationTypeRef.coneType.expandedClassType()?.lookupTag?.classId?.shortClassName?.asString() ==
                "VisibleForTesting"
        }
        if (isVisibleForTesting) return true
        val name = name.asString()
        return "Testing" in name || name.endsWith("ForTest") || name.endsWith("ForTests")
    }

    private data class Site(val parents: List<FirElement>) {
        fun function(): FirNamedFunction? = parents.filterIsInstance<FirNamedFunction>().lastOrNull()

        fun inFinally(): Boolean = parents.filterIsInstance<FirTryExpression>().any { tryExpression ->
            val finallyBlock = tryExpression.finallyBlock
            finallyBlock != null && parents.any { it === finallyBlock }
        }

        context(context: CheckerContext)
        fun isSynchronized(
            calls: Map<FirCallableSymbol<*>, List<Site>>,
            visiting: MutableSet<FirCallableSymbol<*>>,
        ): Boolean {
            val hasSynchronizedCall = parents.filterIsInstance<FirFunctionCall>().any {
                it.calleeReference.toResolvedCallableSymbol()?.callableId in synchronizedCalls
            }
            if (hasSynchronizedCall) return true
            val accessorIsSynchronized =
                parents.filterIsInstance<FirPropertyAccessor>().lastOrNull()
                    ?.hasResolvedAnnotation(synchronizedAnnotation) == true
            if (accessorIsSynchronized) return true
            val function = function() ?: return false
            if (function.hasResolvedAnnotation(synchronizedAnnotation)) return true
            if (function.visibility != Visibilities.Private || !visiting.add(function.symbol)) return false
            val sites = calls[function.symbol].orEmpty()
            val result = sites.isNotEmpty() && sites.all { it.isSynchronized(calls, visiting) }
            visiting.remove(function.symbol)
            return result
        }
    }
}

private object MutableStateInObjectCandidates {
    private val safeTypes = buildSet {
        val atomicTypes = listOf(
            "AtomicInteger", "AtomicLong", "AtomicBoolean", "AtomicReference", "AtomicIntegerArray",
            "AtomicLongArray", "AtomicReferenceArray", "AtomicMarkableReference", "AtomicStampedReference",
            "AtomicIntegerFieldUpdater", "AtomicLongFieldUpdater", "AtomicReferenceFieldUpdater", "LongAdder",
            "DoubleAdder", "LongAccumulator", "DoubleAccumulator",
        )
        for (name in atomicTypes) {
            add(classIdOf("java.util.concurrent.atomic", name))
        }
        val concurrentTypes = listOf(
            "ConcurrentHashMap", "ConcurrentLinkedQueue", "ConcurrentLinkedDeque", "CopyOnWriteArrayList",
            "CopyOnWriteArraySet", "ConcurrentSkipListMap", "ConcurrentSkipListSet", "BlockingQueue",
            "BlockingDeque",
        )
        for (name in concurrentTypes) {
            add(classIdOf("java.util.concurrent", name))
        }
        add(nestedClassIdOf("java.util.concurrent", "ConcurrentHashMap.KeySetView"))
        val kotlinAtomicTypes = listOf(
            "AtomicInt", "AtomicLong", "AtomicBoolean", "AtomicReference", "AtomicIntArray", "AtomicLongArray",
            "AtomicArray",
        )
        for (name in kotlinAtomicTypes) {
            add(classIdOf("kotlin.concurrent.atomics", name))
        }
    }
    private val volatile = setOf(classIdOf("kotlin.jvm", "Volatile"), classIdOf("kotlin.concurrent", "Volatile"))

    context(context: CheckerContext)
    fun isCandidate(declaration: FirProperty): Boolean =
        isRealMutableProperty(declaration) &&
            declaration.returnTypeRef.coneType !is ConeErrorType &&
            hasResolvedInitializers(declaration) &&
            !hasVolatileAnnotation(declaration) &&
            !hasSafeStateType(declaration) &&
            usesBackingFieldWhenRequired(declaration)

    private fun isRealMutableProperty(declaration: FirProperty): Boolean {
        val source = declaration.source
        return declaration.isVar && !declaration.isLocal && source != null &&
            source.kind is KtRealSourceElementKind && source.elementType == KtNodeTypes.PROPERTY
    }

    private fun hasResolvedInitializers(declaration: FirProperty): Boolean =
        listOfNotNull(declaration.initializer, declaration.delegate).none(::hasUnresolvedReference)

    private fun hasUnresolvedReference(initializer: FirElement): Boolean {
        var unresolved = false
        walkWithParents(initializer) { element, _ ->
            if (element is FirExpression && element.resolvedType is ConeErrorType) unresolved = true
            if (element is FirFunctionCall && element.calleeReference.toResolvedCallableSymbol() == null) {
                unresolved = true
            }
        }
        return unresolved
    }

    context(context: CheckerContext)
    private fun hasVolatileAnnotation(declaration: FirProperty): Boolean =
        declaration.hasResolvedAnnotation(volatile) || declaration.backingField?.hasResolvedAnnotation(volatile) == true

    context(context: CheckerContext)
    private fun hasSafeStateType(declaration: FirProperty): Boolean {
        val declaredTypeIsSafe = mentionsSafeType(declaration.returnTypeRef.coneType)
        if (declaredTypeIsSafe) return true
        return declaration.initializer?.resolvedType?.let { mentionsSafeType(it) } == true
    }

    private fun usesBackingFieldWhenRequired(declaration: FirProperty): Boolean {
        val hasInitializer = declaration.initializer != null || declaration.delegate != null
        if (hasInitializer) return true
        val hasDefaultGetter = declaration.symbol.getterSymbol?.isDefault != false
        return hasDefaultGetter || declarationUsesBackingField(declaration)
    }

    private fun declarationUsesBackingField(declaration: FirProperty): Boolean {
        var usesField = false
        walkWithParents(declaration) { element, _ ->
            if (element is FirQualifiedAccessExpression && element.calleeReference is FirBackingFieldReference) {
                usesField = true
            }
        }
        return usesField
    }

    context(context: CheckerContext)
    private fun mentionsSafeType(type: ConeKotlinType, depth: Int = 0): Boolean {
        if (depth > 16 || type is ConeErrorType) return false
        val expanded = type.expandedClassType() ?: return false
        val expandedTypeIsSafe = expanded.lookupTag.classId in safeTypes ||
            expanded.lookupTag.classId.packageFqName.asString() == "java.util.concurrent.atomic"
        if (expandedTypeIsSafe) return true
        val symbol = expanded.lookupTag.toRegularClassSymbol(context.session)
        if (symbol != null && inheritsFromAny(symbol, safeTypes, context.session, lookupInterfaces = true)) return true
        return expanded.typeArguments.any { argument ->
            argument.type?.let { nestedType -> mentionsSafeType(nestedType, depth + 1) } == true
        }
    }

    context(context: CheckerContext)
    private fun FirDeclaration.hasResolvedAnnotation(ids: Set<ClassId>): Boolean =
        annotations.any { it.annotationTypeRef.coneType.expandedClassType()?.lookupTag?.classId in ids }
}
