package kotlin_settings

// The assignment compiler plugin rewrites `x = value` on a type annotated
// with this into `x.assign(value)`.
annotation class SupportsAssignment
