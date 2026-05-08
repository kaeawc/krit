package evidence

import "github.com/kaeawc/krit/internal/scanner"

// Call is a structural view over a call_expression / method_invocation node.
// Build via Evidence.Call. Receiver is the syntactic receiver name (or
// dotted chain) — empty for unqualified calls. Use ResolveOwner to get the
// resolved FQN of the receiver type.
type Call struct {
	Idx      uint32 // FlatNode index of the call site
	Callee   string // simple callee name; "" if it could not be extracted
	Receiver string // syntactic receiver (simple identifier or dotted chain); "" if unqualified

	// ReceiverIdx is the FlatNode index of the receiver expression — the
	// part before the trailing navigation_suffix. Zero when the call has
	// no receiver. Useful for chained calls: pass it to Evidence.Call to
	// descend into the inner call (e.g. unwrap Runtime.getRuntime() inside
	// Runtime.getRuntime().exec(...)).
	ReceiverIdx uint32
}

// Call wraps the call site at the given flat index. Returns nil if the
// node is not a call_expression / method_invocation.
func (e *Evidence) Call(idx uint32) *Call {
	if e == nil || e.file == nil || idx == 0 {
		return nil
	}
	switch e.file.FlatType(idx) {
	case "call_expression":
		return kotlinCall(e.file, idx)
	case "method_invocation":
		return javaCall(e.file, idx)
	case "object_creation_expression":
		return javaObjectCreation(e.file, idx)
	}
	return nil
}

// javaObjectCreation extracts the constructed type name from a Java
// object_creation_expression (`new Foo(args)`). The constructed type
// is exposed as Callee with no Receiver, mirroring the unqualified
// Kotlin constructor-style call (`Foo(args)`) so ResolveCalleeFQN
// works uniformly.
//
// Java tree-sitter shape:
//
//	object_creation_expression
//	  "new"
//	  type_identifier | scoped_type_identifier | generic_type
//	  argument_list
func javaObjectCreation(file *scanner.File, idx uint32) *Call {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "type_identifier":
			return &Call{Idx: idx, Callee: file.FlatNodeString(child, nil)}
		case "scoped_type_identifier", "generic_type":
			// Take the trailing identifier as the simple name.
			last := ""
			var walk func(uint32)
			walk = func(n uint32) {
				if file.FlatType(n) == "type_identifier" {
					last = file.FlatNodeString(n, nil)
				}
				for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
					if file.FlatIsNamed(c) {
						walk(c)
					}
				}
			}
			walk(child)
			return &Call{Idx: idx, Callee: last}
		}
	}
	return &Call{Idx: idx}
}

// kotlinCall extracts callee + receiver from a Kotlin call_expression.
// Tree-sitter shape:
//
//	call_expression
//	  navigation_expression
//	    <receiver expression>            // e.g. simple_identifier "db"
//	    navigation_suffix
//	      simple_identifier "rawQuery"   // callee
//	  call_suffix
//
// Or, for unqualified calls:
//
//	call_expression
//	  simple_identifier "foo"
//	  call_suffix
func kotlinCall(file *scanner.File, idx uint32) *Call {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "simple_identifier":
			return &Call{Idx: idx, Callee: file.FlatNodeString(child, nil)}
		case "navigation_expression":
			callee, receiver, recvIdx := navigationParts(file, child)
			return &Call{Idx: idx, Callee: callee, Receiver: receiver, ReceiverIdx: recvIdx}
		}
	}
	return &Call{Idx: idx}
}

// navigationParts decomposes a navigation_expression into (callee,
// receiver text, receiver flat index) in one pass. The named children
// are the receiver expression followed by exactly one navigation_suffix
// holding the selector identifier.
//
// Returns:
//   - callee: the trailing simple_identifier inside navigation_suffix
//   - receiver: dotted text of the receiver portion ("db", "Foo.Bar.baz")
//     — empty when the receiver is a call_expression (use recvIdx +
//     Evidence.Call to descend instead).
//   - recvIdx: the flat node index of the receiver child (any kind).
func navigationParts(file *scanner.File, navExpr uint32) (callee, receiver string, recvIdx uint32) {
	for child := file.FlatFirstChild(navExpr); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "navigation_suffix":
			if id, ok := file.FlatFindChild(child, "simple_identifier"); ok {
				callee = file.FlatNodeString(id, nil)
			}
		case "simple_identifier":
			if recvIdx == 0 {
				receiver = file.FlatNodeString(child, nil)
				recvIdx = child
			}
		case "navigation_expression":
			if recvIdx == 0 {
				receiver = navigationChainText(file, child)
				recvIdx = child
			}
		default:
			if recvIdx == 0 {
				recvIdx = child
			}
		}
	}
	return callee, receiver, recvIdx
}

// navigationChainText flattens a nested navigation_expression to dotted
// text, joining each leg's identifier.
func navigationChainText(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	out := ""
	var walk func(uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "simple_identifier":
			text := file.FlatNodeString(n, nil)
			if out == "" {
				out = text
			} else {
				out += "." + text
			}
			return
		}
		for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			t := file.FlatType(child)
			if t == "navigation_expression" || t == "navigation_suffix" || t == "simple_identifier" {
				walk(child)
			}
		}
	}
	walk(idx)
	return out
}

// javaCall extracts callee + receiver from a Java method_invocation.
//
// Java tree-sitter shape comes in two flavors:
//
//   - Simple chain (Foo.bar.baz()): a sequence of `identifier` children
//     before `argument_list`; the last is the callee, the prefix is the
//     receiver chain.
//   - Method-on-call (foo().bar()): a `method_invocation` (or
//     `field_access`) child followed by `identifier` (the callee). In
//     this case the receiver is the inner expression — record its flat
//     index in ReceiverIdx so callers can recursively unwrap.
func javaCall(file *scanner.File, idx uint32) *Call {
	var ids []string
	var recvIdx uint32
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t == "argument_list" {
			break
		}
		switch t {
		case "identifier":
			ids = append(ids, file.FlatNodeString(child, nil))
		case "method_invocation", "field_access", "object_creation_expression", "parenthesized_expression", "cast_expression":
			if recvIdx == 0 {
				recvIdx = child
			}
		}
	}
	if len(ids) == 0 {
		return &Call{Idx: idx, ReceiverIdx: recvIdx}
	}
	c := &Call{Idx: idx, Callee: ids[len(ids)-1], ReceiverIdx: recvIdx}
	if len(ids) > 1 {
		c.Receiver = ids[0]
		for _, p := range ids[1 : len(ids)-1] {
			c.Receiver += "." + p
		}
	}
	return c
}
