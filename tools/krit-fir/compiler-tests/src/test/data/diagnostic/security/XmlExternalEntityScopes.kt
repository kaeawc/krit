// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 27, 57, 67
// The callable that creates the factory is the scope searched for parsing and hardening.
package test.xmlexternalentityscopes

import java.io.InputStream
import javax.xml.parsers.DocumentBuilder
import javax.xml.parsers.DocumentBuilderFactory

// The nearest named function is the scope, so a parse in the outer function does not count.
fun localFunctionCreates(input: InputStream) {
    fun create() = DocumentBuilderFactory.newInstance()
    create().newDocumentBuilder().newDocument()
}

// A lambda belongs to the function around it.
fun lambdaCreates(input: InputStream) {
    val create = { <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!> }
    create().newDocumentBuilder().parse(input)
}

// An init block has no function around it, so the class is the scope.
class InitBlock {
    private val builder: DocumentBuilder

    init {
        val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
        builder = factory.newDocumentBuilder()
    }

    fun load(input: InputStream) = builder.parse(input)
}

class HardenedInAnotherMember {
    private val builder: DocumentBuilder

    init {
        val factory = DocumentBuilderFactory.newInstance()
        builder = factory.newDocumentBuilder()
    }

    fun harden(factory: DocumentBuilderFactory) {
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
    }

    fun load(input: InputStream) = builder.parse(input)
}

class NoParseInClass {
    private val factory = DocumentBuilderFactory.newInstance()

    fun create(): DocumentBuilder = factory.newDocumentBuilder()
}

fun localClassMember(input: InputStream) {
    class Loader {
        private val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>

        fun load() = factory.newDocumentBuilder().parse(input)
    }
    Loader().load()
}

fun anonymousObjectMember(input: InputStream) {
    val loader = object {
        fun load() {
            val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
            factory.newDocumentBuilder().parse(input)
        }
    }
    loader.load()
}
