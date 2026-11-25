package p

class B {
    private fun helper() = 1
    private fun overloaded(a: Int) = a
    private fun overloaded(a: Int, b: Int) = a + b

    inner class I : Thread() {
        fun x() { helper() }
    }

    inner class J {
        private fun helper() = 2
        fun x() = helper()
    }

    fun scope(sb: StringBuilder) {
        sb.apply { helper() }
        val f = ::helper
        listOf(1).map { helper() }
        class Local {
            fun y() = helper()
        }
        overloaded(1)
    }

    object Obj {
        private fun secret() = 0
        fun pub() = secret()
    }

    fun useObj() = Obj.pub()

    inner class K {
        fun x() = overloaded(1, 2)
    }
}

private fun String.ext() = length

class C {
    fun x() = "a".ext()
}
