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

fun handle(span: Span, userId: String, sessionId: String) {
    span.setAttribute("user_id", userId)
    span.setAttributes(Attributes.of(AttributeKey.stringKey("session_id"), sessionId))
}
