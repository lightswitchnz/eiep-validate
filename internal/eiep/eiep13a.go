package eiep

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EIEP 13A: Electricity conveyed information for consumers, v1.4 (26 July 2022).
// v1.3 has a Char 15 unique request identifier; v1.2 also lacks response codes 005/006;
// v1.1 also has a Char 15 metering component serial number.
var eiep13a = Format{Name: "EIEP13A", Versions: map[string]Layout{
	"1.1": eiep13aLayout(15, 15, 5),
	"1.2": eiep13aLayout(15, 30, 5),
	"1.3": eiep13aLayout(15, 30, 7),
	"1.4": eiep13aLayout(36, 30, 7),
}}

const (
	hdrDetailCount = 8
	hdrPeriodStart = 9
	hdrPeriodEnd   = 10

	detICP          = 2
	detResponseCode = 3
	detNZDT         = 4
	detSerial       = 5
	detFlow         = 6
	detRegister     = 7
	detAvailability = 8
	detReadStart    = 9
	detReadEnd      = 10
)

var eiep13aDetail = []Field{
	{Name: "Detail record type", Kind: Char, Len: 3, Required: true, Codes: []string{"DET"}},
	{Name: "Consumer authorisation code", Kind: Char, Len: 20, Hint: "from the EIEP13C request; blank if there wasn't one"},
	{Name: "ICP identifier", Kind: Char, Len: 15, Required: true, Hint: "15-character ICP, e.g. 0001234567AB123"},
	{Name: "Response code", Kind: Char, Len: 3, Required: true, Codes: []string{"000", "001", "002", "003", "004", "005", "006"}, Hint: "000 = accepted, data follows"},
	{Name: "NZDT adjustment", Kind: Char, Len: 4, Codes: []string{"NZST"}, Hint: "leave blank when times are NZDT-adjusted"},
	{Name: "Metering component serial number", Kind: Char, Len: 30, Hint: "UNM for unmetered load"},
	{Name: "Energy flow direction", Kind: Char, Len: 1, Required: true, Codes: []string{"I", "X"}, Hint: "I = injection into the network, X = extraction from the network"},
	{Name: "Register content code", Kind: Char, Len: 6, Required: true, Hint: "registry register content code, e.g. UN, CN"},
	{Name: "Period of availability", Kind: Char, Len: 6, Required: true, Hint: "registry period of availability for the register, e.g. 24"},
	{Name: "Read period start date and time", Kind: DateTime, Required: true},
	{Name: "Read period end date and time", Kind: DateTime, Required: true},
	{Name: "Read status", Kind: Char, Len: 2, Required: true, Codes: []string{"RD", "ES"}, Hint: "RD = actual, ES = estimated"},
	{Name: "Unit quantity active energy volume", Kind: Num, Len: 12, Dec: 2, Required: true, Hint: "kWh"},
	{Name: "Unit quantity reactive energy volume", Kind: Num, Len: 12, Dec: 2, Hint: "kVArh; leave blank if not provided"},
}

func eiep13aLayout(uidLen, serialLen, codes int) Layout {
	det := append([]Field(nil), eiep13aDetail...)
	det[detSerial].Len = serialLen
	det[detResponseCode].Codes = det[detResponseCode].Codes[:codes]
	return Layout{
		Header: []Field{
			{Name: "Header record type", Kind: Char, Len: 3, Required: true, Codes: []string{"HDR"}},
			{Name: "File type", Kind: Char, Len: 7, Required: true, Codes: []string{"ICPCONS"}},
			{Name: "Version", Kind: Num, Len: 3, Dec: 1, Required: true},
			{Name: "Sender", Kind: Char, Len: 20, Required: true, Hint: "sending party's participant identifier"},
			{Name: "Sent on behalf of", Kind: Char, Len: 4, Required: true, Hint: "4-character participant identifier, e.g. CTCT"},
			{Name: "Recipient participant identifier", Kind: Char, Len: 4, Required: true, Hint: "CUST for a consumer, or the agent's participant identifier"},
			{Name: "Report run date", Kind: Date, Required: true},
			{Name: "Unique request identifier", Kind: Char, Len: uidLen, Required: true, Hint: "copy it from the EIEP13C request"},
			{Name: "Number of detail records", Kind: Num, Len: 8, Required: true, Hint: "the number of DET rows in the file"},
			{Name: "Report period start date", Kind: Date, Required: true},
			{Name: "Report period end date", Kind: Date, Required: true},
		},
		Records:     map[string][]Field{"DET": det},
		CheckRecord: func(_ string, v []string) []Problem { return checkEIEP13ADetail(det, v) },
		CheckFile:   checkEIEP13AFile,
		Summary:     summariseEIEP13A,
	}
}

