package p

object Holder {
    private fun helper() = 1
    class User {
        fun test() = Holder.helper()
    }
}

class Other {
    fun helper() = 2
}

class Host {
    companion object {
        private fun helper() = 3
    }
    class Caller {
        fun call(Host: Other) = Host.helper()
    }
}

private fun String.toString(): String = "x"

class Strings {
    fun s(s: String) = s.toString()
}
