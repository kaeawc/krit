package test

import javax.xml.parsers.DocumentBuilderFactory
import javax.xml.stream.XMLInputFactory

class XmlExternalEntitySafeFixture {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
        factory.newDocumentBuilder().parse(input)

        val streamFactory = XMLInputFactory.newInstance()
        streamFactory.setProperty(XMLInputFactory.SUPPORT_DTD, false)
    }

    fun loadMultiLine(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.setFeature(
            "http://apache.org/xml/features/disallow-doctype-decl",
            true,
        )
        factory.newDocumentBuilder().parse(input)
    }

    fun loadWithProperties(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.isXIncludeAware = false
        factory.isExpandEntityReferences = false
        factory.newDocumentBuilder().parse(input)
    }
}
