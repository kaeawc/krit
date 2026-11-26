package fixtures.androidlint

import androidx.test.uiautomator.textAsString
import androidx.test.uiautomator.uiAutomator

fun login() = uiAutomator {
    onElement { textAsString() == "Login" }.click()
}
