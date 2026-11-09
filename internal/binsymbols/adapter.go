package binsymbols

// ResolverClass mirrors typeinfer.binarySymbolClass shape. Exported so
// the wiring layer can assemble the function shim without depending on
// typeinfer-internal types. The field set must stay structurally
// identical to typeinfer.binarySymbolClass.
type ResolverClass struct {
	Name       string
	FQN        string
	Kind       string
	Supertypes []string
	IsAbstract bool
	IsSealed   bool
}
