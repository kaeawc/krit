package com.example

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity

// `android:onClick="onSubmitClicked"` in res/layout/activity_form.xml
// references this Activity, but the method below has the wrong signature
// (no View parameter). The framework resolves the handler by reflection and
// throws NoSuchMethodException at runtime when the user taps the button.
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    fun onSubmitClicked() {
        submitForm()
    }

    private fun submitForm() {}
}
