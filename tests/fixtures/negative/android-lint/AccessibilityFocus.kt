package com.example

import android.view.View
import android.view.accessibility.AccessibilityNodeInfo

fun click(view: View) {
    view.performAccessibilityAction(AccessibilityNodeInfo.ACTION_CLICK, null)
}
