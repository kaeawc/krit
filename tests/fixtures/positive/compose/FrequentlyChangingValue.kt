package fixtures.compose

import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.Composable

@Composable
fun ScrollToTopButton(state: LazyListState, show: (Boolean) -> Unit) {
    show(state.firstVisibleItemIndex > 0)
}
