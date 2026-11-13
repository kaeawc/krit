// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19
// A project class named like a JAXP factory is not one.
package test.xmlexternalentitylookalike

import java.io.InputStream

class Parser {
    fun parse(input: InputStream) {}
}

object DocumentBuilderFactory {
    fun newInstance(): DocumentBuilderFactory = this

    fun newParser(): Parser = Parser()
}

fun lookalike(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.newParser().parse(input)
}
