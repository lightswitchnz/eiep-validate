package eiep

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EIEP 14A: Retailer price schedule (PRCSCHD), v2.0 effective 30 October 2026.
// This protocol writes Num n.d as up to n integer and d decimal digits (a rate is "Num 6.6"),
// so Len below is n+d.
var eiep14a = Format{Name: "EIEP14A", ByMajor: true, Versions: map[string]Layout{"2": {
	Header:      eiep14aHeader,
	Records:     eiep14aRecords,
	Quoted:      true,
	CheckRecord: checkEIEP14ARecord,
	CheckFile:   checkEIEP14AFile,
	Summary:     summariseEIEP14A,
}}}

const hdrRecordCount = 9

var eiep14aHeader = []Field{
	{Name: "Header record type", Kind: Char, Len: 3, Required: true, Codes: []string{"HDR"}},
	{Name: "File type", Kind: Char, Len: 7, Required: true, Codes: []string{"PRCSCHD"}},
	{Name: "Version", Kind: Num, Len: 6, Dec: 3, Required: true},
	{Name: "Sender", Kind: Char, Len: 20, Required: true, Hint: "participant identifier if the sender is a participant"},
	{Name: "Sent on behalf of", Kind: Char, Len: 4, Required: true, Hint: "participant identifier of the party the data is provided for"},
	{Name: "Recipient", Kind: Char, Len: 4, Hint: "party code, ANY for multiple recipients, or blank if none is assigned"},
	{Name: "Report run date and time", Kind: ISODateTime, Required: true},
	{Name: "Data extract date and time", Kind: ISODateTime},
	{Name: "Unique file identifier", Kind: UUID, Required: true},
	{Name: "Number of detailed records", Kind: Num, Len: 8, Required: true, Hint: "the number of lines in the file, header included"},
	{Name: "Utility type", Kind: Char, Len: 1, Required: true, Codes: []string{"E", "G"}, Hint: "E = electricity"},
	{Name: "File status", Kind: Char, Len: 1, Required: true, Codes: []string{"I", "R", "X"}, Hint: "I = initial, R = replacement, X = replaces only the tariffs in this file"},
}

func recordType(name string) Field {
	return Field{Name: "Record type", Kind: Char, Len: 15, Required: true, Codes: []string{name}}
}

func attributeIDs(n int) Field {
	return Field{Name: "Attribute identifiers", Kind: IdentList, Len: n, Hint: "ATTRIBUTE identifiers separated by spaces"}
}

