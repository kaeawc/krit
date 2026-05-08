package test

interface Span {
    fun setAttribute(key: String, value: String)
    fun setAttributes(attributes: Attributes)
}

class Attributes {
    companion object {
        fun of(key: AttributeKey, value: String): Attributes = Attributes()
    }
}

class AttributeKey {
    companion object {
        fun stringKey(name: String): AttributeKey = AttributeKey()
    }
}

fun handle(span: Span, userTier: String) {
    span.setAttribute("user_tier", userTier)
    span.setAttributes(Attributes.of(AttributeKey.stringKey("region"), "us-central"))
    span.setAttributes(Attributes.of(AttributeKey.stringKey("debug_value"), "user_id"))
}
