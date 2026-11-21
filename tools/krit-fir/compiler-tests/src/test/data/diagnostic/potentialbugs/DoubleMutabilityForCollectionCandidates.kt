// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
package test

class CandidateOptions {
    var publicItems: MutableList<String> = mutableListOf()
    internal var shared: MutableSet<String> = mutableSetOf()
    protected var inherited: MutableMap<String, Int> = mutableMapOf()
    private var nullable: MutableList<String>? = null
    private var written = mutableListOf<String>()
    <!DoubleMutabilityForCollection!>private<!> var candidate = mutableListOf<String>()

    fun reset() { written = mutableListOf() }

    fun run() {
        var local = mutableListOf<String>()
        local = mutableListOf()
    }
}

// Default narrowing also reports private constructor properties in FIR;
// Go only visits property_declaration nodes, not constructor parameters.
// Compatibility mode excludes both constructor parameters (covered by file tests).
class ConstructorOptions(
    var publicItems: MutableList<String>,
    <!DoubleMutabilityForCollection!>private<!> var privateItems: MutableList<String>,
)
