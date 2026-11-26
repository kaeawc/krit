package com.example

import androidx.activity.compose.PredictiveBackHandler
import androidx.compose.runtime.Composable
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.map

@Composable
fun Back(enabled: Boolean) {
    PredictiveBackHandler { }
    PredictiveBackHandler { progress -> }
    PredictiveBackHandler(enabled) { progress -> progress.collect { } }
    PredictiveBackHandler { it.collect { } }
    PredictiveBackHandler { progress -> progress.collectLatest { } }
    PredictiveBackHandler { progress -> progress.map { it.progress } }
    PredictiveBackHandler(onBack = { progress -> println(progress) })
    PredictiveBackHandler { progress -> progress.collectIndexed { _, _ -> } }
    PredictiveBackHandler { _ -> }
}
