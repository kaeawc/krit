package com.example.ui

import androidx.annotation.VisibleForTesting
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.tooling.preview.PreviewLightDark
import com.example.tooling.DevicePreviews
import com.example.tooling.NestedPreview
import com.example.tooling.NotAPreview

@Composable
fun Screen() {
}

@Preview
@Composable
fun ScreenPreview() {
    Screen()
}

/** Documented preview. */
@Preview
@Composable
public fun PublicScreenPreview() {
    Screen()
}

@Preview
@Composable
private fun PrivateScreenPreview() {
    Screen()
}

@Preview
@Composable
internal fun InternalScreenPreview() {
    Screen()
}

@PreviewLightDark
@Composable
fun LightDarkPreview() {
    Screen()
}

@DevicePreviews
@Composable
fun DevicesPreview() {
    Screen()
}

@NestedPreview
@Composable
fun NestedScreenPreview() {
    Screen()
}

@NotAPreview
@Composable
fun NotPreviewed() {
    Screen()
}

@VisibleForTesting
@Preview
@Composable
fun TestingPreview() {
    Screen()
}

@Preview
fun NotComposablePreview() {
}

class Holder {
    @Preview
    @Composable
    fun MemberPreview() {
        Screen()
    }
}

@LocalPreviews
@Composable
fun LocalMultiPreview() {
    Screen()
}

@Preview
@Preview(fontScale = 2f)
annotation class LocalPreviews

@Preview
annotation class LocalOne

@Preview
annotation class Plain
