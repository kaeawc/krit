package fixtures.androidlint

import android.widget.TextView
import androidx.fragment.app.Fragment
import androidx.lifecycle.whenStarted
import kotlinx.coroutines.delay

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
}
