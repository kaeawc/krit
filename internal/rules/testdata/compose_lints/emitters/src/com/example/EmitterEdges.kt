package com.example.edges

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

object Ui {
    @Composable
    fun Text(text: String) {}
}

@Composable
fun Outer() {
    @Composable
    fun Inner() {
        Text("a")
        Text("b")
    }
    Inner()
}

@Composable
fun QualifiedReturns(): Int {
    Ui.Text("a")
    return 1
}

@Composable
fun SafeQualified(ui: Ui?): Int {
    ui?.Text("a")
    return 1
}

@Composable
fun LoopSingle(items: List<String>) {
    for (item in items) Text(item)
}

@Composable
fun CustomTwo() {
    MyCard()
    MyCard()
}

@Composable
fun MyCard() {}

@Composable
fun CustomReturns(): Int {
    MyCard()
    return 1
}
