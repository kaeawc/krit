package test

@RequiresOptIn(level = RequiresOptIn.Level.ERROR)
annotation class InternalApi

@RequiresOptIn(level = RequiresOptIn.Level.WARNING, message = "experimental")
annotation class ExperimentalApi

class NotAnAnnotation
