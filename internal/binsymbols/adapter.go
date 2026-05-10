package binsymbols

// Adapter converts a Reader into the function shape that
// typeinfer.SetBinSymbolReader accepts. Use it at the wiring layer:
//
//	resolver.SetBinSymbolReader(binsymbols.AdaptForResolver(reader))
//
// The adapter mirrors only the fields typeinfer's local
// binarySymbolClass exposes; member detail (Members, IsSealed
// nuances) is intentionally elided since the resolver's ClassHierarchy
// surface does not yet propagate Members from binary readers.
//
// AdaptForResolver returns a function whose return type is the small
// struct typeinfer expects. The struct is anonymous-equivalent — its
// fields are validated at the call boundary by Go's structural
// matching against the typeinfer interface.
func AdaptForResolver(r Reader) func(fqn string) *ResolverClass {
	if r == nil {
		return func(string) *ResolverClass { return nil }
	}
	return func(fqn string) *ResolverClass {
		c := r.LookupClass(fqn)
		if c == nil {
			return nil
		}
		return &ResolverClass{
			Name:       c.Name,
			FQN:        c.FQN,
			Kind:       c.Kind,
			Supertypes: c.Supertypes,
			IsAbstract: c.IsAbstract,
			IsSealed:   c.IsSealed,
		}
	}
}

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
