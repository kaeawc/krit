package com.example

class RepositoryTest {
    // A real instance, not a mock.
    private val repository = Repository()

    // A local helper that only shares the factory's name.
    private fun <T> mock(value: T): T = value

    private val other = mock(Repository())
}
