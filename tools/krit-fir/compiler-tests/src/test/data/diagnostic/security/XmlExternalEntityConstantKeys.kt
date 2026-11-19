// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 26, 32, 40
// A project constant that holds a setting's string names that setting, as the literal does.
package test.xmlexternalentityconstantkeys

import java.io.InputStream
import javax.xml.parsers.DocumentBuilderFactory
import javax.xml.stream.XMLInputFactory

const val DISALLOW_DOCTYPE = "http://apache.org/xml/features/disallow-doctype-decl"
const val ENABLE_DOCTYPE_CHECK = "http://xml.org/sax/features/namespaces"

object XmlKeys {
    const val SUPPORT_DTD_KEY = "javax.xml.stream.supportDTD"
}

// Go reads only the constant's name, so it reports this hardened factory.
fun constantFeature(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.setFeature(DISALLOW_DOCTYPE, true)
    factory.newDocumentBuilder().parse(input)
}

// Go reads only the constant's name, so it reports this hardened factory.
fun qualifiedConstantProperty(input: InputStream) {
    val factory = XMLInputFactory.newInstance()
    factory.setProperty(XmlKeys.SUPPORT_DTD_KEY, false)
    factory.createXMLStreamReader(input)
}

fun unrelatedConstant(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    factory.setFeature(ENABLE_DOCTYPE_CHECK, true)
    factory.newDocumentBuilder().parse(input)
}

// The companion's property initializer is searched together with the class around it, where the parse happens.
class CompanionFactory {
    companion object {
        private val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    }

    fun load(input: InputStream) = factory.newDocumentBuilder().parse(input)
}
