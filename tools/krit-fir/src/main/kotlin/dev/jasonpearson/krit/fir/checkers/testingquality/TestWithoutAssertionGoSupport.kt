package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.GoTestAssertionNames
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.lightText
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirImport
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.classId

internal object TestWithoutAssertionGoSupport {
    private val testAnnotations = setOf(
        classIdOf("org.junit", "Test"),
        classIdOf("org.junit.jupiter.api", "Test"),
        classIdOf("kotlin.test", "Test"),
    )
    private val ignoredAnnotations = setOf(
        classIdOf("org.junit", "Ignore"),
        classIdOf("org.junit.jupiter.api", "Disabled"),
        classIdOf("kotlin.test", "Ignore"),
    )

    fun isTest(function: FirNamedFunction): Boolean =
        function.annotations.any { it.annotationTypeRef.coneType.classId in testAnnotations }

    fun isIgnored(declaration: FirDeclaration): Boolean = declaration.annotations.any { annotation ->
        annotation.annotationTypeRef.coneType.classId in ignoredAnnotations ||
            GoTestAssertionNames.identifiers(annotation.source).any { it == "Ignore" || it == "Disabled" }
    }

    fun sourceText(source: KtSourceElement?): String =
        source?.let { lightText(it, it.lighterASTNode) }.orEmpty()

    fun goUIVisibilityExempts(name: String, uiEvidence: Boolean): Boolean =
        uiEvidence && name.startsWith("is") && name.endsWith("Displayed")

    fun isBenchmarkImport(imported: FirImport): Boolean {
        val name = imported.importedFqName?.asString() ?: return false
        return name in GoTestAssertionNames.benchmarkImports ||
            imported.isAllUnder && GoTestAssertionNames.benchmarkImports.any {
                it.substringBeforeLast('.') == name
            } ||
            inPackage(name, "androidx.test.uiautomator") || inPackage(name, "androidx.benchmark.macro")
    }

    fun isBenchmarkPath(path: String): Boolean =
        "/benchmark/" in path || "/macrobenchmark/" in path

    fun inPackage(actual: String, expected: String): Boolean = actual == expected || actual.startsWith("$expected.")
}
