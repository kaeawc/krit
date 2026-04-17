package com.example

import android.content.ClipData
import android.content.ClipboardManager
import android.os.Bundle
import android.widget.EditText
import androidx.appcompat.app.AppCompatActivity

class ClipboardOnSensitiveInputType : AppCompatActivity() {
    private lateinit var clipboardManager: ClipboardManager

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_login)

        val emailField = findViewById<EditText>(R.id.emailField)
        clipboardManager.setPrimaryClip(ClipData.newPlainText("", emailField.text))
    }
}
