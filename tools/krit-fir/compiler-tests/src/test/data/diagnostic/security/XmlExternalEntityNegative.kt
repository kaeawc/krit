// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives that match Go: hardened factories, factories that never parse, and hardening forms.
package test.xmlexternalentitynegative

import java.io.InputStream
import javax.xml.XMLConstants
import javax.xml.parsers.DocumentBuilderFactory
import javax.xml.parsers.SAXParserFactory
import javax.xml.stream.XMLInputFactory
import javax.xml.transform.TransformerFactory
import javax.xml.transform.dom.DOMSource
import javax.xml.transform.stream.StreamResult
import javax.xml.validation.SchemaFactory
import org.xml.sax.helpers.DefaultHandler

fun disallowDoctype(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
    factory.newDocumentBuilder().parse(input)
}

fun multiLineFeature(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.setFeature(
        "http://apache.org/xml/features/disallow-doctype-decl",
        true,
    )
    factory.newDocumentBuilder().parse(input)
}

fun externalEntitiesOff(input: InputStream) {
    val factory = SAXParserFactory.newInstance()
    factory.setFeature("http://xml.org/sax/features/external-general-entities", false)
    factory.newSAXParser().parse(input, DefaultHandler())
}

fun externalParameterEntitiesOff(input: InputStream) {
    val factory = SAXParserFactory.newInstance()
    factory.setFeature("http://xml.org/sax/features/external-parameter-entities", false)
    factory.newSAXParser().parse(input, DefaultHandler())
}

fun loadExternalDtdOff(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.setFeature("http://apache.org/xml/features/nonvalidating/load-external-dtd", false)
    factory.newDocumentBuilder().parse(input)
}

fun includeAndExpandOff(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.isXIncludeAware = false
    factory.isExpandEntityReferences = false
    factory.newDocumentBuilder().parse(input)
}

fun supportDtdOff(input: InputStream) {
    val factory = XMLInputFactory.newInstance()
    factory.setProperty(XMLInputFactory.SUPPORT_DTD, false)
    factory.createXMLStreamReader(input)
}

fun externalEntitiesPropertyOff(input: InputStream) {
    val factory = XMLInputFactory.newFactory()
    factory.setProperty("javax.xml.stream.isSupportingExternalEntities", false)
    factory.createXMLEventReader(input)
}

fun accessExternalDtd(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.setAttribute(XMLConstants.ACCESS_EXTERNAL_DTD, "")
    factory.newDocumentBuilder().parse(input)
}

fun accessExternalSchema(input: InputStream) {
    val factory = SchemaFactory.newInstance(XMLConstants.W3C_XML_SCHEMA_NS_URI)
    factory.setProperty("http://javax.xml.XMLConstants/property/accessExternalSchema", "")
    factory.newSchema(javax.xml.transform.stream.StreamSource(input))
}

fun secureProcessingOnTransformer(input: InputStream) {
    val factory = TransformerFactory.newInstance()
    factory.setFeature(XMLConstants.FEATURE_SECURE_PROCESSING, true)
    factory.newTransformer().transform(javax.xml.transform.stream.StreamSource(input), StreamResult(System.out))
}

fun hardeningInLambda(input: InputStream) {
    val factory = DocumentBuilderFactory.newInstance()
    factory.apply { setFeature("http://apache.org/xml/features/disallow-doctype-decl", true) }
    factory.newDocumentBuilder().parse(input)
}

fun hardeningOnChain(input: InputStream) {
    DocumentBuilderFactory.newInstance().apply {
        setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
    }.newDocumentBuilder().parse(input)
}

fun noParse() {
    val factory = DocumentBuilderFactory.newInstance()
    factory.newDocumentBuilder().newDocument()
}

fun transformDom(source: DOMSource) {
    val factory = TransformerFactory.newInstance()
    factory.newTransformer().transform(source, StreamResult(System.out))
}

fun parseOnOtherType(text: String): Int {
    val factory = DocumentBuilderFactory.newInstance()
    factory.newDocumentBuilder().newDocument()
    return Integer.parseInt(text)
}

class SeparateCallables {
    fun create() = DocumentBuilderFactory.newInstance()

    fun load(input: InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
        factory.newDocumentBuilder().parse(input)
    }
}
