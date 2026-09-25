// Smoke: AsyncImage and rememberAsyncImagePainter.
package stubs

import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import coil.compose.AsyncImagePainter
import coil.compose.rememberAsyncImagePainter

@Composable
fun Avatar(url: String) {
    AsyncImage(
        model = url,
        contentDescription = "Avatar",
        modifier = Modifier.size(48.dp),
        placeholder = painterResource(android.R.drawable.ic_menu_add),
        contentScale = ContentScale.Crop,
    )
    val painter: AsyncImagePainter = rememberAsyncImagePainter(url)
    Image(painter = painter, contentDescription = null)
}
