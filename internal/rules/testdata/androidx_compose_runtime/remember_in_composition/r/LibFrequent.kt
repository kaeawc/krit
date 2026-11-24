package r

import androidx.compose.foundation.gestures.AnchoredDraggableState
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.pager.PagerState
import androidx.compose.material3.SearchBarState
import androidx.compose.runtime.Composable
import androidx.compose.ui.layout.LayoutBoundsHolder

@Composable
fun LibFrequent(grid: LazyGridState, pager: PagerState, drag: AnchoredDraggableState<Int>, search: SearchBarState, holder: LayoutBoundsHolder) {
    grid.firstVisibleItemIndex
    grid.layoutInfo
    rememberLazyGridState().firstVisibleItemScrollOffset
    pager.currentPageOffsetFraction
    pager.currentPage
    drag.offset
    drag.progress
    drag.requireOffset()
    drag.progress(1, 2)
    search.progress
    holder.bounds
}
