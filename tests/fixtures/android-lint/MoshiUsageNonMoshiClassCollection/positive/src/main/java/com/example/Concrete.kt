package com.example

import com.squareup.moshi.JsonClass
import java.util.LinkedList

@JsonClass(generateAdapter = true)
data class Concrete(
  val list: ArrayList<String>,
  val set: LinkedHashSet<String>,
  val linked: LinkedList<String>,
  val custom: Names,
)

class Names : ArrayList<String>()
