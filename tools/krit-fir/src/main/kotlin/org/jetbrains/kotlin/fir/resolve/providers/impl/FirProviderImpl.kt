/*
 * Copyright 2010-2018 JetBrains s.r.o. and Kotlin Programming Language contributors.
 * Use of this source code is governed by the Apache 2.0 license that can be found in the license/LICENSE.txt file.
 *
 * Modified by Krit: this is the Kotlin 2.4.21 FirProviderImpl with one change. A per-package index of
 * callable names is kept as callables are recorded, and getTopLevelCallableNamesInPackage reads it. The
 * original scanned every function and property in the compilation for each package it was asked about,
 * which made a large compilation spend about a third of its time comparing names. Everything else is
 * unchanged, except that the compiler-test-only ensureConsistent check is left out. krit-fir loads this class instead of the compiler's own; see FirProviderImplPinTest.
 */


package org.jetbrains.kotlin.fir.resolve.providers.impl

import org.jetbrains.kotlin.fir.*
import org.jetbrains.kotlin.fir.declarations.*
import org.jetbrains.kotlin.fir.declarations.synthetic.FirSyntheticProperty
import org.jetbrains.kotlin.fir.resolve.providers.*
import org.jetbrains.kotlin.fir.scopes.FirKotlinScopeProvider
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.*
import org.jetbrains.kotlin.fir.visitors.FirDefaultVisitor
import org.jetbrains.kotlin.name.*
import org.jetbrains.kotlin.utils.mapToSetOrEmpty

@OptIn(SymbolInternals::class)
@ThreadSafeMutableState
class FirProviderImpl(val session: FirSession, val kotlinScopeProvider: FirKotlinScopeProvider) : FirProvider() {
    override val symbolProvider: FirSymbolProvider = SymbolProvider()

    override fun getFirCallableContainerFile(symbol: FirCallableSymbol<*>): FirFile? {
        symbol.originalIfFakeOverride()?.let { originalSymbol ->
            return originalSymbol.moduleData.session.firProvider.getFirCallableContainerFile(originalSymbol)
        }
        if (symbol is FirBackingFieldSymbol) {
            return getFirCallableContainerFile(symbol.fir.propertySymbol)
        }
        if (symbol is FirSyntheticPropertySymbol) {
            val fir = symbol.fir
            if (fir is FirSyntheticProperty) {
                return getFirCallableContainerFile(fir.getter.delegate.symbol)
            }
        }
        return state.callableContainerMap[symbol]
    }

    override fun getFirScriptContainerFile(symbol: FirScriptSymbol): FirFile? {
        return state.scriptContainerMap[symbol]
    }

    override fun getFirScriptByFilePath(path: String): FirScriptSymbol? {
        return state.scriptByFilePathMap[toSystemIndependentScriptPath(path)]
    }

    override fun getFirReplSnippetContainerFile(symbol: FirReplSnippetSymbol): FirFile? {
        return state.snippetContainerMap[symbol]
    }

    override fun getFirClassifierContainerFile(fqName: ClassId): FirFile {
        return state.classifierContainerFileMap[fqName] ?: error("Couldn't find container for $fqName")
    }

    override fun getFirClassifierContainerFileIfAny(fqName: ClassId): FirFile? {
        return state.classifierContainerFileMap[fqName]
    }

    fun recordFile(file: FirFile) {
        recordFile(file, state)
    }

    private inner class SymbolProvider : FirSymbolProvider(session) {
        override fun getClassLikeSymbolByClassId(classId: ClassId): FirClassLikeSymbol<*>? {
            return getFirClassifierByFqName(classId)?.symbol
        }

        @FirSymbolProviderInternals
        override fun getTopLevelCallableSymbolsTo(destination: MutableList<FirCallableSymbol<*>>, packageFqName: FqName, name: Name) {
            destination += (state.functionMap[CallableId(packageFqName, name)] ?: emptyList())
            destination += (state.propertyMap[CallableId(packageFqName, name)] ?: emptyList())
        }

        @FirSymbolProviderInternals
        override fun getTopLevelFunctionSymbolsTo(destination: MutableList<FirNamedFunctionSymbol>, packageFqName: FqName, name: Name) {
            destination += (state.functionMap[CallableId(packageFqName, name)] ?: emptyList())
        }

        @FirSymbolProviderInternals
        override fun getTopLevelPropertySymbolsTo(destination: MutableList<FirPropertySymbol>, packageFqName: FqName, name: Name) {
            destination += (state.propertyMap[CallableId(packageFqName, name)] ?: emptyList())
        }

        override fun hasPackage(fqName: FqName): Boolean {
            return fqName in state.allSubPackages
        }

        override val symbolNamesProvider: FirSymbolNamesProvider = object : FirSymbolNamesProvider() {
            override fun getPackageNames(): Set<String> = state.allSubPackages.mapToSetOrEmpty(FqName::asString)

            override val hasSpecificClassifierPackageNamesComputation: Boolean get() = false
            override val hasSpecificCallablePackageNamesComputation: Boolean get() = false

            override fun getTopLevelClassifierNamesInPackage(packageFqName: FqName): Set<Name> =
                state.classifierInPackage[packageFqName].orEmpty()

            override fun getTopLevelCallableNamesInPackage(packageFqName: FqName): Set<Name> =
                state.callableNamesInPackage[packageFqName].orEmpty()
        }
    }

