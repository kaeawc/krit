package style

// Multi-method interface — not SAM convertible
interface Worker {
    fun run()
    fun cancel()
}

fun example() {
    val worker = object : Worker {
        override fun run() {
            println("running")
        }
        override fun cancel() {
            println("cancelled")
        }
    }
}

// Abstract class — not an interface, can't SAM convert
abstract class Task {
    abstract fun execute()
}

fun abstractClassExample() {
    val task = object : Task() {
        override fun execute() {
            println("executing")
        }
    }
}

// Object that references 'this' — can't convert to lambda
fun selfReferenceExample() {
    val task: Runnable = object : Runnable {
        override fun run() {
            println("removing self")
            someList.remove(this)
        }
    }
}

val someList = mutableListOf<Any>()

// Multiple supertypes — can't SAM convert
interface Taggable

fun multipleSupertypes() {
    val task = object : Runnable, Taggable {
        override fun run() {
            println("running")
        }
    }
}

// Object with init block — can't SAM convert
fun initBlockExample() {
    val task = object : Runnable {
        init {
            println("initializing")
        }
        override fun run() {
            println("running")
        }
    }
}

// Object with property — can't SAM convert
fun propertyExample() {
    val task = object : Runnable {
        private var count = 0
        override fun run() {
            count++
            println("running $count")
        }
    }
}

// Non-fun Kotlin interface — can't SAM convert
interface Callback {
    fun onComplete()
}

fun kotlinInterfaceExample() {
    val cb = object : Callback {
        override fun onComplete() {
            println("done")
        }
    }
}

// Interface with default methods where only one is overridden
// (like DefaultLifecycleObserver)
interface LifecycleAware {
    fun onCreate() {}
    fun onDestroy() {}
    fun onResume() {}
}

fun defaultMethodsExample() {
    val observer = object : LifecycleAware {
        override fun onDestroy() {
            println("destroyed")
        }
    }
}

// Non-override method — not implementing interface
interface Marker

fun nonOverrideExample() {
    val obj = object : Marker {
        fun doSomething() {
            println("doing")
        }
    }
}
