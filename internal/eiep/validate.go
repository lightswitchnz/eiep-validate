// Package eiep validates NZ Electricity Authority EIEP files strictly against their specs.
package eiep

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Layout is one version of one EIEP format.
type Layout struct {
	// Name overrides the format's name for this version.
	Name   string
	Header []Field
	// Records holds the fields of each record type allowed after the header.
	Records map[string][]Field
	// Alt holds other accepted field lists per record type ("HDR" for the header), chosen by field count.
	Alt map[string][][]Field
	// Quoted allows RFC 4180 double-quoted fields.
	Quoted bool
	// CheckRecord replaces the plain per-field check of a record when set.
	CheckRecord func(typ string, v []string) []Problem
	// CheckFile runs once over a well-formed header and every record.
	CheckFile func(f File) []Finding
	// Summary describes a valid file's contents.
	Summary func(f File) string
}

type Format struct {
	Name     string
	Versions map[string]Layout
	// ByMajor keys Versions by the major version only, e.g. "2" for 2.011.
	ByMajor bool
}

// formats is keyed by the HDR file type field.
var formats = map[string]Format{
	"ICPCONS": eiep13a,
	"PRCSCHD": eiep14a,
}

type File struct {
	Header  []string
	Records []Record
	Lines   int
}

type Record struct {
	Line int
	Type string
	V    []string
}

type Finding struct {
	Line  int    `json:"line"`
	Field string `json:"field,omitempty"`
	Code  string `json:"code"`
	Msg   string `json:"message"`
}

var recordSep = regexp.MustCompile(`\r\n|\n|\r`)

func Validate(data []byte) []Finding {
	return Check(data).Findings
}

// collapsed are warnings reported once, with a count of the rest appended.
var collapsed = map[string]string{
	"extra_decimals":     "; %d more values in this field also have extra decimal places, up to line %d",
	"extra_empty_fields": "; %d more records also have empty trailing fields, up to line %d",
}

// warningCodes are findings that do not make a file invalid.
var warningCodes = map[string]bool{"extra_decimals": true, "leading_zeros": true, "edge_spaces": true, "duplicate_plan": true, "too_long": true, "extra_empty_fields": true, "shared_description": true, "duplicate_description": true, "deprecated": true}

type Result struct {
	Findings []Finding
	Warnings []Finding
	// Format is the detected format and declared version, e.g. "EIEP13A 1.4"; blank if unsupported.
	Format string
	// Summary describes the contents of a valid file.
	Summary string
}

func Check(data []byte) Result {
	records := recordSep.Split(string(data), -1)
	if records[len(records)-1] == "" {
		records = records[:len(records)-1]
	}
	if len(records) == 0 {
		return Result{Findings: []Finding{{1, "", "structure", "file is empty; it must start with an HDR header record"}}}
	}

	var out []Finding
	add := func(line int, ps []Problem) {
		for _, p := range ps {
			out = append(out, Finding{line, p.Field, p.Code, p.Msg})
		}
	}

	hdr := strings.Split(records[0], ",")
	if !strings.EqualFold(hdr[0], "HDR") {
		return Result{Findings: []Finding{{1, "", "structure", fmt.Sprintf("first record must be the HDR header but starts with %q; remove anything above the header (e.g. a row of column titles)", hdr[0])}}}
	}
	if len(hdr) < 3 {
		return Result{Findings: []Finding{{1, "", "structure", "header is too short; it must declare a file type and version, e.g. HDR,ICPCONS,1.4,..."}}}
	}
	format, ok := formats[strings.ToUpper(hdr[1])]
	if !ok {
		return Result{Findings: []Finding{{1, "File type", "unsupported", fmt.Sprintf("%q is not a supported file type; supported: %s", hdr[1], keys(formats))}}}
	}
	version, supported := hdr[2], keys(format.Versions)
	if format.ByMajor {
		version, _, _ = strings.Cut(version, ".")
		supported = strings.ReplaceAll(supported+",", ",", ".x,")
		supported = strings.TrimSuffix(supported, ",")
	}
	layout, ok := format.Versions[version]
	if !ok {
		return Result{Findings: []Finding{{1, "Version", "unsupported", fmt.Sprintf("%s version %q is not supported; supported: %s. Declare the version the file actually follows", format.Name, hdr[2], supported)}}}
	}

	name := format.Name + " " + hdr[2]
	if layout.Name != "" {
		name = layout.Name + " " + hdr[2]
	}
	split := func(s string) ([]string, error) { return strings.Split(s, ","), nil }
	if layout.Quoted {
		split = splitQuoted
	}
	hdr, err := split(records[0])
	hdr = layout.trimEmpty("HDR", layout.Header, hdr, 1, add)
	hfs := layout.fields("HDR", layout.Header, len(hdr))
	hdrOK := err == nil && len(hdr) == len(hfs)
	switch {
	case err != nil:
		out = append(out, Finding{1, "", "quoting", err.Error()})
	case hdrOK:
		add(1, checkFields(hfs, hdr))
	default:
		out = append(out, Finding{1, "", "field_count", fieldCountMsg("header", hdr, layout.Header, layout.Quoted)})
	}

	var recs []Record
	for i, rec := range records[1:] {
		line := i + 2
		if rec == "" {
			out = append(out, Finding{line, "", "structure", "blank line; remove it"})
			continue
		}
		v, err := split(rec)
		if err != nil {
			out = append(out, Finding{line, "", "quoting", err.Error()})
			// Keep the record's place in the file's structure.
			typ, _, _ := strings.Cut(rec, ",")
			if _, ok := layout.Records[strings.ToUpper(typ)]; ok {
				recs = append(recs, Record{line, strings.ToUpper(typ), nil})
			}
			continue
		}
		typ := strings.ToUpper(v[0])
		fs, ok := layout.Records[typ]
		switch {
		case typ == "HDR":
			out = append(out, Finding{line, "", "structure", "second HDR record; a file may contain only one header, so send each report as its own file"})
			continue
		case !ok:
			out = append(out, Finding{line, "", "structure", fmt.Sprintf("unknown record type %q; every row after the header must start with one of: %s", v[0], keys(layout.Records))})
			continue
		}
		v = layout.trimEmpty(typ, fs, v, line, add)
		recs = append(recs, Record{line, typ, v})
		fs = layout.fields(typ, fs, len(v))
		if len(v) != len(fs) {
			out = append(out, Finding{line, "", "field_count", fieldCountMsg(typ+" record", v, fs, layout.Quoted)})
			continue
		}
		add(line, layout.checkRecord(typ, fs, v))
	}

	f := File{hdr, recs, len(records)}
	if hdrOK && layout.CheckFile != nil {
		out = append(out, layout.CheckFile(f)...)
	}
	return finish(out, name, layout, f)
}

