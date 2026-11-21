package dev.jasonpearson.krit.fir.checkers.testingquality

import org.jetbrains.kotlin.name.CallableId
import dev.jasonpearson.krit.fir.support.classIdOf

internal fun isAndroidCall(id: CallableId): Boolean =
    isComposeCall(id) || isEspressoCall(id) || isComposeTestUtilsCall(id) || isWaitForCall(id)

private fun isComposeCall(id: CallableId): Boolean =
    inPackage(id.packageName.asString(), "androidx.compose.ui.test") &&
        (id.callableName.asString().startsWith("assert") || id.callableName.asString() in composeCallNames)

private fun isEspressoCall(id: CallableId): Boolean =
    inPackage(id.packageName.asString(), "androidx.test.espresso") && id.callableName.asString() in espressoCallNames

private fun isComposeTestUtilsCall(id: CallableId): Boolean =
    inPackage(id.packageName.asString(), "androidx.compose.testutils") &&
        id.callableName.asString() in setOf("expectError", "testContent")

private fun isWaitForCall(id: CallableId): Boolean =
    id.classId == classIdOf("androidx.testutils", "PollingCheck") && id.callableName.asString() == "waitFor"

private val composeCallNames = setOf(
    "isDisplayed", "isNotDisplayed", "doesNotExist", "waitUntil", "waitUntilAtLeastOneExists",
    "waitUntilExactlyOneExists", "waitUntilDoesNotExist",
)
private val espressoCallNames = setOf("check", "matches", "isDisplayed", "intended")
