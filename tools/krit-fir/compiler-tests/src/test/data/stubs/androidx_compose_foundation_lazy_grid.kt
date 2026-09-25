// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.foundation.lazy.grid

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.lazy.LazyScopeMarker
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

@LazyScopeMarker
interface LazyGridScope {
    fun item(key: Any? = null, contentType: Any? = null, content: @Composable LazyGridItemScope.() -> Unit) {
        error("The method is not implemented")
    }

    fun items(
        count: Int,
        key: ((index: Int) -> Any)? = null,
        contentType: (index: Int) -> Any? = { null },
        itemContent: @Composable LazyGridItemScope.(index: Int) -> Unit,
    ) {
        error("The method is not implemented")
    }
}

@LazyScopeMarker
@Stable
interface LazyGridItemScope

inline fun <T> LazyGridScope.items(
    items: List<T>,
    noinline key: ((item: T) -> Any)? = null,
    noinline contentType: (item: T) -> Any? = { null },
    crossinline itemContent: @Composable LazyGridItemScope.(item: T) -> Unit,
) {
    TODO()
}

@Stable
interface GridCells {
    class Fixed(private val count: Int) : GridCells

    class Adaptive(private val minSize: Dp) : GridCells
}

@Composable
fun LazyVerticalGrid(
    columns: GridCells,
    modifier: Modifier = Modifier,
    contentPadding: PaddingValues = PaddingValues(0.dp),
    reverseLayout: Boolean = false,
    verticalArrangement: Arrangement.Vertical = if (!reverseLayout) Arrangement.Top else Arrangement.Bottom,
    horizontalArrangement: Arrangement.Horizontal = Arrangement.Start,
    userScrollEnabled: Boolean = true,
    content: LazyGridScope.() -> Unit,
) {
    TODO()
}
