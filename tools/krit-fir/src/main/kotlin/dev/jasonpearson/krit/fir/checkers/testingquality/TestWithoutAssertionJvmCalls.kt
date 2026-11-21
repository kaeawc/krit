package dev.jasonpearson.krit.fir.checkers.testingquality

import org.jetbrains.kotlin.name.CallableId

internal fun isJvmAssertion(id: CallableId): Boolean {
    val name = id.callableName.asString()
    val packageName = id.packageName.asString()
    return id.classId in TestWithoutAssertionCalls.junitOwners && isAssertOrFail(name) ||
        id.classId == null && packageName == "org.junit.jupiter.api" && isAssertOrFail(name) ||
        packageName == "kotlin.test" && (isAssertOrFail(name) || name == "expect")
}

internal fun isAssertionFrameworkCall(id: CallableId): Boolean {
    val name = id.callableName.asString()
    val packageName = id.packageName.asString()
    return inPackage(packageName, "org.hamcrest") && name == "assertThat" ||
        isTruthOrAssertJCall(packageName, name) || isKotestCall(packageName, name)
}

private fun isTruthOrAssertJCall(packageName: String, name: String): Boolean =
    (inPackage(packageName, "com.google.common.truth") || inPackage(packageName, "org.assertj")) &&
        (isAssertOrFail(name) || name.startsWith("is") || name.startsWith("contains") || name.startsWith("has"))

private fun isKotestCall(packageName: String, name: String): Boolean = inPackage(packageName, "io.kotest") &&
    (name.startsWith("should") || name.startsWith("assert") || name.startsWith("expect") || name == "fail")

private fun isAssertOrFail(name: String): Boolean = name.startsWith("assert") || name == "fail"
