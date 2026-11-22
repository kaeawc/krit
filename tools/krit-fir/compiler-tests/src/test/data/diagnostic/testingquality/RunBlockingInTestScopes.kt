// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 18, 24, 26
package test.runblocking.scopes

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.runTest
import org.junit.Test

interface BlockingInterfaceTests {
    @Test
    fun defaultTest() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
}

class BlockingCompanionTests {
    companion object {
        @Test
        fun companionTest() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
    }

    @Test
    fun localScopes() {
        fun unannotatedHelper() = runBlocking { assert(true) }
        @Test fun annotatedLocal() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
        val anonymous = object {
            @Test fun anonymousMember() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
            fun ordinaryMember() = runBlocking { assert(true) }
        }
        unannotatedHelper()
        annotatedLocal()
        anonymous.anonymousMember()
    }
}

class DispatcherCompanionTests {
    private val dispatcher = Dispatchers.IO

    companion object {
        @Test
        fun companionOwner() = runBlocking { assert(true) }
    }
}

class DispatcherAnonymousObjectTests {
    private val dispatcher = Dispatchers.Default
    private val nested = object {
        @Test
        fun anonymousOwner() = runBlocking { assert(true) }
    }
}
