// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.foundation.pager

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// Current (1.5+) API: the page count lives on PagerState; the pageCount
// parameter on the pager composables was removed.
@Stable
abstract class PagerState(currentPage: Int = 0, currentPageOffsetFraction: Float = 0f) {
    abstract val pageCount: Int

    val currentPage: Int
        get() = TODO()

    val settledPage: Int
        get() = TODO()

    suspend fun scrollToPage(page: Int, pageOffsetFraction: Float = 0f) {
        TODO()
    }

    suspend fun animateScrollToPage(page: Int, pageOffsetFraction: Float = 0f) {
        TODO()
    }
}

@Composable
fun rememberPagerState(
    initialPage: Int = 0,
    initialPageOffsetFraction: Float = 0f,
    pageCount: () -> Int,
): PagerState = TODO()

interface PagerScope

@Composable
fun HorizontalPager(
    state: PagerState,
    modifier: Modifier = Modifier,
    contentPadding: PaddingValues = PaddingValues(0.dp),
    beyondViewportPageCount: Int = 0,
    pageSpacing: Dp = 0.dp,
    verticalAlignment: Alignment.Vertical = Alignment.CenterVertically,
    userScrollEnabled: Boolean = true,
    reverseLayout: Boolean = false,
    key: ((index: Int) -> Any)? = null,
    pageContent: @Composable PagerScope.(page: Int) -> Unit,
) {
    TODO()
}

@Composable
fun VerticalPager(
    state: PagerState,
    modifier: Modifier = Modifier,
    contentPadding: PaddingValues = PaddingValues(0.dp),
    beyondViewportPageCount: Int = 0,
    pageSpacing: Dp = 0.dp,
    horizontalAlignment: Alignment.Horizontal = Alignment.CenterHorizontally,
    userScrollEnabled: Boolean = true,
    reverseLayout: Boolean = false,
    key: ((index: Int) -> Any)? = null,
    pageContent: @Composable PagerScope.(page: Int) -> Unit,
) {
    TODO()
}