var eiep14aRecords = map[string][]Field{
	"RETAILER": {
		recordType("RETAILER"),
		{Name: "Trader participant identifier", Kind: Char, Len: 4, Hint: "mandatory if the Authority has assigned one"},
		{Name: "Retailer participant identifier", Kind: Char, Len: 4},
		{Name: "Retailer name", Kind: Char, Len: 50, Required: true, Hint: "brand name"},
	},
	"ATTRIBUTE": {
		recordType("ATTRIBUTE"),
		{Name: "Attribute record identifier", Kind: Ident, Len: 20, Required: true},
		{Name: "Attribute code", Kind: Char, Len: 50, Hint: "e.g. ANZSIC, KVA_MAX, EXIT_FEE"},
		{Name: "Date value", Kind: ISODate},
		{Name: "Numerical value", Kind: Num, Len: 28, Dec: 8},
		{Name: "Text value", Kind: Char, Len: 50},
		{Name: "Description", Kind: Char, Len: 1000},
	},
	"SCHEDULE": {
		recordType("SCHEDULE"),
		{Name: "Schedule record identifier", Kind: Ident, Len: 20, Required: true},
		{Name: "Daylight savings adjustment", Kind: Char, Len: 1, Codes: []string{"1", "2"}, Hint: "1 = UTC+13 while daylight saving is in operation, 2 = always UTC+12"},
		{Name: "Start time", Kind: Clock},
		{Name: "End time", Kind: Clock},
		{Name: "Day type", Kind: Char, Len: 50},
		{Name: "Month", Kind: Char, Len: 50},
	},
	"TARIFFTYPE": {
		recordType("TARIFFTYPE"),
		{Name: "Tariff type identifier", Kind: Ident, Len: 20, Required: true},
		{Name: "Tariff name", Kind: Char, Len: 50, Required: true},
		{Name: "Fixed / variable", Kind: Char, Len: 1, Required: true, Codes: []string{"F", "V"}, Hint: "F = fixed, V = variable"},
		{Name: "Unit type", Kind: Char, Len: 25, Hint: "e.g. Day, kWh, kVA"},
		{Name: "Flow direction", Kind: Char, Len: 1, Codes: []string{"X", "I", "B"}, Hint: "X = consumption, I = generation, B = both"},
		{Name: "RCC-POA", Kind: Char, Len: 50},
		{Name: "Schedule identifiers", Kind: IdentList, Len: 200, Hint: "SCHEDULE identifiers separated by spaces"},
		attributeIDs(256),
	},
	"TARIFFREGION": {
		recordType("TARIFFREGION"),
		{Name: "Tariff region identifier", Kind: Ident, Len: 20, Required: true},
		{Name: "Description", Kind: Char, Len: 50},
	},
	"NETWORK": {
		recordType("NETWORK"),
		{Name: "Distributor participant identifier", Kind: Char, Len: 4},
		{Name: "NSP", Kind: Char, Len: 200},
		{Name: "Distributor price category code", Kind: Char, Len: 200},
		{Name: "Distributor loss category code", Kind: Char, Len: 50},
	},
	"CUSTOMER": {
		recordType("CUSTOMER"),
		{Name: "Customer text", Kind: Char, Len: 1000},
		attributeIDs(200),
	},
	"PLAN": {
		recordType("PLAN"),
		{Name: "Product identification code", Kind: Char, Len: 32, Required: true},
		{Name: "Plan description", Kind: Char, Len: 50, Required: true, Hint: "unique across all retailers' plans"},
		{Name: "Start date", Kind: ISODate},
		{Name: "End date", Kind: ISODate},
		{Name: "Close date", Kind: ISODate},
		{Name: "Low fixed charge", Kind: Char, Len: 1, Required: true, Codes: []string{"Y", "N"}},
		attributeIDs(256),
	},
	"TARIFF": {
		recordType("TARIFF"),
		{Name: "Tariff identifier", Kind: Char, Len: 50},
		{Name: "Tariff region identifier", Kind: Ident, Len: 20, Hint: "blank if the rate is common across all networks"},
		{Name: "Tariff type identifier", Kind: Ident, Len: 20, Required: true},
		{Name: "Tariff rate", Kind: Num, Len: 12, Dec: 6, Required: true},
		attributeIDs(256),
	},
}

var (
	dayTypes  = []string{"ANY", "BD", "NBD", "WD", "WE", "LBD", "LNBD", "SBD", "SPNBD"}
	dayNames  = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}
	monthName = []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}
	rccPOARe  = regexp.MustCompile(`^[A-Za-z0-9]{1,6}-([0-9]{1,2})$`)
)

