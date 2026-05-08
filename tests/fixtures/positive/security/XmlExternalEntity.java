package test;

import javax.xml.parsers.DocumentBuilderFactory;

class XmlExternalEntityJavaFixture {
    void load(java.io.InputStream input) throws Exception {
        DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
        factory.newDocumentBuilder().parse(input);
    }
}
