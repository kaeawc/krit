package com.example.negative

import java.util.logging.Logger

class SilentGreeter {
    private val log: Logger = Logger.getLogger(SilentGreeter::class.java.name)

    fun greet(name: String) {
        log.info("hello, $name")
    }
}
