package com.example

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.NonRestartableComposable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview

@Composable
fun Two() {
    Text("a")
    Text("b")
}

@Composable
fun One() {
    Column {
        Text("a")
        Text("b")
    }
}

/** Doc. */
@Composable
fun WithSpacer() {
    Text("a")
    Spacer(Modifier)
}

@Composable
fun Indirect() {
    One()
    Text("c")
}

@Composable
fun IndirectTwice() {
    One()
    Indirect()
}

@Composable
fun Looping(items: List<String>) {
    for (item in items) {
        Text(item)
    }
}

@Composable
fun Conditional(flag: Boolean) {
    if (flag) Text("a") else Text("b")
}

@Composable
fun EmitsAndReturns(): Int {
    Text("a")
    return remember { 1 }
}

@Composable
fun IndirectReturns(): String {
    One()
    return "x"
}

@Composable
fun Modifier.Receiver() {
    Text("a")
    Text("b")
}

@Composable
fun Wrapper(text: String) {
    Label(text)
}

@Composable
fun Label(text: String) {
    Text(text)
}

@Composable
fun WrapperWithLiteral() {
    Label("fixed")
}

@Composable
fun WrapperComputed(text: String) {
    Label(text.uppercase())
}

@NonRestartableComposable
@Composable
fun AlreadyNonRestartable(text: String) {
    Label(text)
}

@Composable
fun WrapsLibrary(text: String) {
    Text(text)
}

@Composable
fun WrapsReadOnly() {
    readOnly()
}

@Composable
@ReadOnlyComposable
fun readOnly() {}

@Composable
fun WrapsInline(text: String) {
    InlineLabel(text)
}

@Composable
inline fun InlineLabel(text: String) {
    Text(text)
}

@Composable
fun WrapsValue(text: String) {
    valueReturning(text)
}

@Composable
fun valueReturning(text: String): Int = text.length

@Composable
fun ExpressionWrapper(text: String) = Label(text)

@Preview
@Composable
fun PreviewWrapper() {
    Label("p")
}

@Composable
private fun PrivateWrapper(text: String) {
    Label(text = text)
}

class Holder {
    @Composable
    fun Member(text: String) {
        this.Render(text)
    }

    @Composable
    fun Render(text: String) {
        Text(text)
    }
}
