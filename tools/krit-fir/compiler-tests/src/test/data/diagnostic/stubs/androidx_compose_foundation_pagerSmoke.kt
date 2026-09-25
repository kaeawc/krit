// Smoke: the PagerState-based pager API (pageCount lives on the state).
package stubs

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.PagerState
import androidx.compose.foundation.pager.VerticalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun PagerSmoke() {
    val pagerState: PagerState = rememberPagerState(pageCount = { 3 })
    HorizontalPager(state = pagerState, modifier = Modifier.fillMaxSize(), pageSpacing = 8.dp, key = { it }) { page ->
        Text("Page $page of ${pagerState.pageCount}, current ${pagerState.currentPage}")
    }
    VerticalPager(state = rememberPagerState { 2 }) { page -> Text("$page") }
    LaunchedEffect(Unit) { pagerState.animateScrollToPage(1) }
}
