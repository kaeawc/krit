package com.example

import android.view.View
import android.view.accessibility.AccessibilityNodeInfo

fun focus(view: View) {
    view.performAccessibilityAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS, null)
}
