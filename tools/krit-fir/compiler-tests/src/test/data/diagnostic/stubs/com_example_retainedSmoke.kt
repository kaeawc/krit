package stubs

import androidx.compose.runtime.Composable
import com.example.retained.produceRetainedState

@Composable
fun RetainedSmoke() = produceRetainedState(initialValue = 0) { value = 1 }
