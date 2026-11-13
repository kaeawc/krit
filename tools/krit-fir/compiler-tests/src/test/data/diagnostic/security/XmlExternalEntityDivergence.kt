// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Shapes where resolution and Go's name matching disagree.
package test.xmlexternalentitydivergence

import java.io.InputStream
import javax.xml.parsers.DocumentBuilderFactory as Dbf
import javax.xml.parsers.SAXParserFactory
import org.xml.sax.helpers.DefaultHandler

typealias SaxFactory = SAXParserFactory

// Go misses the aliased factory: it needs the receiver spelled DocumentBuilderFactory.
fun importAlias(input: InputStream) {
    val factory = <!XmlExternalEntity!>Dbf.newInstance()<!>
    factory.newDocumentBuilder().parse(input)
}

// Go misses the type alias of SAXParserFactory for the same reason.
fun typeAlias(input: InputStream) {
    val factory = <!XmlExternalEntity!>SaxFactory.newInstance()<!>
    factory.newSAXParser().parse(input, DefaultHandler())
}
