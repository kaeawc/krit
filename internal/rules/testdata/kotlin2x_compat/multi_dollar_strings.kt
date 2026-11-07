// rules: UnusedPrivateProperty, UnusedImport, StringShouldBeRawString
package compat

import compat.json.Schema
import compat.json.Unused // expect: UnusedImport

private val usedViaInterpolation = 1
private val onlyLiteralText = 2 // expect: UnusedPrivateProperty
private val usedPlain = 3
private val usedTriple = 4
private val onlyLiteralTriple = 5 // expect: UnusedPrivateProperty

// With `$$` only `$$` starts an interpolation, so `$name` and `$schema` are plain text.
val s1 = $$"value=$${usedViaInterpolation} text=$onlyLiteralText"
val s2 = "plain $usedPlain"
val s3 = $$$"x $$${usedTriple} y $$onlyLiteralTriple"
val s4 = $$"""
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "$${Schema.ID}"
}
""".trimIndent()
val s5 = "{\"key\": \"value\", \"other\": \"thing\"}" // expect: StringShouldBeRawString
