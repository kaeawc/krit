// Smoke: repeatable @Preview, @PreviewLightDark, and a multipreview annotation.
package stubs

import android.content.res.Configuration
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Devices
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.tooling.preview.PreviewLightDark

@Preview(name = "Default", showBackground = true, widthDp = 320)
@Preview(name = "Night", uiMode = Configuration.UI_MODE_NIGHT_YES, device = Devices.PIXEL)
@Composable
fun PreviewSmoke() {
    Text("preview")
}

@PreviewLightDark
@Composable
private fun LightDarkSmoke() {
    Text("both")
}

@Preview(fontScale = 1.5f)
@PreviewLightDark
annotation class MultiPreview

@MultiPreview
@Composable
private fun MultiSmoke() {
    Text("multi")
}