func checkEIEP13ADetail(det []Field, v []string) []Problem {
	rest := detResponseCode + 1
	p := checkFields(det[:rest], v[:rest])

	switch code := v[detResponseCode]; code {
	case "001", "002", "003", "004":
		// The spec requires every field after the response code to be NULL.
		for i := rest; i < len(v); i++ {
			if v[i] != "" {
				p = append(p, Problem{det[i].Name, "must_be_blank", fmt.Sprintf("must be blank when the response code is %s (request rejected); remove %q", code, v[i])})
			}
		}
		return p
	case "005", "006":
		// Rejected, but the spec doesn't say these must be NULL: check format only.
		fs := append([]Field(nil), det[rest:]...)
		for i := range fs {
			fs[i].Required = false
		}
		return append(p, checkFields(fs, v[rest:])...)
	}

	p = append(p, checkFields(det[rest:], v[rest:])...)
	start, err1 := parseDateTime(v[detReadStart])
	end, err2 := parseDateTime(v[detReadEnd])
	if err1 == nil && err2 == nil && !end.After(start) {
		p = append(p, Problem{det[detReadEnd].Name, "end_before_start", fmt.Sprintf("%q is not after the start %q; the end must be later than the start", v[detReadEnd], v[detReadStart])})
	}
	return p
}

func checkEIEP13AFile(f File) []Finding {
	hdr, dets := f.Header, f.Records
	var out []Finding
	if n, err := strconv.Atoi(hdr[hdrDetailCount]); err == nil && n != len(dets) {
		out = append(out, Finding{1, "Number of detail records", "count_mismatch", fmt.Sprintf("declares %d but the file has %d DET records; set it to %d", n, len(dets), len(dets))})
	}
	start, err1 := parseDate(hdr[hdrPeriodStart])
	end, err2 := parseDate(hdr[hdrPeriodEnd])
	if err1 == nil && err2 == nil && end.Before(start) {
		out = append(out, Finding{1, "Report period end date", "end_before_start", fmt.Sprintf("%q is before the start date %q; the end date must not be earlier than the start", hdr[hdrPeriodEnd], hdr[hdrPeriodStart])})
	}
	return append(out, duplicatePeriods(dets)...)
}

func summariseEIEP13A(f File) string {
	type span struct {
		n           int
		first, last time.Time
		from, to    string
	}
	add := func(x *span, v []string) {
		x.n++
		start, err1 := parseDateTime(v[detReadStart])
		end, err2 := parseDateTime(v[detReadEnd])
		if err1 != nil || err2 != nil {
			return
		}
		if x.from == "" || start.Before(x.first) {
			x.first, x.from = start, v[detReadStart]
		}
		if x.to == "" || end.After(x.last) {
			x.last, x.to = end, v[detReadEnd]
		}
	}
	reads := func(x *span) string {
		if x.from == "" {
			return ""
		}
		return fmt.Sprintf(", reads %s to %s", x.from, x.to)
	}

	var all span
	regs := map[register]*span{}
	var order []register
	icps := map[string]bool{}
	for _, d := range f.Records {
		v := d.V
		icps[v[detICP]] = true
		add(&all, v)
		if v[detResponseCode] != "000" {
			continue
		}
		r := register{v[detICP], v[detSerial], strings.ToUpper(v[detRegister]), v[detAvailability], strings.ToUpper(v[detFlow])}
		if regs[r] == nil {
			regs[r] = &span{}
			order = append(order, r)
		}
		add(regs[r], v)
	}

	plural := "s"
	if len(icps) == 1 {
		plural = ""
	}
	s := fmt.Sprintf("%d DET records, %d ICP%s, report period %s to %s%s", all.n, len(icps), plural, f.Header[hdrPeriodStart], f.Header[hdrPeriodEnd], reads(&all))
	for _, r := range order {
		x := regs[r]
		s += fmt.Sprintf("\n  ICP %s, meter %q, register %s/%s, flow %s: %d DET records%s", r.icp, r.serial, r.content, r.availability, flowLabel(r.flow), x.n, reads(x))
	}
	return s
}

