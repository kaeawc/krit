package com.example

import com.squareup.moshi.JsonClass
import java.util.TreeMap

@JsonClass(generateAdapter = true)
data class ConcreteMaps(
  val hash: HashMap<String, Int>,
  val linked: LinkedHashMap<String, Int>,
  val tree: TreeMap<String, Int>,
)
