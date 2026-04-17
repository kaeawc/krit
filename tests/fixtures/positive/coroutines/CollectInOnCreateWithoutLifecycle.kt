package coroutines

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch

private class ExampleViewModel {
    val state = MutableStateFlow(0)
}

class CollectInOnCreateWithoutLifecycleActivity : AppCompatActivity() {
    private val vm = ExampleViewModel()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        lifecycleScope.launch {
            vm.state.collect { render(it) }
        }
    }

    private fun render(state: Int) {}
}
