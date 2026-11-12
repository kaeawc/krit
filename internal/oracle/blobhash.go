package oracle

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"sort"

	"github.com/kaeawc/krit/internal/hashutil"
)

// FactSchemaVersion identifies the canonical per-file oracle fact stream.
// Bump it whenever BlobHash's stable projection changes.
const FactSchemaVersion = 1

// BlobHash returns a deterministic hash of the facts emitted for one source
// file. Declaration records include class identity/kind, supertypes, flags,
// visibility, type parameters, annotations and source position, plus member
// name/kind/return nullability/visibility/flags/parameters/annotations/source
// position. Expression records include type/nullability, call-target fields,
// and sorted annotations. Diagnostic records include factory, severity,
// message, line and column. Every collection whose semantic order is
// irrelevant is sorted before framing; parameter and type-parameter order is
// preserved because it is part of the declaration signature.
func BlobHash(f *File) string {
	return blobHashWithSchemaVersion(f, FactSchemaVersion)
}

func blobHashWithSchemaVersion(f *File, schemaVersion int) string {
	h := hashutil.Hasher().New()
	writeBlobToken(h.Write, "fact-schema")
	writeBlobInt(h.Write, schemaVersion)

	var declarations []*Class
	var expressions map[string]*ExpressionType
	var diagnostics []*Diagnostic
	if f != nil {
		declarations = append([]*Class(nil), f.Declarations...)
		expressions = f.Expressions
		diagnostics = append([]*Diagnostic(nil), f.Diagnostics...)
	}
	// Encode each declaration once. The sort key is the framed FQN and kind;
	// ties fall back to the canonical bytes instead of storing a second copy.
	type declarationRecord struct {
		key       []byte
		canonical []byte
	}
	declarationRecords := make([]declarationRecord, len(declarations))
	for i, declaration := range declarations {
		canonical := canonicalDeclaration(declaration)
		declarationRecords[i] = declarationRecord{key: declarationSortKeyFor(declaration), canonical: canonical}
	}
	sort.Slice(declarationRecords, func(i, j int) bool {
		a, b := declarationRecords[i], declarationRecords[j]
		if c := bytes.Compare(a.key, b.key); c != 0 {
			return c < 0
		}
		// Same order as comparing the length-framed canonical bytes.
		if len(a.canonical) != len(b.canonical) {
			return len(a.canonical) < len(b.canonical)
		}
		return bytes.Compare(a.canonical, b.canonical) < 0
	})
	writeBlobToken(h.Write, "declarations")
	writeBlobInt(h.Write, len(declarationRecords))
	for _, record := range declarationRecords {
		writeBlobBytes(h.Write, record.canonical)
	}

	type expressionRecord struct {
		key  string
		fact *ExpressionType
	}
	expressionRecords := make([]expressionRecord, 0, len(expressions))
	for key, fact := range expressions {
		expressionRecords = append(expressionRecords, expressionRecord{key: key, fact: fact})
	}
	sort.Slice(expressionRecords, func(i, j int) bool {
		a, b := expressionRecords[i], expressionRecords[j]
		as, ae := expressionBounds(a.fact)
		bs, be := expressionBounds(b.fact)
		if as != bs {
			return as < bs
		}
		if ae != be {
			return ae < be
		}
		return a.key < b.key
	})
	writeBlobToken(h.Write, "expressions")
	writeBlobInt(h.Write, len(expressionRecords))
	for _, record := range expressionRecords {
		writeBlobBytes(h.Write, canonicalExpression(record.fact))
	}

	sort.Slice(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a == nil || b == nil {
			return a == nil && b != nil
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.FactoryName != b.FactoryName {
			return a.FactoryName < b.FactoryName
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.Message < b.Message
	})
	writeBlobToken(h.Write, "diagnostics")
	writeBlobInt(h.Write, len(diagnostics))
	for _, diagnostic := range diagnostics {
		writeBlobBytes(h.Write, canonicalDiagnostic(diagnostic))
	}
	return hex.EncodeToString(h.Sum(nil))
}

type blobWriter func([]byte) (int, error)

func writeBlobToken(write blobWriter, value string) { writeBlobBytes(write, []byte(value)) }

func writeBlobBytes(write blobWriter, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = write(length[:])
	_, _ = write(value)
}

func writeBlobInt(write blobWriter, value int) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(int64(value)))
	_, _ = write(encoded[:])
}

func writeBlobBool(write blobWriter, value bool) {
	if value {
		_, _ = write([]byte{1})
		return
	}
	_, _ = write([]byte{0})
}

