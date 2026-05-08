package com.example

import androidx.compose.foundation.Image
import androidx.compose.runtime.Composable

@Composable
fun ProfileScreen() {
    Image(
        painter = painterResource(R.drawable.profile),
        modifier = Modifier.size(48.dp)
    )
}
