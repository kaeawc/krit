package scanner

import "bytes"

// braceDeclStarters are tokens that begin a declaration. Kotlin allows one to
// follow a closing brace on the same line (`class A { fun a() {} fun b() {} }`),
// but the grammar needs a separator and turns the whole body into an ERROR.
var braceDeclStarters = map[string]bool{
	"fun": true, "val": true, "var": true, "class": true, "object": true, "interface": true,
	"private": true, "public": true, "internal": true, "protected": true, "override": true,
	"open": true, "abstract": true, "final": true, "data": true, "inner": true, "sealed": true,
	"enum": true, "annotation": true, "companion": true, "suspend": true, "inline": true,
	"operator": true, "infix": true, "const": true, "lateinit": true, "tailrec": true,
}

// rewriteBraceDeclSeparators puts a ';' in the space between a closing brace
// and a declaration that starts on the same line.
func (r *kotlinRewriter) rewriteBraceDeclSeparators() {
	o := r.orig
	for i := 0; i < len(o); {
		j := bytes.IndexByte(o[i:], '}')
		if j < 0 {
			return
		}
		b := i + j
		i = b + 1
		sp := b + 1
		if sp >= len(o) || (o[sp] != ' ' && o[sp] != '\t') {
			continue
		}
		k := sp
		for k < len(o) && (o[k] == ' ' || o[k] == '\t') {
			k++
		}
		e := k
		for e < len(o) && isIdentByte(o[e]) {
			e++
		}
		if e == k || !braceDeclStarters[string(o[k:e])] || !r.isCode(b) {
			continue
		}
		r.set(sp, ';')
		r.semis = append(r.semis, uint32(sp))
	}
}
