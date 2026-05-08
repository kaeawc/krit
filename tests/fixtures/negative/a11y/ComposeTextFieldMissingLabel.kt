package com.example

import androidx.compose.foundation.layout.Column
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue

@Composable
fun LabeledTextField() {
    var email by remember { mutableStateOf("") }

    TextField(
        value = email,
        onValueChange = { email = it },
        label = { Text("Email") },
    )
}

@Composable
fun TextSiblingProvidesLabel() {
    var name by remember { mutableStateOf("") }

    Column {
        Text("Name")
        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
        )
    }
}
