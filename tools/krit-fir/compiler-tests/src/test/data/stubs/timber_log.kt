// Compiler-test source stubs; never packaged in the production artifact.
package timber.log

// Timber 5 (Kotlin): `Timber.d(...)` resolves to the companion object
// `Forest`, so its callable id is timber/log/Timber.Forest.d.
class Timber private constructor() {
    abstract class Tree {
        open fun v(message: String?, vararg args: Any?) {
            TODO()
        }

        open fun v(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun v(t: Throwable?) {
            TODO()
        }

        open fun d(message: String?, vararg args: Any?) {
            TODO()
        }

        open fun d(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun d(t: Throwable?) {
            TODO()
        }

        open fun i(message: String?, vararg args: Any?) {
            TODO()
        }

        open fun i(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun i(t: Throwable?) {
            TODO()
        }

        open fun w(message: String?, vararg args: Any?) {
            TODO()
        }

        open fun w(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun w(t: Throwable?) {
            TODO()
        }

        open fun e(message: String?, vararg args: Any?) {
            TODO()
        }

        open fun e(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun e(t: Throwable?) {
            TODO()
        }

        open fun wtf(message: String?, vararg args: Any?) {
            TODO()
        }

        open fun wtf(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun wtf(t: Throwable?) {
            TODO()
        }

        open fun log(priority: Int, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun log(priority: Int, t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        open fun log(priority: Int, t: Throwable?) {
            TODO()
        }

        protected open fun isLoggable(tag: String?, priority: Int): Boolean = TODO()

        protected abstract fun log(priority: Int, tag: String?, message: String, t: Throwable?)
    }

    open class DebugTree : Tree() {
        protected open fun createStackElementTag(element: StackTraceElement): String? = TODO()

        override fun log(priority: Int, tag: String?, message: String, t: Throwable?) {
            TODO()
        }
    }

    companion object Forest : Tree() {
        @JvmStatic
        override fun v(message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun v(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun v(t: Throwable?) {
            TODO()
        }

        @JvmStatic
        override fun d(message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun d(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun d(t: Throwable?) {
            TODO()
        }

        @JvmStatic
        override fun i(message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun i(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun i(t: Throwable?) {
            TODO()
        }

        @JvmStatic
        override fun w(message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun w(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun w(t: Throwable?) {
            TODO()
        }

        @JvmStatic
        override fun e(message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun e(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun e(t: Throwable?) {
            TODO()
        }

        @JvmStatic
        override fun wtf(message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun wtf(t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun wtf(t: Throwable?) {
            TODO()
        }

        @JvmStatic
        override fun log(priority: Int, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun log(priority: Int, t: Throwable?, message: String?, vararg args: Any?) {
            TODO()
        }

        @JvmStatic
        override fun log(priority: Int, t: Throwable?) {
            TODO()
        }

        override fun log(priority: Int, tag: String?, message: String, t: Throwable?) {
            TODO()
        }

        @JvmStatic
        fun asTree(): Tree = TODO()

        @JvmStatic
        fun tag(tag: String): Tree = TODO()

        @JvmStatic
        fun plant(tree: Tree) {
            TODO()
        }

        @JvmStatic
        fun plant(vararg trees: Tree) {
            TODO()
        }

        @JvmStatic
        fun uproot(tree: Tree) {
            TODO()
        }

        @JvmStatic
        fun uprootAll() {
            TODO()
        }

        @JvmStatic
        fun forest(): List<Tree> = TODO()

        @get:JvmStatic
        val treeCount: Int
            get() = TODO()
    }
}
