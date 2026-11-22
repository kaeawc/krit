// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class A {
    class Context {
        fun getSystemService(name: String): Any = Any()
    }

    fun service(context: Context): String = context.getSystemService("service") as String
}

class B {
    class Context {
        fun getSystemService(name: String): Any? = null
    }
}
