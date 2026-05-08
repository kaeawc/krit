package test

import org.slf4j.Logger

fun recordLogin(logger: Logger, id: String) {
    logger.info("user $id logged in")
}
