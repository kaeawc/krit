package exceptions

object MyError : Exception("singleton error")

object AnotherError : RuntimeException("another singleton")
