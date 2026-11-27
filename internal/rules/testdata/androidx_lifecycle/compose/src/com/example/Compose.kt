package com.example

import android.app.Application
import androidx.compose.material.Button
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.LocalLifecycleOwner

class MyViewModel : ViewModel()
open class BaseViewModel : ViewModel()
class ChildViewModel : BaseViewModel()
class AppViewModel(app: Application) : AndroidViewModel(app)
class NotAViewModel

@Composable
fun States(lifecycle: Lifecycle, owner: LifecycleOwner) {
    val a = lifecycle.currentState
    val b = owner.lifecycle.currentState
    val c = LocalLifecycleOwner.current.lifecycle.currentState
    LaunchedEffect(Unit) { println(lifecycle.currentState) }
    Button(onClick = { println(lifecycle.currentState) }) { Text("x") }
    if (lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)) Text("y")
    val d = remember { lifecycle.currentState }
}

fun notComposable(lifecycle: Lifecycle) = lifecycle.currentState

@Composable
fun ViewModels(app: Application) {
    val a = MyViewModel()
    val b = ChildViewModel()
    val c = AppViewModel(app)
    val d = NotAViewModel()
    val e = remember { MyViewModel() }
    Button(onClick = { MyViewModel() }) { Text("x") }
    LaunchedEffect(Unit) { MyViewModel() }
}

fun notComposableVm() = MyViewModel()
