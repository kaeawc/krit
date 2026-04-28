package fixtures.positive.compose

import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource

@Composable
fun ComposeDecorativeImageContentDescriptionPositive() {
    Image(
        painter = painterResource(R.drawable.decoration),
        contentDescription = null,
        modifier = Modifier.clickable { },
    )
}
