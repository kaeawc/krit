package fixtures.negative.compose

import androidx.compose.foundation.Image
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.invisibleToUser
import androidx.compose.ui.semantics.semantics
import coil.compose.AsyncImage

@Composable
fun ComposeIconButtonMissingContentDescriptionNegative() {
    IconButton(onClick = { }) {
        Icon(
            imageVector = Icons.Filled.ArrowBack,
            contentDescription = "Back",
        )
    }

    IconButton(onClick = { }) {
        Icon(
            imageVector = Icons.Filled.ArrowBack,
            contentDescription = null,
        )
    }

    Image(
        painter = painterResource(R.drawable.avatar),
        contentDescription = "Avatar",
        modifier = Modifier,
    )

    Image(
        painter = painterResource(R.drawable.avatar),
        contentDescription = null,
        modifier = Modifier,
    )

    AsyncImage(
        model = "https://example.com/avatar.png",
        contentDescription = null,
        modifier = Modifier.semantics { invisibleToUser() },
    )
}
