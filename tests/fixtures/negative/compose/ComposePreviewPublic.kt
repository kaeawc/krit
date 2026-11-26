package fixtures.compose

import androidx.annotation.VisibleForTesting
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview

@Preview
@Composable
private fun PrivatePreview() {}

@Preview
@Composable
internal fun InternalPreview() {}

@VisibleForTesting
@Preview
@Composable
fun TestedPreview() {}

@Composable
fun NotAPreview() {}

@Preview
fun NotComposable() {}

