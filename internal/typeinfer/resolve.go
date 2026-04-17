package typeinfer

// ---------------------------------------------------------------------------
// Type resolution helpers
// ---------------------------------------------------------------------------

// makeResolvedType creates a ResolvedType from a simple name.
func (r *defaultResolver) makeResolvedType(name string, it *ImportTable, nullable bool) *ResolvedType {
	fqn := ""
	if it != nil {
		fqn = it.Resolve(name)
	}
	if fqn == "" {
		if f, ok := PrimitiveTypes[name]; ok {
			fqn = f
		} else if f, ok := KotlinStdlibTypes[name]; ok {
			fqn = f
		}
	}

	kind := TypeClass
	if _, ok := PrimitiveTypes[name]; ok {
		kind = TypePrimitive
	}
	if name == "Unit" {
		kind = TypeUnit
	}
	if name == "Nothing" {
		kind = TypeNothing
	}

	return &ResolvedType{
		Name:     name,
		FQN:      fqn,
		Kind:     kind,
		Nullable: nullable,
	}
}

// applyStdlibReturnType creates a ResolvedType from a stdlib method match,
// propagating generic type arguments from the receiver when applicable.
func (r *defaultResolver) applyStdlibReturnType(m *StdlibMethod, receiverType *ResolvedType) *ResolvedType {
	result := &ResolvedType{
		Name:     m.ReturnType.Name,
		FQN:      m.ReturnType.FQN,
		Kind:     m.ReturnType.Kind,
		Nullable: m.Nullable,
	}
	// Propagate generic type args from receiver
	if m.ReturnTypeArgIndex >= 0 && receiverType != nil && len(receiverType.TypeArgs) > m.ReturnTypeArgIndex {
		arg := receiverType.TypeArgs[m.ReturnTypeArgIndex]
		result.Name = arg.Name
		result.FQN = arg.FQN
		result.Kind = arg.Kind
		if m.Nullable {
			result.Nullable = true
		}
	}
	return result
}

