package com.example.tooling

import androidx.compose.ui.tooling.preview.Preview

@Preview(name = "phone", device = "spec:width=360dp,height=640dp")
@Preview(name = "tablet", device = "spec:width=1280dp,height=800dp")
annotation class DevicePreviews

@Preview
annotation class SinglePreview

@Preview
annotation class BadName

@Preview(name = "a")
@Preview(name = "b")
annotation class BadNamePreview

@DevicePreviews
annotation class NestedPreview

@DevicePreviews
@SinglePreview
annotation class NestedDouble

annotation class NotAPreview
