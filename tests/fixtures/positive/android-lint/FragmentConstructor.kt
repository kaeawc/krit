package com.example

import androidx.fragment.app.Fragment

class MyFragment(val id: Int) : Fragment() {
    // Has parameterized constructor without no-arg constructor
}

class LoginFragment(private val userId: String) : Fragment() {
    // No no-arg constructor — crashes on back-stack restoration
    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?
    ) = inflater.inflate(R.layout.fragment_login, container, false)
}
