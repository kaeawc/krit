package r

import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.annotation.FrequentlyChangingValue
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.remember

@FrequentlyChangingValue
fun frequent(): Int = 1

class Box {
    @get:FrequentlyChangingValue
    val value: Int get() = 1
    @FrequentlyChangingValue
    fun calculateValue(): Int = 1
}

@Composable
fun Reads(state: LazyListState, scroll: ScrollState, box: Box) {
    frequent()
    box.value
    box.calculateValue()
    state.firstVisibleItemIndex
    state.firstVisibleItemScrollOffset
    state.layoutInfo
    scroll.value
    val local = rememberLazyListState()
    local.firstVisibleItemIndex
    val s2 = rememberScrollState()
    s2.value
    val ok = remember { derivedStateOf { state.firstVisibleItemIndex > 0 } }
    val onClick = { state.firstVisibleItemIndex }
    state.canScrollForward
}

fun notComposable(state: LazyListState) = state.firstVisibleItemIndex
