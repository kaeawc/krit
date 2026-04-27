package scanner

import (
	"encoding/json"
	"fmt"
)

type findingColumnsJSON struct {
	Files          []string    `json:"files,omitempty"`
	RuleSets       []string    `json:"ruleSets,omitempty"`
	Rules          []string    `json:"rules,omitempty"`
	Messages       []string    `json:"messages,omitempty"`
	FixPool        []Fix       `json:"fixPool,omitempty"`
	BinaryFixPool  []BinaryFix `json:"binaryFixPool,omitempty"`
	FileIdx        []uint32    `json:"fileIdx,omitempty"`
	Line           []uint32    `json:"line,omitempty"`
	Col            []uint16    `json:"col,omitempty"`
	StartByte      []uint32    `json:"startByte,omitempty"`
	EndByte        []uint32    `json:"endByte,omitempty"`
	RuleSetIdx     []uint16    `json:"ruleSetIdx,omitempty"`
	RuleIdx        []uint16    `json:"ruleIdx,omitempty"`
	SeverityID     []uint8     `json:"severityID,omitempty"`
	MessageIdx     []uint32    `json:"messageIdx,omitempty"`
	Confidence     []uint8     `json:"confidence,omitempty"`
	FixStart       []uint32    `json:"fixStart,omitempty"`
	BinaryFixStart []uint32    `json:"binaryFixStart,omitempty"`
	N              int         `json:"n,omitempty"`
}

// MarshalJSON persists finding columns with a stable lowercase schema rather
// than exposing Go field names directly.
func (c FindingColumns) MarshalJSON() ([]byte, error) {
	return json.Marshal(findingColumnsJSON{
		Files:          c.Files,
		RuleSets:       c.RuleSets,
		Rules:          c.Rules,
		Messages:       c.Messages,
		FixPool:        c.FixPool,
		BinaryFixPool:  c.BinaryFixPool,
		FileIdx:        c.FileIdx,
		Line:           c.Line,
		Col:            c.Col,
		StartByte:      omitZeroUint32Column(c.StartByte),
		EndByte:        omitZeroUint32Column(c.EndByte),
		RuleSetIdx:     c.RuleSetIdx,
		RuleIdx:        c.RuleIdx,
		SeverityID:     c.SeverityID,
		MessageIdx:     c.MessageIdx,
		Confidence:     c.Confidence,
		FixStart:       c.FixStart,
		BinaryFixStart: c.BinaryFixStart,
	})
}

// UnmarshalJSON accepts the stable lowercase schema and the prior exported Go
// field-name schema written by older iterations of the cache.
func (c *FindingColumns) UnmarshalJSON(data []byte) error {
	var payload findingColumnsJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	rowCount := payload.N
	rowSlices := []int{
		len(payload.FileIdx),
		len(payload.Line),
		len(payload.Col),
		len(payload.RuleSetIdx),
		len(payload.RuleIdx),
		len(payload.SeverityID),
		len(payload.MessageIdx),
		len(payload.Confidence),
		len(payload.FixStart),
		len(payload.BinaryFixStart),
	}
	if len(payload.StartByte) > 0 {
		rowSlices = append(rowSlices, len(payload.StartByte))
	}
	if len(payload.EndByte) > 0 {
		rowSlices = append(rowSlices, len(payload.EndByte))
	}
	for _, size := range rowSlices {
		switch {
		case rowCount == 0:
			rowCount = size
		case size != rowCount:
			return fmt.Errorf("scanner: invalid FindingColumns row lengths")
		}
	}
	if err := validateFindingColumnsIndexes(&payload); err != nil {
		return err
	}

	*c = FindingColumns{
		Files:          append([]string(nil), payload.Files...),
		RuleSets:       append([]string(nil), payload.RuleSets...),
		Rules:          append([]string(nil), payload.Rules...),
		Messages:       append([]string(nil), payload.Messages...),
		FixPool:        append([]Fix(nil), payload.FixPool...),
		FileIdx:        append([]uint32(nil), payload.FileIdx...),
		Line:           append([]uint32(nil), payload.Line...),
		Col:            append([]uint16(nil), payload.Col...),
		StartByte:      normalizeOptionalUint32Column(payload.StartByte, rowCount),
		EndByte:        normalizeOptionalUint32Column(payload.EndByte, rowCount),
		RuleSetIdx:     append([]uint16(nil), payload.RuleSetIdx...),
		RuleIdx:        append([]uint16(nil), payload.RuleIdx...),
		SeverityID:     append([]uint8(nil), payload.SeverityID...),
		MessageIdx:     append([]uint32(nil), payload.MessageIdx...),
		Confidence:     append([]uint8(nil), payload.Confidence...),
		FixStart:       append([]uint32(nil), payload.FixStart...),
		BinaryFixStart: append([]uint32(nil), payload.BinaryFixStart...),
		N:              rowCount,
	}
	if len(payload.BinaryFixPool) > 0 {
		c.BinaryFixPool = make([]BinaryFix, len(payload.BinaryFixPool))
		for i, fix := range payload.BinaryFixPool {
			c.BinaryFixPool[i] = cloneBinaryFix(fix)
		}
	}
	return nil
}

func normalizeOptionalUint32Column(values []uint32, rowCount int) []uint32 {
	if len(values) == rowCount {
		return append([]uint32(nil), values...)
	}
	return make([]uint32, rowCount)
}

func omitZeroUint32Column(values []uint32) []uint32 {
	for _, value := range values {
		if value != 0 {
			return values
		}
	}
	return nil
}

func validateFindingColumnsIndexes(payload *findingColumnsJSON) error {
	for row, idx := range payload.FileIdx {
		if int(idx) >= len(payload.Files) {
			return fmt.Errorf("scanner: invalid fileIdx at row %d", row)
		}
	}
	for row, idx := range payload.RuleSetIdx {
		if int(idx) >= len(payload.RuleSets) {
			return fmt.Errorf("scanner: invalid ruleSetIdx at row %d", row)
		}
	}
	for row, idx := range payload.RuleIdx {
		if int(idx) >= len(payload.Rules) {
			return fmt.Errorf("scanner: invalid ruleIdx at row %d", row)
		}
	}
	for row, idx := range payload.MessageIdx {
		if int(idx) >= len(payload.Messages) {
			return fmt.Errorf("scanner: invalid messageIdx at row %d", row)
		}
	}
	for row, ref := range payload.FixStart {
		if ref > uint32(len(payload.FixPool)) {
			return fmt.Errorf("scanner: invalid fixStart at row %d", row)
		}
	}
	for row, ref := range payload.BinaryFixStart {
		if ref > uint32(len(payload.BinaryFixPool)) {
			return fmt.Errorf("scanner: invalid binaryFixStart at row %d", row)
		}
	}
	if payload.N < 0 {
		return fmt.Errorf("scanner: invalid FindingColumns row count")
	}
	return nil
}
