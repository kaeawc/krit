// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24
// Hardening forms where resolution and Go's name matching disagree.
package test.xmlexternalentityhardeningforms

import java.io.InputStream
import javax.xml.parsers.DocumentBuilderFactory

class FakeFactory {
    fun setFeature(name: String, value: Boolean) {}
}

fun lookalikeSetter(input: InputStream) {
    // Go takes any call named setFeature with a hardening key as hardening, so it misses this report; the
    // setter belongs to a project class, so the factory stays unhardened.
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    FakeFactory().setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
    factory.newDocumentBuilder().parse(input)
}

fun setterCalls(input: InputStream) {
    // Go reads only the property assignments (`factory.isXIncludeAware = false`), so it reports this factory
    // although both setters turn the same flags off.
    val factory = DocumentBuilderFactory.newInstance()
    factory.setXIncludeAware(false)
    factory.setExpandEntityReferences(false)
    factory.newDocumentBuilder().parse(input)
}
