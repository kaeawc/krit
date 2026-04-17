package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview

object R {
	object string {
		const val welcome = 1
	}
}

@Composable
fun Header() {
	Text(stringResource(R.string.welcome))
}

@Preview
@Composable
fun HeaderPreview() {
	Text("Welcome")
}
