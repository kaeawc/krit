// Smoke: Image, background with a shape, clickable with a11y params, scroll modifiers.
package stubs

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp

@Composable
fun ClickableCard(onOpen: () -> Unit) {
    val scroll = rememberScrollState()
    Image(
        painter = painterResource(android.R.drawable.ic_menu_add),
        contentDescription = "Add",
        modifier = Modifier
            .background(Color.White, RoundedCornerShape(8.dp))
            .background(Color(0xFF000000), CircleShape)
            .background(color = Color.Red)
            .clickable(onClickLabel = "Open", role = Role.Button) { onOpen() }
            .clickable(enabled = false, onClick = onOpen)
            .verticalScroll(scroll)
            .horizontalScroll(rememberScrollState()),
        alignment = Alignment.Center,
        contentScale = ContentScale.Crop,
        colorFilter = ColorFilter.tint(Color.Black),
    )
    println(scroll.value)
}
