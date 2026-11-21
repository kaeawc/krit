package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.classIdOf
import org.jetbrains.kotlin.name.CallableId

internal fun isBuildAndCompilerCall(id: CallableId): Boolean =
    isGradleBuilderCall(id) || isCompilerCall(id) || isIncrementalCompilerCall(id) ||
        isRoboImageCall(id) || isPaparazziCall(id) || isReactiveCall(id)

private fun isGradleBuilderCall(id: CallableId): Boolean =
    id.classId == classIdOf("com.autonomousapps.kit", "GradleBuilder") && id.callableName.asString() == "build"

private fun isCompilerCall(id: CallableId): Boolean {
    val packageName = id.packageName.asString()
    val compilerPackage = inPackage(packageName, "org.jetbrains.kotlin.test") ||
        inPackage(packageName, "dev.zacsweers.metro.compiler")
    return compilerPackage && id.callableName.asString() == "compile" &&
        id.classId?.shortClassName?.asString()?.endsWith("CompilerTest") == true
}

private fun isIncrementalCompilerCall(id: CallableId): Boolean =
    inPackage(id.packageName.asString(), "org.jetbrains.kotlin.incremental") &&
        id.callableName.asString() == "compileKotlinAndFail"

private fun isRoboImageCall(id: CallableId): Boolean {
    val packageName = id.packageName.asString()
    val roboPackage = inPackage(packageName, "com.github.takahirom.roborazzi") ||
        inPackage(packageName, "io.github.takahirom.roborazzi")
    return roboPackage && id.callableName.asString().endsWith("RoboImage")
}

private fun isPaparazziCall(id: CallableId): Boolean =
    inPackage(id.packageName.asString(), "app.cash.paparazzi") && id.callableName.asString() in paparazziCallNames

internal fun isReactiveCall(id: CallableId): Boolean {
    val packageName = id.packageName.asString()
    val reactivePackage = inPackage(packageName, "io.reactivex") || inPackage(packageName, "rx")
    val reactiveName = id.callableName.asString().startsWith("assert") ||
        id.callableName.asString().startsWith("await")
    return reactivePackage && reactiveName
}

private val paparazziCallNames = setOf("snapshot", "gif")
