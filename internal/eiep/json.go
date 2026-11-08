package eiep

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// jval is a parsed JSON value and the line it starts on.
type jval struct {
	line int
	v    any // string, json.Number, bool, nil, []*jval or *jobj
}

type jobj struct {
	keys []string
	m    map[string]*jval
}

type jparser struct {
	d       *json.Decoder
	data    []byte
	pos, ln int
}

func (p *jparser) line() int {
	off := int(p.d.InputOffset())
	p.ln += bytes.Count(p.data[p.pos:off], []byte("\n"))
	p.pos = off
	return p.ln
}

func (p *jparser) value() (*jval, error) {
	t, err := p.d.Token()
	if err != nil {
		return nil, err
	}
	v := &jval{p.line(), t}
	switch t {
	case json.Delim('{'):
		o := &jobj{m: map[string]*jval{}}
		for p.d.More() {
			k, err := p.d.Token()
			if err != nil {
				return nil, err
			}
			c, err := p.value()
			if err != nil {
				return nil, err
			}
			o.keys = append(o.keys, k.(string))
			o.m[k.(string)] = c
		}
		v.v = o
	case json.Delim('['):
		var a []*jval
		for p.d.More() {
			c, err := p.value()
			if err != nil {
				return nil, err
			}
			a = append(a, c)
		}
		v.v = a
	default:
		return v, nil
	}
	_, err = p.d.Token()
	return v, err
}

// text converts a JSON value to its CSV field: lists are space-separated, and pairs such as RCC-POA ["UN", 24] hyphenated.
func text(v *jval, sep string) (string, bool) {
	switch x := v.v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	case nil:
		return "", true
	case []*jval:
		parts := make([]string, len(x))
		for i, e := range x {
			s, ok := text(e, "-")
			if !ok {
				return "", false
			}
			parts[i] = s
		}
		return strings.Join(parts, sep), true
	}
	return "", false
}

type jpos struct {
	path string
	line int
}

// checkJSON validates an EIEP14A JSON file by flattening it into the records of the CSV format, so both share every check.
func checkJSON(data []byte) Result {
	p := &jparser{d: json.NewDecoder(bytes.NewReader(data)), data: data, ln: 1}
	p.d.UseNumber()
	root, err := p.value()
	if err == nil {
		if _, e := p.d.Token(); e != io.EOF {
			err = errors.New("unexpected data after the closing brace")
		}
	}
	if err != nil {
		line := p.line()
		var se *json.SyntaxError
		if errors.As(err, &se) {
			line = 1 + bytes.Count(data[:se.Offset], []byte("\n"))
		}
		return Result{Findings: []Finding{{line, "", "json", "not valid JSON: " + err.Error()}}}
	}

	layout := eiep14a.Versions["2"]
	var out []Finding
	var hdr []string
	var hpos map[string]jpos
	var recs []Record

	var walk func(typ, path string, n *jval)
	walk = func(typ, path string, n *jval) {
		o, ok := n.v.(*jobj)
		if !ok {
			out = append(out, Finding{n.line, path, "json_type", "must be a JSON object"})
			return
		}
		fs := layout.Records[typ]
		if typ == "HDR" {
			// The header is optional in JSON.
			fs = slices.Clone(layout.Header)
			for i := range fs {
				fs[i].Required = false
			}
		}
		v := make([]string, len(fs))
		v[0] = typ
		pos := map[string]jpos{}
		for _, f := range fs[1:] {
			pos[f.Name] = jpos{path + "." + f.JSON, n.line}
		}
		var allowed []string
		for _, f := range fs[1:] {
			allowed = append(allowed, f.JSON)
		}
		for _, c := range jsonChildren[typ] {
			allowed = append(allowed, c.key)
		}
		for _, k := range o.keys {
			val, name := o.m[k], k
			if a := jsonAliases[typ][k]; a != "" {
				out = append(out, Finding{val.line, path + "." + k, "json_alias", fmt.Sprintf("%q is the name used in the spec's examples; its tables name this element %q", k, a)})
				name = a
			}
			i := slices.IndexFunc(fs, func(f Field) bool { return f.JSON == name })
			switch {
			case i > 0:
				pos[fs[i].Name] = jpos{path + "." + k, val.line}
				if v[i], ok = text(val, " "); !ok {
					out = append(out, Finding{val.line, path + "." + k, "json_type", "must be a string, a number, null or a list of them"})
				}
			case !slices.Contains(allowed, k):
				out = append(out, Finding{val.line, path + "." + k, "unknown_element", fmt.Sprintf("%q is not an element of this object; use one of %s", k, strings.Join(allowed, ", "))})
			}
		}

		r := Record{n.line, typ, v, pos}
		var ps []Problem
		if typ == "HDR" {
			hdr, hpos, ps = v, pos, checkFields(fs, v)
		} else {
			ps = layout.checkRecord(typ, fs, v)
			recs = append(recs, r)
		}
		for _, pr := range ps {
			out = append(out, r.finding(pr.Field, pr.Code, pr.Msg))
		}

		for _, c := range jsonChildren[typ] {
			val := o.m[c.key]
			if val == nil || val.v == nil {
				continue
			}
			a, ok := val.v.([]*jval)
			if !ok {
				out = append(out, Finding{val.line, path + "." + c.key, "json_type", "must be a list"})
				continue
			}
			for i, e := range a {
				walk(c.typ, fmt.Sprintf("%s.%s[%d]", path, c.key, i), e)
			}
		}
	}
	walk("HDR", "$", root)

	version := or(hdr[2], "2")
	if major, _, _ := strings.Cut(version, "."); major != "2" {
		return Result{Findings: []Finding{{1, "$.Version", "unsupported", fmt.Sprintf("EIEP14A version %q is not supported in JSON; supported: 2.x", version)}}}
	}
	for _, f := range layout.CheckFile(File{hdr, recs, 0, true}) {
		if l, ok := hpos[f.Field]; ok && f.Line == 1 {
			f.Field, f.Line = l.path, l.line
		}
		out = append(out, f)
	}
	return finish(out, "EIEP14A "+version+" JSON", layout, File{hdr, recs, 0, true})
}