func writeBlobStrings(write blobWriter, values []string, sortValues bool) {
	values = append([]string(nil), values...)
	if sortValues {
		sort.Strings(values)
	}
	writeBlobInt(write, len(values))
	for _, value := range values {
		writeBlobToken(write, value)
	}
}

func declarationSortKeyFor(c *Class) []byte {
	if c == nil {
		return nil
	}
	out := bytes.NewBuffer(make([]byte, 0, len(c.FQN)+len(c.Kind)+16))
	writeBlobToken(out.Write, c.FQN)
	writeBlobToken(out.Write, c.Kind)
	return out.Bytes()
}

func canonicalDeclaration(c *Class) []byte {
	var out bytes.Buffer
	if c == nil {
		return out.Bytes()
	}
	writeBlobToken(out.Write, c.FQN)
	writeBlobToken(out.Write, c.Kind)
	writeBlobStrings(out.Write, c.Supertypes, true)
	writeBlobBool(out.Write, c.IsSealed)
	writeBlobBool(out.Write, c.IsData)
	writeBlobBool(out.Write, c.IsOpen)
	writeBlobBool(out.Write, c.IsAbstract)
	writeBlobToken(out.Write, c.Visibility)
	writeBlobStrings(out.Write, c.TypeParameters, false)
	writeBlobStrings(out.Write, c.Annotations, true)
	writeBlobInt(out.Write, c.Line)
	writeBlobInt(out.Write, c.Column)

	members := make([][]byte, len(c.Members))
	for i, member := range c.Members {
		members[i] = canonicalMember(member)
	}
	sort.Slice(members, func(i, j int) bool { return bytes.Compare(members[i], members[j]) < 0 })
	writeBlobInt(out.Write, len(members))
	for _, member := range members {
		writeBlobBytes(out.Write, member)
	}
	return out.Bytes()
}

func canonicalMember(m *Member) []byte {
	var out bytes.Buffer
	if m == nil {
		return out.Bytes()
	}
	writeBlobToken(out.Write, m.Name)
	writeBlobToken(out.Write, m.Kind)
	writeBlobToken(out.Write, m.ReturnType)
	writeBlobBool(out.Write, m.Nullable)
	writeBlobToken(out.Write, m.Visibility)
	writeBlobBool(out.Write, m.IsOverride)
	writeBlobBool(out.Write, m.IsAbstract)
	writeBlobInt(out.Write, len(m.Params))
	for _, param := range m.Params {
		if param == nil {
			writeBlobBytes(out.Write, nil)
			continue
		}
		var encoded bytes.Buffer
		writeBlobToken(encoded.Write, param.Name)
		writeBlobToken(encoded.Write, param.Type)
		writeBlobBool(encoded.Write, param.Nullable)
		writeBlobBytes(out.Write, encoded.Bytes())
	}
	writeBlobStrings(out.Write, m.Annotations, true)
	writeBlobInt(out.Write, m.Line)
	writeBlobInt(out.Write, m.Column)
	return out.Bytes()
}

func expressionBounds(fact *ExpressionType) (int, int) {
	if fact == nil {
		return 0, 0
	}
	return fact.StartByte, fact.EndByte
}

func canonicalExpression(fact *ExpressionType) []byte {
	var out bytes.Buffer
	if fact == nil {
		return out.Bytes()
	}
	writeBlobToken(out.Write, fact.Type)
	writeBlobBool(out.Write, fact.Nullable)
	writeBlobToken(out.Write, fact.CallTarget)
	writeBlobBool(out.Write, fact.CallTargetResolved)
	writeBlobBool(out.Write, fact.CallTargetSuspend)
	writeBlobStrings(out.Write, fact.Annotations, true)
	// Only when present, so the hash of every other expression is unchanged.
	if len(fact.SmartCastDecls) > 0 {
		writeBlobStrings(out.Write, fact.SmartCastDecls, true)
	}
	return out.Bytes()
}

func canonicalDiagnostic(diagnostic *Diagnostic) []byte {
	var out bytes.Buffer
	if diagnostic == nil {
		return out.Bytes()
	}
	writeBlobToken(out.Write, diagnostic.FactoryName)
	writeBlobToken(out.Write, diagnostic.Severity)
	writeBlobToken(out.Write, diagnostic.Message)
	writeBlobInt(out.Write, diagnostic.Line)
	writeBlobInt(out.Write, diagnostic.Col)
	// Only when present, so the hash of every other diagnostic is unchanged.
	if len(diagnostic.SmartCastDecls) > 0 {
		writeBlobStrings(out.Write, diagnostic.SmartCastDecls, true)
	}
	return out.Bytes()
}
