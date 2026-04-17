package test

open class AppCompatActivity {
    open fun onCreate(savedInstanceState: Any?) {}
}

class LoginActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Any?) {
        super.onCreate(savedInstanceState)
        window.setFlags(FLAG_SECURE, FLAG_SECURE)
        setContentView(R.layout.login)
    }
}
