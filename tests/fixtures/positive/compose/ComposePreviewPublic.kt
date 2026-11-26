package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.tooling.preview.PreviewLightDark

@Preview
@Composable
fun ButtonPreview() {}

@PreviewLightDark
@Composable
fun ThemedPreview() {}

@ScreenPreviews
@Composable
fun ScreensPreview() {}

@Preview(name = "phone")
@Preview(name = "tablet")
annotation class ScreenPreviews
