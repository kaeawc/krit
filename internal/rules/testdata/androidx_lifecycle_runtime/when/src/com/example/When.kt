package com.example

import android.view.View
import android.widget.TextView
import androidx.fragment.app.Fragment
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.whenStarted
import kotlinx.coroutines.delay

class WhenFragment : Fragment() {
    lateinit var label: TextView

    suspend fun load() = delay(10)

    fun start(view: View) {
        lifecycleScope.launchWhenStarted { }
    }

    suspend fun direct() {
        lifecycle.whenStarted {
            try {
                load()
            } finally {
                label.text = "done"
            }
        }
        lifecycle.whenStarted {
            try {
                label.text = "a"
            } finally {
                label.text = "no suspend in try"
            }
        }
        lifecycle.whenStarted {
            try {
                delay(1)
            } finally {
                if (lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)) {
                    label.text = "guarded"
                }
            }
        }
        lifecycle.whenStarted {
            try {
                delay(1)
            } finally {
                println("no ui")
                updateUi()
            }
        }
    }

    fun updateUi() {
        label.visibility = View.GONE
    }
}
