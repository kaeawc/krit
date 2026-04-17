package scanner

// BinaryFix represents a fix that operates on binary files (images, etc.)
type BinaryFix struct {
	Type         BinaryFixType
	SourcePath   string // original file
	TargetPath   string // new file (empty = generate alongside with new extension)
	Description  string
	DeleteSource bool   // delete source file after successful conversion
	Content      []byte // file content for BinaryFixCreateFile
	HintOnly     bool   // when true, the fix is informational only (no automatic action)
	MinSdk       int    // minimum SDK for this fix to be safe (0 = no restriction)
}

// BinaryFixType enumerates the kinds of binary file operations.
type BinaryFixType int

const (
	BinaryFixConvertWebP    BinaryFixType = iota
	BinaryFixDeleteFile
	BinaryFixCreateFile
	BinaryFixMoveFile
	BinaryFixOptimizePNG
)
