package com.example.app.fragments

import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.TextView
import androidx.fragment.app.Fragment
import com.example.app.R

// FragmentConstructor: Fragment with parameters and no default constructor
class HomeFragment(private val title: String) : Fragment() {

    private var titleView: TextView? = null

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View? {
        return inflater.inflate(R.layout.fragment_home, container, false)
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        titleView = view.findViewById(R.id.text_title)
        titleView?.text = title

        setupClickListeners(view)
    }

    // LongParameterList
    fun updateContent(
        title: String,
        subtitle: String,
        description: String,
        imageUrl: String,
        category: String,
        tags: List<String>,
        isPublished: Boolean
    ) {
        titleView?.text = title
    }

    // EmptyCatchBlock
    private fun setupClickListeners(view: View) {
        try {
            val button = view.findViewById<View>(R.id.btn_action)
            button.setOnClickListener { navigateToDetail() }
        } catch (e: Exception) {
        }
    }

    private fun navigateToDetail() {
        // TODO: implement navigation
    }

    override fun onDestroyView() {
        super.onDestroyView()
        titleView = null
    }
}
