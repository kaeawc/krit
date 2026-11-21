// Smoke: atomicfu lock construction and lock helpers.
package stubs

import kotlinx.atomicfu.locks.ReentrantLock
import kotlinx.atomicfu.locks.reentrantLock
import kotlinx.atomicfu.locks.synchronized
import kotlinx.atomicfu.locks.withLock

fun atomicfuLocksSmoke(lock: ReentrantLock = reentrantLock()): Int =
    synchronized(lock) { lock.withLock { 1 } }
