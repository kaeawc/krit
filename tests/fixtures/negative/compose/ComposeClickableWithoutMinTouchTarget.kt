package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun ComposeClickableWithoutMinTouchTarget(onTap: () -> Unit) {
	Box(
		modifier = Modifier
			.clickable { onTap() },
	)
}