func checkEIEP14ARecord(typ string, v []string) []Problem {
	fs := eiep14aRecords[typ]
	p := checkFields(fs, v)
	ok := func(i int) bool { c, _ := fs[i].check(v[i]); return v[i] != "" && c == "" }
	add := func(i int, code, msg string) { p = append(p, Problem{fs[i].Name, code, msg}) }

	switch typ {
	case "SCHEDULE":
		if (v[3] != "" || v[4] != "") && v[2] == "" {
			add(2, "blank", "mandatory when the schedule has a start or end time; fill it in"+fs[2].hint())
		}
		start, _ := parseClock(v[3])
		end, _ := parseClock(v[4])
		if ok(3) && start == 24*60 {
			add(3, "bad_time", `a start time cannot be 24:00; use "00:00"`)
		}
		if ok(3) && ok(4) && end <= start {
			add(4, "end_before_start", fmt.Sprintf("%q is not after the start %q; an active period cannot span midnight, so split it into two SCHEDULE records (e.g. 22:00-24:00 and 00:00-06:00)", v[4], v[3]))
		}
		if ok(5) && !validSet(v[5], dayTypes, dayNames) {
			add(5, "bad_code", fmt.Sprintf("%q is not a valid day type; use one of %s, or day names separated by single spaces, e.g. \"MON TUE WED\"", v[5], strings.Join(dayTypes, ", ")))
		}
		if ok(6) && !validSet(v[6], []string{"ANY"}, monthName) {
			add(6, "bad_code", fmt.Sprintf("%q is not a valid month; use ANY, or three-letter month names separated by single spaces, e.g. \"JUN JUL AUG\"", v[6]))
		}
	case "TARIFFTYPE":
		if ok(6) {
			for _, pair := range strings.Split(v[6], " ") {
				m := rccPOARe.FindStringSubmatch(pair)
				if m == nil || atoi(m[1]) > 24 {
					add(6, "bad_rcc_poa", fmt.Sprintf("%q is not a list of register content code and period of availability pairs; write each as CODE-HOURS (hours 0 to 24) separated by single spaces, e.g. \"UN-24 CN-19\"", v[6]))
					break
				}
			}
		}
	case "PLAN":
		start, _ := parseISODate(v[3])
		end, _ := parseISODate(v[4])
		closed, _ := parseISODate(v[5])
		if ok(3) && ok(4) && end.Before(start) {
			add(4, "end_before_start", fmt.Sprintf("%q is before the start date %q; the end date must not be earlier than the start", v[4], v[3]))
		}
		if ok(3) && ok(5) && closed.Before(start) {
			add(5, "close_before_start", fmt.Sprintf("%q is before the start date %q; the close date must not be earlier than the start", v[5], v[3]))
		}
		switch {
		case ok(4) && ok(5) && closed.After(end):
			add(5, "close_after_end", fmt.Sprintf("%q is after the end date %q; customers can't sign up after the plan ends, so use a date on or before the end date", v[5], v[4]))
		case ok(4) && ok(5) && closed.Equal(end):
			add(5, "close_equals_end", "is the same as the end date; leave the close date blank in that case")
		}
	}
	return p
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// validSet reports whether v is one of codes, or distinct names separated by single spaces.
func validSet(v string, codes, names []string) bool {
	if foldIn(v, codes) {
		return true
	}
	seen := map[string]bool{}
	for _, t := range strings.Split(strings.ToUpper(v), " ") {
		if !foldIn(t, names) || seen[t] {
			return false
		}
		seen[t] = true
	}
	return true
}

// identifiers maps each record type that defines an identifier to that identifier's field.
var identifiers = map[string]int{"ATTRIBUTE": 1, "SCHEDULE": 1, "TARIFFTYPE": 1, "TARIFFREGION": 1, "PLAN": 1}

var references = []struct {
	typ    string
	field  int
	target string
}{
	{"TARIFFTYPE", 7, "SCHEDULE"},
	{"TARIFFTYPE", 8, "ATTRIBUTE"},
	{"CUSTOMER", 2, "ATTRIBUTE"},
	{"PLAN", 7, "ATTRIBUTE"},
	{"TARIFF", 2, "TARIFFREGION"},
	{"TARIFF", 3, "TARIFFTYPE"},
	{"TARIFF", 5, "ATTRIBUTE"},
}

func checkEIEP14AFile(f File) []Finding {
	var out []Finding
	if n, err := strconv.Atoi(f.Header[hdrRecordCount]); err == nil && n != f.Lines {
		out = append(out, Finding{1, eiep14aHeader[hdrRecordCount].Name, "count_mismatch", fmt.Sprintf("declares %d but the file has %d lines (header included); set it to %d", n, f.Lines, f.Lines)})
	}
	if len(f.Records) == 0 {
		return append(out, Finding{1, "", "structure", "file has no RETAILER record; add the retailer and its plans after the header"})
	}

	var retailers [][]Record
	var prev string
	var customer, plan bool
	structure := func(r Record, msg string) { out = append(out, Finding{r.Line, "", "structure", msg}) }
	for _, r := range f.Records {
		if r.Type == "RETAILER" {
			retailers = append(retailers, nil)
			customer, plan, prev = false, false, r.Type
			continue
		}
		if retailers == nil {
			structure(r, fmt.Sprintf("%s record comes before any RETAILER record; every record belongs to the RETAILER above it", r.Type))
			continue
		}
		switch r.Type {
		case "NETWORK":
			if prev != "TARIFFREGION" && prev != "NETWORK" {
				structure(r, "NETWORK record must directly follow its TARIFFREGION record (or another NETWORK in the same region); move it there")
			}
		case "CUSTOMER":
			customer, plan = true, false
		case "PLAN":
			if !customer {
				structure(r, "PLAN record has no CUSTOMER record above it; plans belong to the customer type above them, so add a CUSTOMER record first")
			}
			plan = true
		case "TARIFF":
			if !plan {
				structure(r, "TARIFF record has no PLAN record above it; tariffs belong to the plan above them, so move it under its PLAN")
			}
		default:
			if customer {
				structure(r, fmt.Sprintf("%s record comes after a CUSTOMER record; ATTRIBUTE, SCHEDULE, TARIFFTYPE, TARIFFREGION and NETWORK records belong to the retailer, so move it above the retailer's first CUSTOMER", r.Type))
			}
		}
		retailers[len(retailers)-1] = append(retailers[len(retailers)-1], r)
		prev = r.Type
	}

	descriptions := map[string]int{}
	for _, recs := range retailers {
		out = append(out, checkRetailer(recs, descriptions)...)
	}
	return out
}

// checkRetailer checks identifiers are unique and references resolve within one retailer.
// descriptions collects plan descriptions across retailers, which must also be unique.
func checkRetailer(recs []Record, descriptions map[string]int) []Finding {
	var out []Finding
	valid := func(r Record, i int) bool {
		fs := eiep14aRecords[r.Type]
		if len(r.V) != len(fs) || r.V[i] == "" {
			return false
		}
		c, _ := fs[i].check(r.V[i])
		return c == ""
	}

	ids := map[string]map[string]int{}
	for _, r := range recs {
		i, ok := identifiers[r.Type]
		if !ok || !valid(r, i) {
			continue
		}
		if ids[r.Type] == nil {
			ids[r.Type] = map[string]int{}
		}
		id := strings.ToUpper(r.V[i])
		if line, dup := ids[r.Type][id]; dup {
			out = append(out, Finding{r.Line, eiep14aRecords[r.Type][i].Name, "duplicate_id", fmt.Sprintf("%q is already used by the %s record on line %d; identifiers must be unique within the retailer, so rename one of them", r.V[i], r.Type, line)})
			continue
		}
		ids[r.Type][id] = r.Line
	}

	for _, r := range recs {
		if r.Type == "PLAN" && valid(r, 2) {
			d := strings.ToUpper(r.V[2])
			if line, dup := descriptions[d]; dup {
				out = append(out, Finding{r.Line, "Plan description", "duplicate_description", fmt.Sprintf("%q is already used by the plan on line %d; each plan's description must be unique across all retailers' plans", r.V[2], line)})
			} else {
				descriptions[d] = r.Line
			}
		}
		for _, ref := range references {
			if ref.typ != r.Type || !valid(r, ref.field) {
				continue
			}
			for _, id := range strings.Split(r.V[ref.field], " ") {
				if _, ok := ids[ref.target][strings.ToUpper(id)]; !ok {
					out = append(out, Finding{r.Line, eiep14aRecords[r.Type][ref.field].Name, "unknown_reference", fmt.Sprintf("%q does not match any %s record under this RETAILER; add one with that identifier, or correct the reference", id, ref.target)})
				}
			}
		}
	}
	return out
}

func summariseEIEP14A(f File) string {
	n := map[string]int{}
	var from, to string
	for _, r := range f.Records {
		n[r.Type]++
		if r.Type != "PLAN" {
			continue
		}
		if d := r.V[3]; d != "" && (from == "" || d < from) {
			from = d
		}
		if d := r.V[4]; d != "" && d > to {
			to = d
		}
	}
	s := fmt.Sprintf("%d records (retailers: %d, plans: %d, tariffs: %d)", len(f.Records), n["RETAILER"], n["PLAN"], n["TARIFF"])
	if from != "" || to != "" {
		s += fmt.Sprintf(", plans from %s to %s", or(from, "open"), or(to, "open"))
	}
	return s
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
