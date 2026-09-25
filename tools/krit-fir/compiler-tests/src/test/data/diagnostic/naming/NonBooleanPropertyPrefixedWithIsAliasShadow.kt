// RENDER_DIAGNOSTICS_FULL_TEXT
// True positive Go misses: a same-package type alias named Boolean takes
// priority over the default import of kotlin.Boolean.
package test

typealias Boolean = String

// Go misses this: the declared type text is `Boolean`, but it expands to
// String through the alias above, so the property is not a Boolean.
<!NonBooleanPropertyPrefixedWithIs!>val<!> isAliasShadowed: Boolean = "shadowed"
