package fixtures.negative.compose

import androidx.compose.foundation.Image
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.clearAndSetSemantics

@Composable
fun ComposeDecorativeImageContentDescriptionNegative() {
    Image(
        painter = painterResource(R.drawable.decoration),
        contentDescription = null,
        modifier = Modifier.clearAndSetSemantics { },
    )
}
