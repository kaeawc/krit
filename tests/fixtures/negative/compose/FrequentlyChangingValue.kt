package fixtures.compose

import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.remember

@Composable
fun ScrollToTopButton(state: LazyListState, show: (Boolean) -> Unit) {
    val showButton = remember(state) { derivedStateOf { state.firstVisibleItemIndex > 0 } }
    show(showButton.value)
}

class Pager(val firstVisibleItemIndex: Int)

@Composable
fun Lookalike(pager: Pager) {
    val index = pager.firstVisibleItemIndex
}
