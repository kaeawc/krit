package test

class Sample(private val title: TitleView) {
    fun show(userName: String, email: String) {
        title.text = userName.uppercase()
        title.setHint(email.lowercase())
    }
}

class TitleView {
    var text: String = ""

    fun setHint(value: String) {
        text = value
    }
}
