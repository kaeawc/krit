package fixtures.androidlint

import android.widget.TextView
import androidx.fragment.app.Fragment
import androidx.lifecycle.whenStarted
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

class StatusFragment : Fragment() {
    lateinit var status: TextView

    suspend fun refresh() {
        lifecycle.whenStarted {
            try {
                delay(100)
                status.text = "loaded"
            } finally {
                println("cleanup without touching views")
            }
        }
    }

    // List.first() is not a suspend call; a suspend call in a catch block or a launched coroutine does not count.
    suspend fun notSuspending(items: List<String>) {
        lifecycle.whenStarted {
            try {
                items.first()
            } catch (e: Exception) {
                delay(1)
            } finally {
                status.text = "done"
            }
            coroutineScope {
                launch {
                    try {
                        delay(1)
                    } finally {
                        status.text = "launched"
                    }
                }
            }
        }
    }
}
