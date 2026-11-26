package com.example

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun myButton() {
}

@Composable
fun MyButton() {
}

@Composable
fun Count(): Int {
    return 1
}

@Composable
fun count(): Int {
    return 1
}

@Composable
fun Modifier.lowercaseWithReceiver() {
}

@Composable
fun Modifier.UppercaseValueWithReceiver(): Int {
    return 2
}

@Composable
fun expressionBody(): Unit = Unit

@Composable
fun ExpressionValue(): Int = 3

@Composable
fun allowedLowercase() {
}

@Composable
fun AllowedValue(): Int {
    return 4
}

interface Screen {
    @Composable
    fun content()
}

class Impl : Screen {
    @Composable
    override fun content() {
    }
}

fun notComposable() {
}

@Composable
fun outer() {
    @Composable
    fun inner() {
    }
}
