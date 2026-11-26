package com.example

class ResourceProvider {
  fun getQuantityString(id: Int, count: Int, vararg args: Any): String = ""
}

class Foo {
  val resources = ResourceProvider()
}

class Repo(private val resources: ResourceProvider, private val ctx: Foo) {
  fun a(n: Int) = resources.getQuantityString(1, n, n)
  fun b(n: Int) = ctx.resources.getQuantityString(1, n, n)
}
