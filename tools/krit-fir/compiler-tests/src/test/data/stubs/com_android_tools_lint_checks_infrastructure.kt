package com.android.tools.lint.checks.infrastructure

class TestLintTask {
    private var sourceFiles = emptyList<String>()
    fun files(vararg files: String): TestLintTask {
        sourceFiles = files.toList()
        return this
    }
    fun run(): TestLintResult = TestLintResult(sourceFiles)
}
class TestLintResult(val files: List<String>) {
    fun expectClean() = Unit
    fun expectErrorCount(count: Int) = Unit
}
fun lint(): TestLintTask = TODO()