    private fun recordFile(file: FirFile, state: State) {
        val packageName = file.packageFqName
        state.fileMap.merge(packageName, listOf(file)) { a, b -> a + b }
        generateSequence(packageName) { it.parentOrNull() }.forEach(state.allSubPackages::add)
        file.acceptChildren(FirRecorder, FirRecorderData(state, file, session.nameConflictsTracker))
    }

    private class FirRecorderData(
        val state: State,
        val file: FirFile,
        val nameConflictsTracker: FirNameConflictsTracker?,
    )

    private object FirRecorder : FirDefaultVisitor<Unit, FirRecorderData>() {
        override fun visitElement(element: FirElement, data: FirRecorderData) {}

        override fun visitRegularClass(regularClass: FirRegularClass, data: FirRecorderData) {
            visitClassifier(regularClass, data)
            val classId = regularClass.symbol.classId

            if (!classId.isNestedClass && !regularClass.isLocal) {
                data.state.classesInPackage.getOrPut(classId.packageFqName, ::mutableSetOf).add(classId.shortClassName)
                data.state.classifierInPackage.getOrPut(classId.packageFqName, ::mutableSetOf).add(classId.shortClassName)
            }

            regularClass.acceptChildren(this, data)
        }

        override fun visitTypeAlias(typeAlias: FirTypeAlias, data: FirRecorderData) {
            visitClassifier(typeAlias, data)
            val classId = typeAlias.symbol.classId
            data.state.classifierInPackage.getOrPut(classId.packageFqName, ::mutableSetOf).add(classId.shortClassName)
        }

        private fun visitClassifier(classLike: FirClassLikeDeclaration, data: FirRecorderData) {
            val classId = classLike.symbol.classId

            if (classId !in data.state.classifierMap) {
                data.state.classifierMap[classId] = classLike
                data.state.classifierContainerFileMap[classId] = data.file
            } else {
                data.nameConflictsTracker?.registerClassifierRedeclaration(
                    classId, classLike.symbol, data.file,
                    data.state.classifierMap.getValue(classId).symbol,
                    data.state.classifierContainerFileMap.getValue(classId),
                )
            }
        }

        override fun visitPropertyAccessor(
            propertyAccessor: FirPropertyAccessor,
            data: FirRecorderData,
        ) {
            val symbol = propertyAccessor.symbol
            data.state.callableContainerMap[symbol] = data.file
        }

        private inline fun <reified D : FirCallableDeclaration, S : FirCallableSymbol<D>> registerCallable(
            symbol: S,
            data: FirRecorderData,
            map: MutableMap<CallableId, List<S>>,
        ) {
            // TODO: KT-78984: we shouldn't call this function for symbols with callableId == null
            val callableId = symbol.callableId
                ?: return // For scripts, we can come here with local variables like <local>/<destruct>
            map.merge(callableId, listOf(symbol)) { a, b -> a + b }
            data.state.callableContainerMap[symbol] = data.file
            if (map === data.state.functionMap || map === data.state.propertyMap) {
                data.state.callableNamesInPackage.getOrPut(callableId.packageName, ::mutableSetOf).add(callableId.callableName)
            }
        }

        override fun visitConstructor(constructor: FirConstructor, data: FirRecorderData) {
            val symbol = constructor.symbol
            registerCallable(symbol, data, data.state.constructorMap)
        }

        override fun visitNamedFunction(namedFunction: FirNamedFunction, data: FirRecorderData) {
            val symbol = namedFunction.symbol
            registerCallable(symbol, data, data.state.functionMap)
        }

