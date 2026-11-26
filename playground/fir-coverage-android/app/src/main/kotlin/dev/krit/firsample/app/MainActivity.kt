package dev.krit.firsample.app

import android.app.Activity
import android.os.Bundle
import dev.krit.firsample.app.databinding.ActivityMainBinding

class MainActivity : Activity() {
    private lateinit var binding: ActivityMainBinding
    private val presenter = ProfilePresenter()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // ActivityMainBinding is generated from res/layout/activity_main.xml.
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        binding.title.text = getString(R.string.app_name)
        binding.refreshButton.setOnClickListener { render() }
        render()
    }

    private fun render() {
        binding.profileLabel.text = presenter.label(SAMPLE_USER_JSON) ?: getString(R.string.refresh)
    }

    private companion object {
        const val SAMPLE_USER_JSON = """{"id":"u1","name":"Ada Lovelace","visits":3}"""
    }
}