// flowLabel names the direction from the consumer's side: X is extracted from the network
// into the ICP, I is injected from the ICP into the network.
func flowLabel(flow string) string {
	switch flow {
	case "X":
		return "X (IN)"
	case "I":
		return "I (OUT)"
	}
	return flow
}

type register struct{ icp, serial, content, availability, flow string }

type period struct{ start, end time.Time }

// duplicatePeriods reports registers that give more than one reading for the same read
// period, which is what a file looks like when several registers are labelled as one.
// One finding per register, at its first repeated row.
func duplicatePeriods(dets []Record) []Finding {
	type dup struct {
		line, firstLine int
		start, end      string
		more            int
	}
	seen := map[register]map[period][]int{}
	dups := map[register]*dup{}
	var order []register

	for _, d := range dets {
		v := d.V
		if len(v) != len(eiep13aDetail) || v[detResponseCode] != "000" {
			continue
		}
		start, err1 := parseDateTime(v[detReadStart])
		end, err2 := parseDateTime(v[detReadEnd])
		if err1 != nil || err2 != nil {
			continue
		}
		r := register{v[detICP], v[detSerial], strings.ToUpper(v[detRegister]), v[detAvailability], strings.ToUpper(v[detFlow])}
		pr := period{start, end}
		if seen[r] == nil {
			seen[r] = map[period][]int{}
		}
		lines := append(seen[r][pr], d.Line)
		seen[r][pr] = lines

		allowed := 1
		if !strings.EqualFold(v[detNZDT], "NZST") && inRepeatedHour(start, end) {
			allowed = 2
		}
		if len(lines) <= allowed {
			continue
		}
		if x, ok := dups[r]; ok {
			x.more++
			continue
		}
		dups[r] = &dup{d.Line, lines[0], v[detReadStart], v[detReadEnd], 0}
		order = append(order, r)
	}

	var out []Finding
	for _, r := range order {
		x := dups[r]
		more := ""
		if x.more > 0 {
			more = fmt.Sprintf(", and %d more rows repeat periods already reported for it", x.more)
		}
		out = append(out, Finding{x.line, "Register content code", "duplicate_period", fmt.Sprintf(
			"register (ICP %s, meter %q, register %s/%s, flow %s) already has a reading for %s to %s on line %d%s. "+
				"Each register may report each read period only once: if these rows come from a different register "+
				"(e.g. a controlled load), give them that register's meter serial number, register content code and "+
				"period of availability; otherwise remove the duplicates",
			r.icp, r.serial, r.content, r.availability, r.flow, x.start, x.end, x.firstLine, more)})
	}
	return out
}

// inRepeatedHour reports whether a period falls in the wall-clock hour that occurs twice when
// NZ daylight saving ends (02:00–03:00 on the first Sunday in April), so legitimately appears
// twice in NZDT-adjusted data.
func inRepeatedHour(start, end time.Time) bool {
	d := time.Date(start.Year(), time.April, 1, 0, 0, 0, 0, time.UTC)
	d = d.AddDate(0, 0, (7-int(d.Weekday()))%7)
	return !start.Before(d.Add(2*time.Hour)) && !end.After(d.Add(3*time.Hour))
}
