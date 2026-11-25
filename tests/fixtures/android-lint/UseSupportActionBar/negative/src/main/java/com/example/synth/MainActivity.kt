package com.example.synth

import android.app.Activity
import android.os.Bundle
import android.widget.Toolbar
import androidx.appcompat.app.AppCompatActivity

class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setSupportActionBar(androidx.appcompat.widget.Toolbar(this))
        // setActionBar(toolbar) in a comment is not a call
        val note = "setActionBar(toolbar)"
    }
}

// A framework Activity has no support action bar to use instead.
class PlainActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setActionBar(Toolbar(this))
    }
}

// A local lookalike with the same method name.
class ToolbarHost {
    fun setActionBar(toolbar: Toolbar?) {}

    fun install() {
        setActionBar(null)
    }
}
