package stubs

import test.logconditional.external.Config

fun logConditionalExternalValues(): String =
    Config.prefix + Config.timeoutMs + Config.runtimePrefix
