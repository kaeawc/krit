package test

open class AppCompatActivity {
    open fun onCreate(savedInstanceState: Any?) {}
}

class LoginActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Any?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.login)
    }
}
