package com.example

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.remember
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.flow.MutableStateFlow

class HomeViewModel : ViewModel()

class Helper(val vm: HomeViewModel)

@Composable
fun Mutable(
    items: MutableList<String>,
    map: MutableMap<String, Int>?,
    state: MutableState<Int>,
    flow: MutableStateFlow<Int>,
    list: ArrayList<String>,
    plain: List<String>,
    set: Set<Int>,
    m: Map<String, Int>,
    c: Collection<Int>,
    ok: String,
) {
    Text(ok)
}

@Composable
fun MutableState<Int>.ReceiverMutable() {
    Text("$value")
}

@Composable
fun Forwarding(viewModel: HomeViewModel) {
    HomeContent(viewModel)
    HomeContent(viewModel = viewModel)
    HomeContent((viewModel))
    Text(viewModel.toString())
    LaunchedEffect(viewModel) {}
    val helper = Helper(viewModel)
    remember(viewModel) { 1 }
    other(viewModel)
    Text(helper.toString())
}

@Composable
fun HomeContent(viewModel: HomeViewModel) {
    Text(viewModel.toString())
}

fun other(vm: HomeViewModel) = vm

@Composable
fun Injecting() {
    val vm = viewModel<HomeViewModel>()
    val vm2: HomeViewModel = viewModel()
    val lazy by lazy { 1 }
    Text("$vm $vm2 $lazy")
}

@Composable
fun Injected(vm: HomeViewModel = viewModel()) {
    Text("$vm")
}

/** Doc. */
@Composable
fun DocInjecting() {
    // comment
    val vm = viewModel<HomeViewModel>()
    Text("$vm")
}
