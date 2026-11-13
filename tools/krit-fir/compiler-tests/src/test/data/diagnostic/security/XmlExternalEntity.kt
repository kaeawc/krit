// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 24, 28, 32, 37, 42, 47, 52, 58, 64, 70, 76, 82, 90, 95, 102
// Positives that match Go: a JAXP factory created in a callable that parses XML and never hardens it.
package test.xmlexternalentity

import java.io.File
import java.io.InputStream
import javax.xml.XMLConstants
import javax.xml.parsers.DocumentBuilderFactory
import javax.xml.parsers.SAXParserFactory
import javax.xml.stream.XMLInputFactory
import javax.xml.transform.TransformerFactory
import javax.xml.transform.stream.StreamResult
import javax.xml.transform.stream.StreamSource
import javax.xml.validation.SchemaFactory
import org.xml.sax.helpers.DefaultHandler

fun documentBuilder(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    factory.newDocumentBuilder().parse(input)
}

fun chained(input: InputStream) {
    <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>.newDocumentBuilder().parse(input)
}

fun saxParser(input: InputStream) {
    <!XmlExternalEntity!>SAXParserFactory.newInstance()<!>.newSAXParser().parse(input, DefaultHandler())
}

fun streamReader(input: InputStream) {
    val factory = <!XmlExternalEntity!>XMLInputFactory.newFactory()<!>
    factory.createXMLStreamReader(input)
}

fun eventReader(input: InputStream) {
    val factory = <!XmlExternalEntity!>XMLInputFactory.newInstance()<!>
    factory.createXMLEventReader(input)
}

fun transform(input: InputStream) {
    val factory = <!XmlExternalEntity!>TransformerFactory.newInstance()<!>
    factory.newTransformer().transform(StreamSource(input), StreamResult(System.out))
}

fun schema(file: File) {
    val factory = <!XmlExternalEntity!>SchemaFactory.newInstance(XMLConstants.W3C_XML_SCHEMA_NS_URI)<!>
    factory.newSchema(file)
}

fun multiLine(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory<!>
        .newInstance()
    factory.newDocumentBuilder().parse(input)
}

fun lambdaParse(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    val action = { factory.newDocumentBuilder().parse(input) }
    action()
}

fun unrelatedSetting(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    factory.setFeature("http://xml.org/sax/features/namespaces", true)
    factory.newDocumentBuilder().parse(input)
}

fun wrongValue(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", false)
    factory.newDocumentBuilder().parse(input)
}

fun onlyOneProperty(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    factory.isXIncludeAware = false
    factory.newDocumentBuilder().parse(input)
}

fun secureProcessingOnParserFactory(input: InputStream) {
    val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    factory.setFeature(XMLConstants.FEATURE_SECURE_PROCESSING, true)
    factory.newDocumentBuilder().parse(input)
}

fun hardeningBeforeCreation(input: InputStream) {
    val first = DocumentBuilderFactory.newInstance()
    first.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
    val second = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>
    second.newDocumentBuilder().parse(input)
}

class PropertyFactory {
    private val factory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>

    fun load(input: InputStream) {
        factory.newDocumentBuilder().parse(input)
    }
}

val topLevelFactory = <!XmlExternalEntity!>DocumentBuilderFactory.newInstance()<!>

fun loadTopLevel(input: InputStream) {
    topLevelFactory.newDocumentBuilder().parse(input)
}
