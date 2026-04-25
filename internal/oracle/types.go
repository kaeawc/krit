package oracle

// OracleData is the top-level JSON structure produced by krit-types.
type OracleData struct {
	Version       int                     `json:"version"`
	KotlinVersion string                  `json:"kotlinVersion"`
	Files         map[string]*OracleFile  `json:"files"`        // source file path → declarations
	Dependencies  map[string]*OracleClass `json:"dependencies"` // FQN → class info from JARs
}

// OracleFile holds declarations extracted from a single source file.
type OracleFile struct {
	Package      string                     `json:"package"`
	Declarations []*OracleClass             `json:"declarations"`
	Expressions  map[string]*ExpressionType `json:"expressions,omitempty"` // "line:col" → type
	Diagnostics  []*OracleDiagnostic        `json:"diagnostics,omitempty"` // compiler diagnostics
}

// OracleDiagnostic holds a compiler diagnostic emitted by the Kotlin Analysis API.
type OracleDiagnostic struct {
	FactoryName string `json:"factoryName"` // e.g. "UNREACHABLE_CODE", "USELESS_ELVIS"
	Severity    string `json:"severity"`    // "ERROR", "WARNING", "INFO"
	Message     string `json:"message"`
	Line        int    `json:"line"` // 1-based
	Col         int    `json:"col"`  // 1-based
}

// ExpressionType holds a compiler-resolved type for an expression at a source position.
type ExpressionType struct {
	Type               string   `json:"type"` // FQN like "kotlin.String"
	Nullable           bool     `json:"nullable"`
	CallTarget         string   `json:"callTarget,omitempty"`         // FQN of resolved function or lexical fallback
	CallTargetResolved bool     `json:"callTargetResolved,omitempty"` // true when callTarget came from KAA resolution
	CallTargetSuspend  bool     `json:"callTargetSuspend,omitempty"`  // true when the resolved callable symbol is suspend
	Annotations        []string `json:"annotations,omitempty"`        // FQNs of annotations on the resolved symbol
}

// OracleClass describes a class, interface, object, or enum.
type OracleClass struct {
	FQN            string          `json:"fqn"`
	Kind           string          `json:"kind"` // class, interface, object, enum, sealed class, sealed interface
	Supertypes     []string        `json:"supertypes"`
	IsSealed       bool            `json:"isSealed"`
	IsData         bool            `json:"isData"`
	IsOpen         bool            `json:"isOpen"`
	IsAbstract     bool            `json:"isAbstract"`
	Visibility     string          `json:"visibility"`
	TypeParameters []string        `json:"typeParameters,omitempty"`
	Members        []*OracleMember `json:"members,omitempty"`
	Annotations    []string        `json:"annotations,omitempty"` // FQNs
}

// OracleMember describes a function or property within a class.
type OracleMember struct {
	Name        string         `json:"name"`
	Kind        string         `json:"kind"` // function, property
	ReturnType  string         `json:"returnType"`
	Nullable    bool           `json:"nullable"`
	Visibility  string         `json:"visibility"`
	IsOverride  bool           `json:"isOverride,omitempty"`
	IsAbstract  bool           `json:"isAbstract,omitempty"`
	Params      []*OracleParam `json:"params,omitempty"`
	Annotations []string       `json:"annotations,omitempty"` // FQNs
}

// OracleParam describes a function parameter.
type OracleParam struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}
