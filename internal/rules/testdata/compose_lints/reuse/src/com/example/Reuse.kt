package com.example

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun SlotTwice(content: @Composable () -> Unit) {
    content()
    content()
}

@Composable
fun SlotBranches(flag: Boolean, content: @Composable () -> Unit) {
    if (flag) content() else content()
}

@Composable
fun SlotEarlyReturn(flag: Boolean, content: @Composable () -> Unit) {
    if (flag) {
        content()
        return
    }
    content()
}

@Composable
fun SlotInColumn(content: @Composable () -> Unit) {
    Column {
        content()
        content()
    }
}

@Composable
fun SlotLoop(items: List<Int>, content: @Composable () -> Unit) {
    for (item in items) {
        content()
    }
}

@Composable
fun SlotForEach(items: List<Int>, content: @Composable () -> Unit) {
    items.forEach { content() }
}

@Composable
fun SlotOnClick(content: @Composable () -> Unit, onClick: () -> Unit) {
    Button(onClick = { onClick() }) { content() }
    Box { content() }
}

@Composable
fun SlotPassed(content: @Composable () -> Unit) {
    Wrapper(content)
    content()
}

@Composable
fun Wrapper(content: @Composable () -> Unit) {
    Box { content() }
}

@Composable
fun SlotLet(content: @Composable () -> Unit) {
    content.let { it() }
    content()
}

@Composable
fun SlotNullable(content: (@Composable () -> Unit)?) {
    content?.invoke()
    content?.invoke()
}

@Composable
fun SlotRun(content: @Composable () -> Unit) {
    run { content() }
    content()
}

@Composable
fun ModifierTwice(modifier: Modifier = Modifier) {
    Text("a", modifier = modifier)
    Text("b", modifier = modifier)
}

@Composable
fun ModifierChained(modifier: Modifier = Modifier) {
    Column(modifier = modifier) {
        Text("a", modifier = modifier.padding(4.dp))
    }
}

@Composable
fun ModifierBranches(flag: Boolean, modifier: Modifier = Modifier) {
    if (flag) {
        Text("a", modifier = modifier)
    } else {
        Text("b", modifier = modifier)
    }
}

@Composable
fun ModifierFreshInner(modifier: Modifier = Modifier) {
    Column(modifier) {
        Text("a", Modifier.fillMaxWidth())
    }
}

@Composable
fun ModifierAlias(modifier: Modifier = Modifier) {
    val m = modifier.fillMaxWidth()
    Row(m) {}
    Text("x", modifier = m)
}

@Composable
fun ModifierEffect(modifier: Modifier = Modifier) {
    LaunchedEffect(modifier) {}
    Text("x", modifier = modifier)
}

@Composable
fun ModifierMissingBranch(flag: Boolean, modifier: Modifier = Modifier) {
    if (flag) {
        Text("a", modifier = modifier)
    } else {
        Text("b")
    }
}

@Composable
fun ModifierReturn(flag: Boolean, modifier: Modifier = Modifier) {
    if (flag) {
        Text("a", modifier = modifier)
        return
    }
    Text("b", modifier = modifier)
}
