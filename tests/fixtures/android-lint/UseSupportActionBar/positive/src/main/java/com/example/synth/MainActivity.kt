package com.example.synth

import android.os.Bundle
import android.widget.Toolbar
import androidx.appcompat.app.AppCompatActivity

class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val toolbar = Toolbar(this)
        setActionBar(toolbar)
    }
}

open class BaseActivity : AppCompatActivity()

class DetailActivity : BaseActivity() {
    fun install(toolbar: Toolbar) {
        this.setActionBar(toolbar)
    }
}
