// Smoke: cast the SLF4J root logger to Logback's Logger and set its level.
package stubs

import ch.qos.logback.classic.Level
import ch.qos.logback.classic.Logger
import org.slf4j.LoggerFactory

fun configureLogback() {
    val root = LoggerFactory.getLogger(org.slf4j.Logger.ROOT_LOGGER_NAME) as Logger
    root.level = Level.DEBUG
    root.info("configured {}", root.name)
    if (root.isDebugEnabled) root.debug("debug")
}
