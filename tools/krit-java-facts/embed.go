// Package javafactshelper provides the Java source used to collect Java
// semantic facts.
package javafactshelper

import _ "embed"

// Source is the embedded source for the Java semantic facts helper.
//
//go:embed src/main/java/dev/jasonpearson/krit/javafacts/Main.java
var Source []byte
