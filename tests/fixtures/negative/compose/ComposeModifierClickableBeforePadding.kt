package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun ComposeModifierClickableBeforePadding(onTap: () -> Unit) {
	Box(
		modifier = Modifier
			.padding(16.dp)
			.clickable { onTap() },
	)
}