        override fun visitProperty(property: FirProperty, data: FirRecorderData) {
            val symbol = property.symbol
            registerCallable(symbol, data, data.state.propertyMap)
            property.getter?.let { visitPropertyAccessor(it, data) }
            property.setter?.let { visitPropertyAccessor(it, data) }
        }

        override fun visitEnumEntry(enumEntry: FirEnumEntry, data: FirRecorderData) {
            val symbol = enumEntry.symbol
            data.state.callableContainerMap[symbol] = data.file
        }

        override fun visitScript(script: FirScript, data: FirRecorderData) {
            val symbol = script.symbol
            data.state.scriptContainerMap[symbol] = data.file
            data.file.sourceFile?.path?.let { data.state.scriptByFilePathMap[toSystemIndependentScriptPath(it)] = symbol }
            script.acceptChildren(this, data)
        }

        override fun visitReplSnippet(
            replSnippet: FirReplSnippet,
            data: FirRecorderData,
        ) {
            data.state.snippetContainerMap[replSnippet.symbol] = data.file
            visitRegularClass(replSnippet.snippetClass, data)
            super.visitReplSnippet(replSnippet, data)
        }
    }

    private val state = State()

    private class State {
        val fileMap: MutableMap<FqName, List<FirFile>> = hashMapOf()
        val allSubPackages = mutableSetOf<FqName>()
        val classifierMap = hashMapOf<ClassId, FirClassLikeDeclaration>()
        val classifierContainerFileMap = hashMapOf<ClassId, FirFile>()
        val classifierInPackage = hashMapOf<FqName, MutableSet<Name>>()
        val classesInPackage = hashMapOf<FqName, MutableSet<Name>>()
        val functionMap = mutableMapOf<CallableId, List<FirNamedFunctionSymbol>>()
        val propertyMap = mutableMapOf<CallableId, List<FirPropertySymbol>>()
        // Krit: function and property names by package, kept in step with functionMap and propertyMap.
        val callableNamesInPackage = hashMapOf<FqName, MutableSet<Name>>()
        val constructorMap = hashMapOf<CallableId, List<FirConstructorSymbol>>()
        val callableContainerMap = hashMapOf<FirCallableSymbol<*>, FirFile>()
        val scriptContainerMap = hashMapOf<FirScriptSymbol, FirFile>()
        val scriptByFilePathMap = hashMapOf<String, FirScriptSymbol>()
        val snippetContainerMap = hashMapOf<FirReplSnippetSymbol, FirFile>()

        fun setFrom(other: State) {
            fileMap.clear()
            allSubPackages.clear()
            classifierMap.clear()
            classifierContainerFileMap.clear()
            functionMap.clear()
            propertyMap.clear()
            callableNamesInPackage.clear()
            constructorMap.clear()
            callableContainerMap.clear()
            scriptContainerMap.clear()
            scriptByFilePathMap.clear()
            snippetContainerMap.clear()

            fileMap.putAll(other.fileMap)
            allSubPackages.addAll(other.allSubPackages)
            classifierMap.putAll(other.classifierMap)
            classifierContainerFileMap.putAll(other.classifierContainerFileMap)
            functionMap.putAll(other.functionMap)
            propertyMap.putAll(other.propertyMap)
            other.callableNamesInPackage.forEach { (pkg, names) -> callableNamesInPackage[pkg] = names.toMutableSet() }
            constructorMap.putAll(other.constructorMap)
            callableContainerMap.putAll(other.callableContainerMap)
            scriptContainerMap.putAll(other.scriptContainerMap)
            scriptByFilePathMap.putAll(other.scriptByFilePathMap)
            snippetContainerMap.putAll(other.snippetContainerMap)
            classesInPackage.putAll(other.classesInPackage)
            classifierInPackage.putAll(other.classifierInPackage)
        }
    }

    override fun getFirFilesByPackage(fqName: FqName): List<FirFile> {
        return state.fileMap[fqName].orEmpty()
    }

    override fun getFirClassifierByFqName(classId: ClassId): FirClassLikeDeclaration? {
        @OptIn(ClassIdBasedLocality::class)
        require(!classId.isLocal) {
            "Local $classId should never be used to find its corresponding classifier"
        }
        return state.classifierMap[classId]
    }

    override fun getClassNamesInPackage(fqName: FqName): Set<Name> {
        return state.classesInPackage[fqName] ?: emptySet()
    }
}

private const val rebuildIndex = true

private fun toSystemIndependentScriptPath(path: String): String = path.replace('\\', '/')
