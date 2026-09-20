package typeinfer

// ---------------------------------------------------------------------------
// Type resolution helpers
// ---------------------------------------------------------------------------

// makeResolvedType creates a ResolvedType from a simple name.
func (r *defaultResolver) makeResolvedType(name string, it *ImportTable, nullable bool) *ResolvedType {
	fqn := ""
	resolved := false
	if it != nil {
		fqn = it.Resolve(name)
		resolved = fqn != ""
	}
	if fqn == "" {
		if r != nil {
			if info, ok := r.classes[name]; ok && info != nil && info.FQN != "" {
				resolved = true
			}
		}
		if f, ok := PrimitiveTypes[name]; !resolved && ok {
			fqn = f
			resolved = true
		} else if f, ok := KotlinStdlibTypes[name]; !resolved && ok {
			fqn = f
			resolved = true
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
	if fqn == "" {
		fqn = name
	}

	return &ResolvedType{
		Name:     name,
		FQN:      fqn,
		Kind:     kind,
		Nullable: nullable,
		Resolved: resolved,
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
		Resolved: m.ReturnType.Resolved,
	}
	// Propagate generic type args from receiver
	if m.ReturnTypeArgIndex >= 0 && receiverType != nil && len(receiverType.TypeArgs) > m.ReturnTypeArgIndex {
		arg := receiverType.TypeArgs[m.ReturnTypeArgIndex]
		result.Name = arg.Name
		result.FQN = arg.FQN
		result.Kind = arg.Kind
		result.Resolved = arg.Resolved
		if m.Nullable {
			result.Nullable = true
		}
	}
	return result
}
