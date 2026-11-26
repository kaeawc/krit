package com.example

import android.view.accessibility.AccessibilityNodeInfo
import androidx.test.uiautomator.uiAutomator

fun scenario() = uiAutomator {
    onView { text == "Login" }.click()
    onView { textAsString() == "Login" }.click()
    onViews { this.text == "x" }
    onViewOrNull { viewIdResourceName == "id" && text.toString() == "y" }
    onView { val text = "shadow"; text == "z" }
    onView { contentDescription == "c" }
    onElement { text == "Released API" }.click()
    onElement { textAsString() == "Released API" }.click()
}

fun explicit(node: AccessibilityNodeInfo) {
    println(node.text)
}
