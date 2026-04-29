package com.example

import android.os.Bundle
import android.view.View
import android.widget.Button
import androidx.appcompat.app.AppCompatActivity

// `android:onClick="onSubmitClicked"` in res/layout/activity_form.xml
// references this Activity. The method below has the required public
// `fun name(view: View)` signature, so the runtime reflection lookup
// succeeds.
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    fun onSubmitClicked(view: View) {
        submitForm()
    }

    private fun submitForm() {}
}

// Programmatic listeners avoid `android:onClick` entirely — no risk of a
// reflective NoSuchMethodException.
class ProgrammaticActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
        findViewById<Button>(R.id.submit).setOnClickListener { submitForm() }
    }

    private fun submitForm() {}
}
