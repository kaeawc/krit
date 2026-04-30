// Package daemon implements a long-lived krit process that keeps parse
// trees, the cross-file index, oracle state, and typeinfer caches resident
// in memory. CLI verbs in the build-integration cluster prefer the daemon
// when a socket is reachable and fall back to in-process execution
// otherwise.
//
// The protocol is line-delimited JSON over a Unix socket. Each request is
// a single JSON object terminated by a newline; each response is a single
// JSON object terminated by a newline.
package daemon

import "encoding/json"

// Request names a verb and carries its arguments as opaque JSON.
type Request struct {
	Verb string          `json:"verb"`
	Args json.RawMessage `json:"args,omitempty"`
}

// Response is the wire form of a verb result. OK=false carries an Error
// message and an empty Data; OK=true carries the verb-specific Data.
type Response struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Built-in verb names.
const (
	VerbStatus   = "status"
	VerbShutdown = "shutdown"
	VerbAbiHash  = "abi-hash"
)

// AbiHashArgs is the argument shape for the abi-hash verb.
type AbiHashArgs struct {
	Target string `json:"target"`
}

// AbiHashResult is the response payload for the abi-hash verb.
type AbiHashResult struct {
	Target string `json:"target"`
	Module string `json:"module,omitempty"`
	File   string `json:"file,omitempty"`
	Hash   string `json:"hash"`
	Inputs int    `json:"inputs"`
}

// StatusResult reports daemon readiness and basic warm-up stats.
type StatusResult struct {
	Ready       bool    `json:"ready"`
	Root        string  `json:"root"`
	Modules     int     `json:"modules"`
	Files       int     `json:"files"`
	WarmSeconds float64 `json:"warm_seconds"`
}
