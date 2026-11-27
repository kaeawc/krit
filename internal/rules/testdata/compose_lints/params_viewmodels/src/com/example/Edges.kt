package com.example

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun List<String>.ReceiverCollection() {
    Text(first())
}

object Screens {
    @Composable
    fun Detail(viewModel: HomeViewModel) {
        Text(viewModel.toString())
    }
}

fun Track(viewModel: HomeViewModel) = viewModel

@Composable
fun EdgeForwarding(viewModel: HomeViewModel, items: List<Int>) {
    Screens.Detail(viewModel)
    items.forEach { viewModel -> Text("$viewModel") }
    Track(viewModel)
    val custom = myFactory<HomeViewModel>()
    Text("$custom")
}

fun <T> myFactory(): T = TODO()
