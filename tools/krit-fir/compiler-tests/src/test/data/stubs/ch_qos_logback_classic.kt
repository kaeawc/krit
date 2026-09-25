// Compiler-test source stubs; never packaged in the production artifact.
package ch.qos.logback.classic

// Java final class with static Level instances; app code never constructs it.
object Level {
    @JvmField
    val OFF: Level = TODO()

    @JvmField
    val ERROR: Level = TODO()

    @JvmField
    val WARN: Level = TODO()

    @JvmField
    val INFO: Level = TODO()

    @JvmField
    val DEBUG: Level = TODO()

    @JvmField
    val TRACE: Level = TODO()

    @JvmField
    val ALL: Level = TODO()

    fun toLevel(sArg: String?): Level = TODO()
}

class Logger private constructor() : org.slf4j.Logger {
    var level: Level?
        get() = TODO()
        set(value) = TODO()

    val effectiveLevel: Level
        get() = TODO()

    override val name: String
        get() = TODO()

    override val isTraceEnabled: Boolean
        get() = TODO()

    override val isDebugEnabled: Boolean
        get() = TODO()

    override val isInfoEnabled: Boolean
        get() = TODO()

    override val isWarnEnabled: Boolean
        get() = TODO()

    override val isErrorEnabled: Boolean
        get() = TODO()

    override fun trace(msg: String?) {
        TODO()
    }

    override fun trace(format: String?, arg: Any?) {
        TODO()
    }

    override fun trace(format: String?, arg1: Any?, arg2: Any?) {
        TODO()
    }

    override fun trace(format: String?, vararg arguments: Any?) {
        TODO()
    }

    override fun trace(msg: String?, t: Throwable?) {
        TODO()
    }

    override fun debug(msg: String?) {
        TODO()
    }

    override fun debug(format: String?, arg: Any?) {
        TODO()
    }

    override fun debug(format: String?, arg1: Any?, arg2: Any?) {
        TODO()
    }

    override fun debug(format: String?, vararg arguments: Any?) {
        TODO()
    }

    override fun debug(msg: String?, t: Throwable?) {
        TODO()
    }

    override fun info(msg: String?) {
        TODO()
    }

    override fun info(format: String?, arg: Any?) {
        TODO()
    }

    override fun info(format: String?, arg1: Any?, arg2: Any?) {
        TODO()
    }

    override fun info(format: String?, vararg arguments: Any?) {
        TODO()
    }

    override fun info(msg: String?, t: Throwable?) {
        TODO()
    }

    override fun warn(msg: String?) {
        TODO()
    }

    override fun warn(format: String?, arg: Any?) {
        TODO()
    }

    override fun warn(format: String?, arg1: Any?, arg2: Any?) {
        TODO()
    }

    override fun warn(format: String?, vararg arguments: Any?) {
        TODO()
    }

    override fun warn(msg: String?, t: Throwable?) {
        TODO()
    }

    override fun error(msg: String?) {
        TODO()
    }

    override fun error(format: String?, arg: Any?) {
        TODO()
    }

    override fun error(format: String?, arg1: Any?, arg2: Any?) {
        TODO()
    }

    override fun error(format: String?, vararg arguments: Any?) {
        TODO()
    }

    override fun error(msg: String?, t: Throwable?) {
        TODO()
    }
}
