// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.foundation.lazy

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@DslMarker
annotation class LazyScopeMarker

@LazyScopeMarker
interface LazyListScope {
    fun item(key: Any? = null, contentType: Any? = null, content: @Composable LazyItemScope.() -> Unit) {
        error("The method is not implemented")
    }

    fun items(
        count: Int,
        key: ((index: Int) -> Any)? = null,
        contentType: (index: Int) -> Any? = { null },
        itemContent: @Composable LazyItemScope.(index: Int) -> Unit,
    ) {
        error("The method is not implemented")
    }

    fun stickyHeader(key: Any? = null, contentType: Any? = null, content: @Composable LazyItemScope.() -> Unit) {
        error("The method is not implemented")
    }
}

@LazyScopeMarker
@Stable
interface LazyItemScope {
    fun Modifier.fillParentMaxSize(fraction: Float = 1f): Modifier

    fun Modifier.fillParentMaxWidth(fraction: Float = 1f): Modifier

    fun Modifier.fillParentMaxHeight(fraction: Float = 1f): Modifier
}

// The list overloads are extensions; the `key` lambda is what keyed-item
// rules inspect.
inline fun <T> LazyListScope.items(
    items: List<T>,
    noinline key: ((item: T) -> Any)? = null,
    noinline contentType: (item: T) -> Any? = { null },
    crossinline itemContent: @Composable LazyItemScope.(item: T) -> Unit,
) {
    TODO()
}

inline fun <T> LazyListScope.items(
    items: Array<T>,
    noinline key: ((item: T) -> Any)? = null,
    noinline contentType: (item: T) -> Any? = { null },
    crossinline itemContent: @Composable LazyItemScope.(item: T) -> Unit,
) {
    TODO()
}

inline fun <T> LazyListScope.itemsIndexed(
    items: List<T>,
    noinline key: ((index: Int, item: T) -> Any)? = null,
    crossinline contentType: (index: Int, item: T) -> Any? = { _, _ -> null },
    crossinline itemContent: @Composable LazyItemScope.(index: Int, item: T) -> Unit,
) {
    TODO()
}

@Stable
class LazyListState(firstVisibleItemIndex: Int = 0, firstVisibleItemScrollOffset: Int = 0) {
    val firstVisibleItemIndex: Int
        get() = TODO()

    val firstVisibleItemScrollOffset: Int
        get() = TODO()

    val isScrollInProgress: Boolean
        get() = TODO()

    suspend fun scrollToItem(index: Int, scrollOffset: Int = 0) {
        TODO()
    }

    suspend fun animateScrollToItem(index: Int, scrollOffset: Int = 0) {
        TODO()
    }
}

@Composable
fun rememberLazyListState(
    initialFirstVisibleItemIndex: Int = 0,
    initialFirstVisibleItemScrollOffset: Int = 0,
): LazyListState = TODO()

@Composable
fun LazyColumn(
    modifier: Modifier = Modifier,
    state: LazyListState = rememberLazyListState(),
    contentPadding: PaddingValues = PaddingValues(0.dp),
    reverseLayout: Boolean = false,
    verticalArrangement: Arrangement.Vertical = if (!reverseLayout) Arrangement.Top else Arrangement.Bottom,
    horizontalAlignment: Alignment.Horizontal = Alignment.Start,
    userScrollEnabled: Boolean = true,
    content: LazyListScope.() -> Unit,
) {
    TODO()
}

@Composable
fun LazyRow(
    modifier: Modifier = Modifier,
    state: LazyListState = rememberLazyListState(),
    contentPadding: PaddingValues = PaddingValues(0.dp),
    reverseLayout: Boolean = false,
    horizontalArrangement: Arrangement.Horizontal = if (!reverseLayout) Arrangement.Start else Arrangement.End,
    verticalAlignment: Alignment.Vertical = Alignment.Top,
    userScrollEnabled: Boolean = true,
    content: LazyListScope.() -> Unit,
) {
    TODO()
}
