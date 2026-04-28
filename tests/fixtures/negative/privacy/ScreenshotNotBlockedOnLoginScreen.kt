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

annotation class Composable
annotation class Preview

@Composable
@Preview
fun PaymentScreenPreview() {
    Text("Preview only")
}

@Composable
fun ShippingAddressView() {
    Text("Address")
}

@Composable
fun RewardCard() {
    Text("Reward")
}

class StoredCard

@Composable
fun KSCardElement(card: StoredCard) {
    Text(card.toString())
}
