package test;

import javax.xml.parsers.DocumentBuilderFactory;

class XmlExternalEntityJavaSafeFixture {
    void load(java.io.InputStream input) throws Exception {
        DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
        factory.newDocumentBuilder().parse(input);
    }
}
