package com.example.retained

import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State

@Composable
fun <T> produceRetainedState(initialValue: T, producer: suspend MutableState<T>.() -> Unit): State<T> = TODO()
