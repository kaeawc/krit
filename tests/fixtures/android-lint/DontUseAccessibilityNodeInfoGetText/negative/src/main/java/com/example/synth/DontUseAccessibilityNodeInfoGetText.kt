package com.example.synth

import androidx.test.uiautomator.textAsString
import androidx.test.uiautomator.uiAutomator

fun login() = uiAutomator {
    onElement { textAsString() == "Login" }.click()
    // A named-argument label is not a read of AccessibilityNodeInfo.text.
    onElement { matches(text = "Hello") }.click()
}

fun matches(text: String): Boolean = text.isNotEmpty()
