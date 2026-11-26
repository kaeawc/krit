package com.example

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview

@Composable
fun NoModifier(text: String) {
    Text(text)
}

@Composable
fun HasModifier(text: String, modifier: Modifier = Modifier) {
    Text(text, modifier = modifier)
}

@Composable
internal fun InternalNoModifier() {
    Column {}
}

@Composable
private fun PrivateNoModifier() {
    Box {}
}

@Composable
fun SpacerOnly() {
    Spacer(Modifier)
}

@Composable
fun Spacer2() {
    Spacer2Impl()
}

@Composable
fun Spacer2Impl() {}

@Composable
fun CustomEmitter(content: @Composable () -> Unit) {
    Wrapper(modifier = Modifier) { content() }
}

@Composable
fun Wrapper(modifier: Modifier, content: @Composable () -> Unit) {
    Box(modifier) { content() }
}

@Composable
fun OnlyDialog(onDismiss: () -> Unit) {
    AlertDialog(onDismissRequest = onDismiss, confirmButton = { Text("ok") })
}

@Composable
fun ReturnsValue(): Int {
    Text("x")
    return remember { 1 }
}

@Preview
@Composable
fun ScreenPreview() {
    Text("preview")
}

@Composable
fun NullableModifier(modifier: Modifier?) {
    Text("x", modifier = modifier ?: Modifier)
}

interface Screen {
    @Composable
    fun Content(modifier: Modifier)
}

class ScreenImpl : Screen {
    @Composable
    override fun Content(modifier: Modifier) {
        Text("x")
    }
}

abstract class Base {
    @Composable
    abstract fun Render(modifier: Modifier)
}

@Composable
fun Modifier.Extension(other: Modifier) {
    Text("x", modifier = this.then(other))
}

@Composable
fun NestedEmitter() {
    val x = remember { 1 }
    if (x > 0) {
        listOf(1).forEach { Text("$it") }
    }
}

@Composable
fun WrongOrder(modifier: Modifier = Modifier, text: String, enabled: Boolean = true) {
    Text(text, modifier)
}

@Composable
fun DefaultsBeforeModifier(text: String, enabled: Boolean = true, modifier: Modifier = Modifier) {
    Text(text, modifier)
}

@Composable
fun TrailingLambda(text: String, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    Column(modifier) { Text(text); content() }
}

@Composable
fun NullableTrailing(text: String, modifier: Modifier = Modifier, onClick: (() -> Unit)?) {
    Text(text, modifier)
}

@Composable
fun MisplacedLambda(content: @Composable () -> Unit, text: String, modifier: Modifier = Modifier) {
    Column(modifier) { Text(text); content() }
}

@Composable
fun TwoModifiers(
    text: String,
    other: Modifier = Modifier,
    modifier: Modifier = Modifier,
) {
    Column(modifier) { Text(text, other) }
}

@Composable
fun Modifier.ReceiverOrder(a: Int = 1, b: String) {
    Text(b + a)
}

@Composable
fun AnnotatedParam(@Suppress("x") enabled: Boolean = true, text: String) {
    Text(text)
}
