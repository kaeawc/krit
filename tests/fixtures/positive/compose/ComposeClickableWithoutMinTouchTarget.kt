package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun ComposeClickableWithoutMinTouchTarget(onTap: () -> Unit) {
	Box(
		modifier = Modifier
			.width(36.dp)
			.clickable { onTap() },
	)
}
