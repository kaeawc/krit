// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Text that is not reported: resource strings, strings with nothing to translate, previews and samples, preview-only
// branches, and code outside a function.
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalInspectionMode
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview

object R {
    object string {
        const val welcome = 1
    }
}

@Composable
fun Resource() {
    Text(stringResource(R.string.welcome))
}

@Composable
fun NothingToTranslate(count: Int) {
    Text(" ")
    Text("•")
    Text("$count")
    Text("12:30")
    Text("")
}

@Composable
fun Dynamic(label: String) {
    Text(label)
    Text(label + "!")
}

@Preview
@Composable
fun HeaderPreview() {
    Text("Welcome")
}

@Composable
fun SampleHeader() {
    Text("Welcome")
}

@Composable
fun FakeHeader() {
    Text("Welcome")
}

@Composable
fun StubbedHeader() {
    Text("Welcome")
}

annotation class ThemePreviews

@ThemePreviews
@Composable
fun Themed() {
    Text("Welcome")
}

@Composable
private fun PreviewOnlyHelper() {
    Text("Welcome")
}

@Preview
@Composable
fun HelperPreview() {
    PreviewOnlyHelper()
}

@Composable
fun InspectionBranch() {
    if (LocalInspectionMode.current) {
        Text("Preview text")
    } else {
        Text(stringResource(R.string.welcome))
    }
}

@Composable
fun NegatedInspectionBranch() {
    if (!LocalInspectionMode.current) {
        Text(stringResource(R.string.welcome))
    } else {
        Text("Preview text")
    }
}

@Composable
fun InspectionLocal() {
    val inPreview = LocalInspectionMode.current
    if (inPreview) {
        Text("Preview text")
    }
}

// No enclosing function: Go does not report a Text outside one.
val Greeting: @Composable () -> Unit = { Text("Hello") }