func (l Layout) checkRecord(typ string, fs []Field, v []string) []Problem {
	if l.CheckRecord != nil {
		return l.CheckRecord(typ, v)
	}
	return checkFields(fs, v)
}

// finish sorts findings, splits off warnings and collapses repeated ones.
func finish(out []Finding, name string, layout Layout, f File) Result {
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	r := Result{Format: name}
	// Extra decimals usually affect a whole column, so report one warning per field.
	first, more, last := map[string]int{}, map[string]int{}, map[string]int{}
	for _, f := range out {
		if collapsed[f.Code] != "" {
			k := f.Code + "|" + f.Field
			if _, ok := first[k]; ok {
				more[k]++
				last[k] = f.Line
				continue
			}
			first[k] = len(r.Warnings)
		}
		if warningCodes[f.Code] {
			r.Warnings = append(r.Warnings, f)
		} else {
			r.Findings = append(r.Findings, f)
		}
	}
	for k, n := range more {
		w := &r.Warnings[first[k]]
		w.Msg += fmt.Sprintf(collapsed[w.Code], n, last[k])
	}
	if len(r.Findings) == 0 && layout.Summary != nil {
		r.Summary = layout.Summary(f)
	}
	return r
}

// fields returns the field list for typ that has n fields, or fs if none does.
func (l Layout) fields(typ string, fs []Field, n int) []Field {
	for _, a := range l.Alt[typ] {
		if len(a) == n {
			return a
		}
	}
	return fs
}

// trimEmpty drops empty trailing fields beyond the longest field list for typ, with a warning.
func (l Layout) trimEmpty(typ string, fs []Field, v []string, line int, add func(int, []Problem)) []string {
	n := -1
	for _, c := range append([][]Field{fs}, l.Alt[typ]...) {
		if len(c) == len(v) {
			return v
		}
		if len(c) < len(v) && len(c) > n && strings.Join(v[len(c):], "") == "" {
			n = len(c)
		}
	}
	if n < 0 {
		return v
	}
	add(line, []Problem{{"", "extra_empty_fields", fmt.Sprintf("%s record has %d empty fields after the %d expected; remove the trailing commas", typ, len(v)-n, n)}})
	return v[:n]
}

func fieldCountMsg(what string, v []string, fs []Field, quoted bool) string {
	got := len(v)
	fix := "add the missing fields (a blank field still needs its comma)"
	switch {
	case got > len(fs) && quoted:
		fix = "remove the extra fields (enclose a field in double quotes if it must contain a comma)"
	case got > len(fs):
		fix = "remove the extra fields (commas are not allowed inside a field)"
	}
	return fmt.Sprintf("%s has %d fields but must have %d; %s.%s", what, got, len(fs), fix, expectedGot(fs, v))
}

func keys[V any](m map[string]V) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ", ")
}

// splitQuoted splits a record per RFC 4180: a field is either unquoted, with no double quotes,
// or wrapped in double quotes with each quote inside it doubled.
func splitQuoted(s string) ([]string, error) {
	var out []string
	for {
		n := len(out) + 1
		if !strings.HasPrefix(s, `"`) {
			f, rest, more := strings.Cut(s, ",")
			if strings.Contains(f, `"`) {
				return nil, fmt.Errorf("field %d contains a double quote but is not enclosed in double quotes; enclose the whole field in quotes and write each quote inside it as \"\"", n)
			}
			out = append(out, f)
			if !more {
				return out, nil
			}
			s = rest
			continue
		}
		var b strings.Builder
		i := 1
		for {
			j := strings.IndexByte(s[i:], '"')
			if j < 0 {
				return nil, fmt.Errorf("field %d opens a double quote that is never closed; add the closing quote", n)
			}
			b.WriteString(s[i : i+j])
			i += j + 1
			if i < len(s) && s[i] == '"' {
				b.WriteByte('"')
				i++
				continue
			}
			break
		}
		out = append(out, b.String())
		switch s = s[i:]; {
		case s == "":
			return out, nil
		case s[0] != ',':
			return nil, fmt.Errorf("field %d has text after its closing double quote; end the field at the quote, or write each quote inside a quoted field as \"\"", n)
		}
		s = s[1:]
	}
}
