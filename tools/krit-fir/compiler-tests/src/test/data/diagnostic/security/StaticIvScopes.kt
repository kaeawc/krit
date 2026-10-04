// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 16, 20, 25, 29, 33, 37, 39, 42, 43, 44
// Scope coverage: Go visits every call expression in the file, so a spec built
// from literal bytes is reported wherever it appears.
package test

import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.IvParameterSpec

open class Base(val spec: IvParameterSpec)

class Delegating() : Base(<!StaticIv!>IvParameterSpec(byteArrayOf(1))<!>) {
    val fromInit: IvParameterSpec

    init {
        fromInit = <!StaticIv!>IvParameterSpec(byteArrayOf(2))<!>
    }

    constructor(flag: Boolean) : this() {
        if (flag) <!StaticIv!>GCMParameterSpec(96, byteArrayOf(3))<!>
    }
}

class Secondary : Base {
    constructor() : super(<!StaticIv!>IvParameterSpec(byteArrayOf(4))<!>)
}

enum class Specs(val spec: IvParameterSpec) {
    FIXED(<!StaticIv!>IvParameterSpec(byteArrayOf(5))<!>),
}

interface Provider {
    fun spec(): IvParameterSpec = <!StaticIv!>IvParameterSpec(byteArrayOf(6))<!>
}

val String.spec: IvParameterSpec
    get() = <!StaticIv!>IvParameterSpec(byteArrayOf(7))<!>

fun withDefault(spec: IvParameterSpec = <!StaticIv!>IvParameterSpec(byteArrayOf(8))<!>) = spec

fun local(): IvParameterSpec {
    fun inner() = <!StaticIv!>IvParameterSpec(byteArrayOf(9))<!>
    val lambda = { <!StaticIv!>IvParameterSpec(byteArrayOf(10))<!> }
    val anonymous = fun() = <!StaticIv!>IvParameterSpec(byteArrayOf(11))<!>
    lambda()
    anonymous()
    return inner()
}
