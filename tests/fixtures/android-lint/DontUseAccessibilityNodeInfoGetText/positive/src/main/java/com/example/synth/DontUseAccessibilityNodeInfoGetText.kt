package com.example.synth

import androidx.test.uiautomator.uiAutomator

fun login() = uiAutomator {
    onElement { text == "Login" }.click()
}
