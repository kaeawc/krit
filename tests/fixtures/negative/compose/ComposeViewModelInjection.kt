package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.lifecycle.viewmodel.compose.viewModel

@Composable
fun Home(vm: HomeScreenViewModel = viewModel()) {
    Text("$vm")
}

fun notComposable() {
    val vm = viewModel<HomeScreenViewModel>()
    println(vm)
}

@Composable
fun Delegated() {
    val text = "val vm = viewModel()"
    Text(text)
}
