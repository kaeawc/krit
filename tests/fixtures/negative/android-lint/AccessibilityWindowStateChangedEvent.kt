package com.example

import android.content.Context
import android.view.View
import android.view.accessibility.AccessibilityEvent

class AnnouncingView(context: Context) : View(context) {
    fun click() {
        sendAccessibilityEvent(AccessibilityEvent.TYPE_VIEW_CLICKED)
    }

    override fun onInitializeAccessibilityEvent(event: AccessibilityEvent) {
        super.onInitializeAccessibilityEvent(event)
        event.eventType = AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
    }
}
