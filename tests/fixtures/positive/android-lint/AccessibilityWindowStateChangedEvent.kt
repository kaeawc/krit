package com.example

import android.content.Context
import android.view.View
import android.view.accessibility.AccessibilityEvent

class AnnouncingView(context: Context) : View(context) {
    fun announce() {
        sendAccessibilityEvent(AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED)
    }
}
