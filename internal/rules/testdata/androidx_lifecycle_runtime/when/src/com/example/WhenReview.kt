package com.example

import android.widget.TextView
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.whenResumed as wr
import androidx.lifecycle.whenStarted
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

class WhenReview(val lifecycle: Lifecycle, val label: TextView) {
    suspend fun listFirst(items: List<String>) {
        lifecycle.whenStarted {
            try {
                val x = items.first()
            } finally {
                label.setText("x")
            }
        }
    }

    suspend fun flowFirst(flow: Flow<String>) {
        lifecycle.whenStarted {
            try {
                val x = flow.first()
            } finally {
                label.setText("flow")
            }
        }
    }

    suspend fun suspendInCatch() {
        lifecycle.whenStarted {
            try {
                println()
            } catch (e: Exception) {
                delay(1)
            } finally {
                label.setText("catch")
            }
        }
    }

    suspend fun nestedLaunch() {
        lifecycle.whenStarted {
            coroutineScope {
                launch {
                    try {
                        delay(1)
                    } finally {
                        label.setText("nested")
                    }
                }
            }
        }
    }

    suspend fun aliased() {
        lifecycle.wr {
            try {
                delay(1)
            } finally {
                label.setText("alias")
            }
        }
    }
}

class WhenShapes(val lifecycle: Lifecycle, val label: TextView) {
    suspend fun bareFlowFirst(flow: Flow<String>) {
        lifecycle.whenStarted {
            try {
                flow.first()
            } finally {
                label.setText("bare")
            }
        }
    }

    suspend fun viewReadInLocal() {
        lifecycle.whenStarted {
            try {
                delay(1)
            } finally {
                val t = label.text
                label.setText("set")
            }
        }
    }
}
