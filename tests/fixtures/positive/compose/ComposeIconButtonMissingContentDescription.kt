package fixtures.positive.compose

import androidx.compose.foundation.Image
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import coil.compose.AsyncImage

@Composable
fun ComposeIconButtonMissingContentDescriptionPositive() {
    IconButton(onClick = { }) {
        Icon(Icons.Filled.ArrowBack)
    }

    Image(
        painter = painterResource(R.drawable.avatar),
        modifier = Modifier,
    )

    AsyncImage(
        model = "https://example.com/avatar.png",
        modifier = Modifier,
    )
}
