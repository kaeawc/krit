package test

import javax.xml.parsers.DocumentBuilderFactory

class XmlExternalEntityFixture {
    fun load(input: java.io.InputStream) {
        val factory = DocumentBuilderFactory.newInstance()
        factory.newDocumentBuilder().parse(input)
    }
}
