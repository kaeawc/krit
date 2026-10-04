// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 20
// Like Go, a class named Context in any package, and any subtype of one, counts
// as a Context: Go's grantURITypeIsContext accepts the simple name. A grant on
// such a type is reported; a same-named function on an unrelated class is not.
package test

open class Context {
    open fun grantUriPermission(toPackage: String, path: String, modeFlags: Int) {}
}

class PluginContext : Context()

class Registry {
    fun grantUriPermission(toPackage: String, path: String, modeFlags: Int) {}
}

fun onThirdPartyContext(context: Context, plugin: PluginContext, registry: Registry) {
    <!GrantAllUris!>context.grantUriPermission("com.other", "/", 1)<!>
    <!GrantAllUris!>plugin.grantUriPermission("com.other", "/", 1)<!>
    registry.grantUriPermission("com.other", "/", 1)
}
