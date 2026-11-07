package scanner

import "bytes"

// Class, interface and object headers: two forms break the grammar.
//
//   - An annotated supertype, `: @Suppress("X") Base<T>`, is blanked.
//   - `: Iface by expr {`. The grammar reads `expr { ... }` as a call with a
//     trailing lambda, so the body is lost. The `by expr` clause is blanked and
//     rebuilt as an explicit_delegation after parsing (see graftDelegations).

var headerModifierWords = map[string]bool{
	"private": true, "public": true, "internal": true, "protected": true, "constructor": true,
}

func (r *kotlinRewriter) rewriteHeaders() {
	for _, kw := range []string{"class", "object", "interface"} {
		o := r.orig
		for pos := 0; pos < len(o); {
			k := indexWord(o, kw, pos)
			if k < 0 {
				break
			}
			pos = k + len(kw)
			if !r.wordAt(k, kw) {
				continue
			}
			if p := r.prevNonSpace(k); p >= 0 && (o[p] == '.' || o[p] == ':' && p > 0 && o[p-1] == ':') {
				continue
			}
			r.scanHeader(pos)
		}
	}
}

func indexWord(o []byte, w string, from int) int {
	k := bytes.Index(o[from:], []byte(w))
	if k < 0 {
		return -1
	}
	return from + k
}

// scanHeader reads a header whose keyword ends at i.
func (r *kotlinRewriter) scanHeader(i int) {
	o := r.orig
	i = r.skipWS(i)
	if i < len(o) && isIdentStartByte(o[i]) {
		for i < len(o) && isIdentByte(o[i]) {
			i++
		}
	}
	i = r.skipWS(i)
	if i < len(o) && o[i] == '<' {
		if i = r.skipAngle(i); i < 0 {
			return
		}
		i = r.skipWS(i)
	}
	for n := 0; n < 8 && i < len(o); n++ {
		if o[i] == '@' {
			j := i + 1
			for j < len(o) && (isIdentByte(o[j]) || o[j] == '.' || o[j] == ':') {
				j++
			}
			if j < len(o) && o[j] == '(' {
				if j = r.skipBalanced(j, '(', ')'); j < 0 {
					return
				}
			}
			i = r.skipWS(j)
			continue
		}
		j := i
		for j < len(o) && isIdentByte(o[j]) {
			j++
		}
		if j > i && headerModifierWords[string(o[i:j])] {
			i = r.skipWS(j)
			continue
		}
		break
	}
	if i < len(o) && o[i] == '(' {
		if i = r.skipBalanced(i, '(', ')'); i < 0 {
			return
		}
		i = r.skipWS(i)
	}
	if i >= len(o) || o[i] != ':' {
		return
	}
	r.scanSupertypes(i + 1)
}

// scanSupertypes walks the supertype list that starts at i.
func (r *kotlinRewriter) scanSupertypes(i int) {
	o := r.orig
	depth := 0
	lastBy := -1
	for j := i; j < len(o); j++ {
		if !r.isCode(j) {
			continue
		}
		switch c := o[j]; c {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
			if depth < 0 {
				return
			}
		case '<':
			if end := r.skipAngle(j); end > 0 {
				j = end - 1
			}
		case '{':
			if depth == 0 {
				r.finishHeader(j, lastBy)
				return
			}
			depth++
		case '}':
			depth--
			if depth < 0 {
				return
			}
		case ',':
			if depth == 0 {
				lastBy = -1
			}
		case ';':
			return
		case '=':
			if depth == 0 {
				return
			}
		case '@':
			if depth != 0 {
				continue
			}
			if p := r.prevNonSpace(j); p >= 0 && (o[p] == ':' || o[p] == ',') {
				end := r.annotationEnd(j)
				r.blank(j, end)
				j = end - 1
			}
		case 'b':
			if depth == 0 && r.wordAt(j, "by") {
				lastBy = j
				j++
			}
		case '\n':
			if depth == 0 && !r.headerContinues(j) {
				return
			}
		}
	}
}

// headerContinues reports whether the header goes on after the line break at
// nl.
func (r *kotlinRewriter) headerContinues(nl int) bool {
	o := r.orig
	if p := r.prevNonSpace(nl); p >= 0 && (o[p] == ',' || o[p] == ':') {
		return true
	}
	k := r.skipWS(nl)
	if k >= len(o) {
		return false
	}
	return o[k] == '{' || o[k] == ',' || r.wordAt(k, "by") || r.wordAt(k, "where")
}

// annotationEnd returns the index after the annotation starting at the '@' at i.
func (r *kotlinRewriter) annotationEnd(i int) int {
	o := r.orig
	j := i + 1
	for j < len(o) && (isIdentByte(o[j]) || o[j] == '.' || o[j] == ':') {
		j++
	}
	if j < len(o) && o[j] == '(' {
		if end := r.skipBalanced(j, '(', ')'); end > 0 {
			j = end
		}
	}
	return j
}

// finishHeader handles the body brace at brace after a supertype list whose
// last `by` is at lastBy (or -1).
func (r *kotlinRewriter) finishHeader(brace, lastBy int) {
	if lastBy < 0 {
		return
	}
	exprStart := r.skipWS(lastBy + 2)
	exprEnd := r.prevNonSpace(brace) + 1
	typeEnd := r.prevNonSpace(lastBy) + 1
	if exprStart >= exprEnd || typeEnd <= 0 {
		return
	}
	r.blank(lastBy, exprEnd)
	r.delegations = append(r.delegations, delegationRepair{
		typeEnd: uint32(typeEnd), byStart: uint32(lastBy), exprStart: uint32(exprStart), exprEnd: uint32(exprEnd),
	})
}
