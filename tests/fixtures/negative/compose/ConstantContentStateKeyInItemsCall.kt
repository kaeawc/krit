package fixtures.compose

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionLayout
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun Gallery(names: List<String>, visible: Boolean) {
    SharedTransitionLayout {
        AnimatedVisibility(visible) {
            LazyColumn {
                items(names) { name ->
                    Text(name, Modifier.sharedElement(rememberSharedContentState("photo-$name"), this@AnimatedVisibility))
                }
                item {
                    Text("header", Modifier.sharedElement(rememberSharedContentState("header"), this@AnimatedVisibility))
                }
            }
        }
    }
}
