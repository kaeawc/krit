package dev.krit.fir.tests

import java.io.File

// Scans src/test/data/diagnostic/**/*.kt and generates a JUnit test class per category.
//
// Usage (via Gradle):
//   ./gradlew :compiler-tests:generateTests
//
// Output: build/generated/source/tests/dev/krit/fir/tests/GeneratedDiagnosticTests.kt
fun main(args: Array<String>) {
    require(args.size == 2) { "Usage: GenerateTestsKt <dataDir> <outputDir>" }
    val dataDir = File(args[0])
    val outputDir = File(args[1])

    require(dataDir.isDirectory) { "dataDir not found: ${dataDir.absolutePath}" }

    // Group .kt files by their immediate parent directory (= category)
    val byCategory = dataDir.walkTopDown()
        .filter { it.isFile && it.extension == "kt" }
        .groupBy { it.parentFile.name }
        .toSortedMap()

    outputDir.resolve("dev/krit/fir/tests").mkdirs()
    val out = outputDir.resolve("dev/krit/fir/tests/GeneratedDiagnosticTests.kt")

    out.writeText(buildString {
        appendLine("// AUTO-GENERATED — do not edit; run :compiler-tests:generateTests to regenerate")
        appendLine("package dev.krit.fir.tests")
        appendLine()

        for ((category, files) in byCategory) {
            val className = category.replaceFirstChar { it.uppercaseChar() } + "DiagnosticTests"
            appendLine("class $className : AbstractDiagnosticTest() {")
            for (file in files.sortedBy { it.name }) {
                val methodName = "test" + file.nameWithoutExtension
                    .split(Regex("(?=[A-Z])"))
                    .joinToString("") { it.replaceFirstChar { c -> c.uppercaseChar() } }
                val relPath = "${category}/${file.name}"
                appendLine("    @org.junit.jupiter.api.Test fun $methodName() = runDiagnosticTest(\"$relPath\")")
            }
            appendLine("}")
            appendLine()
        }
    })

    println("Generated ${out.absolutePath} (${byCategory.values.sumOf { it.size }} tests)")
}
