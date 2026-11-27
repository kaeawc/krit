package com.example.nested

import androidx.compose.runtime.Composable

@Composable
private fun HomeScreenReady(podcastUri: String) {
    Pane(
        supportingPane = {
            if (!podcastUri.isNullOrEmpty()) {
                val podcastDetailsViewModel =
                    hiltViewModel<PodcastDetailsViewModel, PodcastDetailsViewModel.Factory>(
                        key = podcastUri,
                    ) {
                        it.create(podcastUri)
                    }
                Text(podcastDetailsViewModel)
            }
        },
    )
}

@Composable
fun Pane(supportingPane: @Composable () -> Unit) = supportingPane()

@Composable
fun Text(value: Any) {}

fun <T, F> hiltViewModel(key: String, factory: (F) -> T): T = TODO()

class PodcastDetailsViewModel {
    interface Factory
}

@Composable
fun Tracked(viewModel: com.example.HomeViewModel) {
    val content = remember2(viewModel)
    val alias = viewModel
    Pane2(content)
    Pane2(alias)
}

fun remember2(vm: Any): Any = vm

@Composable
fun Pane2(value: Any) {}
