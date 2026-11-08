package eiep

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
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
	Summary:     func(f File) string { return summarisePlans(f, parseISODate, isoDate) },
}, "1": eiep14v1}}

const hdrRecordCount = 9

var eiep14aHeader = []Field{
	{Name: "Header record type", Kind: Char, Len: 3, Required: true, Codes: []string{"HDR"}},
	{Name: "File type", Kind: Char, Len: 7, Required: true, Codes: []string{"PRCSCHD"}, JSON: "FileType"},
	{Name: "Version", Kind: Num, Len: 6, Dec: 3, Required: true, JSON: "Version"},
	{Name: "Sender", Kind: Char, Len: 20, Required: true, Hint: "participant identifier if the sender is a participant", JSON: "Sender"},
	{Name: "Sent on behalf of", Kind: Char, Len: 4, Required: true, Hint: "participant identifier of the party the data is provided for", JSON: "SentOnBehalfOf"},
	{Name: "Recipient", Kind: Char, Len: 4, Hint: "party code, ANY for multiple recipients, or blank if none is assigned", JSON: "Recipient"},
	{Name: "Report run date and time", Kind: ISODateTime, Required: true, JSON: "RunDateTime"},
	{Name: "Data extract date and time", Kind: ISODateTime, JSON: "ExtractDateTime"},
	{Name: "Unique file identifier", Kind: UUID, Required: true, JSON: "Uniquifier"},
	{Name: "Number of detailed records", Kind: Num, Len: 8, Required: true, Hint: "the number of lines in the file after the header", JSON: "RecordCount"},
	{Name: "Utility type", Kind: Char, Len: 1, Required: true, Codes: []string{"E", "G"}, Hint: "E = electricity", JSON: "UtilType"},
	{Name: "File status", Kind: Char, Len: 1, Required: true, Codes: []string{"I", "R", "X"}, Hint: "I = initial, R = replacement, X = replaces only the tariffs in this file", JSON: "FileStatus"},
}

func recordType(name string) Field {
	return Field{Name: "Record type", Kind: Char, Len: 15, Required: true, Codes: []string{name}}
}

func attributeIDs(n int) Field {
	return Field{Name: "Attribute identifiers", Kind: IdentList, Len: n, Hint: "ATTRIBUTE identifiers separated by spaces", JSON: "AttributeIds"}
}

var eiep14aRecords = map[string][]Field{
	"RETAILER": {
		recordType("RETAILER"),
		{Name: "Trader participant identifier", Kind: Char, Len: 4, Hint: "mandatory if the Authority has assigned one", JSON: "TraderId"},
		{Name: "Retailer participant identifier", Kind: Char, Len: 4, JSON: "RetailerId"},
		{Name: "Retailer name", Kind: Char, Len: 50, Required: true, Hint: "brand name", JSON: "RetailerBrandName"},
	},
	"ATTRIBUTE": {
		recordType("ATTRIBUTE"),
		{Name: "Attribute record identifier", Kind: Ident, Len: 20, Required: true, JSON: "AttributeId"},
		{Name: "Attribute code", Kind: Char, Len: 50, Hint: "e.g. ANZSIC, KVA_MAX, EXIT_FEE", JSON: "Attribute"},
		{Name: "Date value", Kind: ISODate, JSON: "DateValue"},
		{Name: "Numerical value", Kind: Num, Len: 28, Dec: 8, JSON: "NumValue"},
		{Name: "Text value", Kind: Char, Len: 50, JSON: "TextValue"},
		{Name: "Description", Kind: Char, Len: 1000, JSON: "Description"},
	},
	"SCHEDULE": {
		recordType("SCHEDULE"),
		{Name: "Schedule record identifier", Kind: Ident, Len: 20, Required: true, JSON: "ScheduleId"},
		{Name: "Daylight savings adjustment", Kind: Char, Len: 1, Codes: []string{"1", "2"}, Hint: "1 = UTC+13 while daylight saving is in operation, 2 = always UTC+12", JSON: "DaylightSavings"},
		{Name: "Start time", Kind: Clock, JSON: "StartTime"},
		{Name: "End time", Kind: Clock, JSON: "EndTime"},
		{Name: "Day type", Kind: Char, Len: 50, JSON: "DayType"},
		{Name: "Month", Kind: Char, Len: 50, JSON: "Month"},
	},
	"TARIFFTYPE": {
		recordType("TARIFFTYPE"),
		{Name: "Tariff type identifier", Kind: Ident, Len: 20, Required: true, JSON: "TariffTypeId"},
		{Name: "Tariff name", Kind: Char, Len: 50, Required: true, JSON: "Description"},
		{Name: "Fixed / variable", Kind: Char, Len: 1, Required: true, Codes: []string{"F", "V"}, Hint: "F = fixed, V = variable", JSON: "FixedVariable"},
		{Name: "Unit type", Kind: Char, Len: 25, Hint: "e.g. Day, kWh, kVA", JSON: "Unit"},
		{Name: "Flow direction", Kind: Char, Len: 1, Codes: []string{"X", "I", "B"}, Hint: "X = consumption, I = generation, B = both", JSON: "FlowDirection"},
		{Name: "RCC-POA", Kind: Char, Len: 50, JSON: "RCC-POA"},
		{Name: "Schedule identifiers", Kind: IdentList, Len: 200, Hint: "SCHEDULE identifiers separated by spaces", JSON: "ScheduleIds"},
		attributeIDs(256),
	},
	"TARIFFREGION": {
		recordType("TARIFFREGION"),
		{Name: "Tariff region identifier", Kind: Ident, Len: 20, Required: true, JSON: "TariffRegionId"},
		{Name: "Description", Kind: Char, Len: 50, JSON: "Description"},
	},
	"NETWORK": {
		recordType("NETWORK"),
		{Name: "Distributor participant identifier", Kind: Char, Len: 4, JSON: "Network"},
		{Name: "NSP", Kind: Char, Len: 200, JSON: "NSP"},
		{Name: "Distributor price category code", Kind: Char, Len: 200, JSON: "DistributorPriceCategory"},
		{Name: "Distributor loss category code", Kind: Char, Len: 50, JSON: "DistributorLossCategory"},
	},
	"CUSTOMER": {
		recordType("CUSTOMER"),
		{Name: "Customer text", Kind: Char, Len: 1000, JSON: "CustomerGroup"},
		attributeIDs(200),
	},
	"PLAN": {
		recordType("PLAN"),
		{Name: "Product identification code", Kind: Char, Len: 32, Required: true, JSON: "PlanId"},
		{Name: "Plan description", Kind: Char, Len: 50, Required: true, Hint: "unique across all retailers' plans", JSON: "Description"},
		{Name: "Start date", Kind: ISODate, JSON: "StartDate"},
		{Name: "End date", Kind: ISODate, JSON: "EndDate"},
		{Name: "Close date", Kind: ISODate, JSON: "CloseDate"},
		{Name: "Low fixed charge", Kind: Char, Len: 1, Required: true, Codes: []string{"Y", "N"}, JSON: "LowFixedCharge"},
		attributeIDs(256),
	},
	"TARIFF": {
		recordType("TARIFF"),
		{Name: "Tariff identifier", Kind: Char, Len: 50, JSON: "Tariff"},
		{Name: "Tariff region identifier", Kind: Ident, Len: 20, Hint: "blank if the rate is common across all networks", JSON: "TariffRegionId"},
		{Name: "Tariff type identifier", Kind: Ident, Len: 20, Required: true, JSON: "TariffTypeId"},
		{Name: "Tariff rate", Kind: Num, Len: 12, Dec: 6, Required: true, JSON: "Rate"},
		attributeIDs(256),
	},
}

// jsonChildren lists the JSON arrays holding each record type's child records, in CSV order.
var jsonChildren = map[string][]struct{ key, typ string }{
	"HDR":          {{"Retailers", "RETAILER"}},
	"RETAILER":     {{"Attributes", "ATTRIBUTE"}, {"Schedules", "SCHEDULE"}, {"TariffTypes", "TARIFFTYPE"}, {"TariffRegions", "TARIFFREGION"}, {"CustomerGroups", "CUSTOMER"}},
	"TARIFFREGION": {{"Networks", "NETWORK"}},
	"CUSTOMER":     {{"Plans", "PLAN"}},
	"PLAN":         {{"Tariffs", "TARIFF"}},
}

// jsonAliases are element names used by the spec's JSON examples instead of those in its tables.
var jsonAliases = map[string]map[string]string{
	"PLAN":   {"Plan": "PlanId"},
	"TARIFF": {"TariffRegion": "TariffRegionId", "TariffType": "TariffTypeId"},
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
	ok := func(i int) bool { c, _ := fs[i].check(v[i]); return v[i] != "" && (c == "" || warningCodes[c]) }
	add := func(i int, code, msg string) { p = append(p, Problem{fs[i].Name, code, msg}) }

	switch typ {
	case "SCHEDULE":
		p = append(p, checkSchedule(fs[2:], v[2:])...)
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
		p = append(p, checkPlanDates(fs, v, parseISODate)...)
	}
	return p
}

// checkSchedule checks a SCHEDULE record from its daylight savings field on.
func checkSchedule(fs []Field, v []string) []Problem {
	var p []Problem
	ok := func(i int) bool { c, _ := fs[i].check(v[i]); return v[i] != "" && (c == "" || warningCodes[c]) }
	add := func(i int, code, msg string) { p = append(p, Problem{fs[i].Name, code, msg}) }
	if (v[1] != "" || v[2] != "") && v[0] == "" {
		add(0, "blank", "mandatory when the schedule has a start or end time; fill it in"+fs[0].hint())
	}
	start, _ := parseClock(v[1])
	end, _ := parseClock(v[2])
	if ok(1) && start == 24*60 {
		add(1, "bad_time", `a start time cannot be 24:00; use "00:00"`)
	}
	if ok(1) && ok(2) && end <= start {
		add(2, "end_before_start", fmt.Sprintf("%q is not after the start %q; an active period cannot span midnight, so split it into two SCHEDULE records (e.g. 22:00-24:00 and 00:00-06:00)", v[2], v[1]))
	}
	if ok(3) && !validSet(v[3], dayTypes, dayNames) {
		add(3, "bad_code", fmt.Sprintf("%q is not a valid day type; use one of %s, or day names separated by single spaces, e.g. \"MON TUE WED\"", v[3], strings.Join(dayTypes, ", ")))
	}
	if ok(4) && !validSet(v[4], []string{"ANY"}, monthName) {
		add(4, "bad_code", fmt.Sprintf("%q is not a valid month; use ANY, or three-letter month names separated by single spaces, e.g. \"JUN JUL AUG\"", v[4]))
	}
	return p
}

// checkPlanDates checks a PLAN record's start (3), end (4) and close (5) dates.
func checkPlanDates(fs []Field, v []string, parse func(string) (time.Time, error)) []Problem {
	var p []Problem
	ok := func(i int) bool { c, _ := fs[i].check(v[i]); return v[i] != "" && (c == "" || warningCodes[c]) }
	add := func(i int, code, msg string) { p = append(p, Problem{fs[i].Name, code, msg}) }
	start, _ := parse(v[3])
	end, _ := parse(v[4])
	closed, _ := parse(v[5])
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
	want, what := f.Lines-1, "lines after the header"
	if f.JSON {
		want, what = 0, "tariffs"
		for _, r := range f.Records {
			if r.Type == "TARIFF" {
				want++
			}
		}
	}
	if n, err := strconv.Atoi(f.Header[hdrRecordCount]); err == nil && n != want && (f.JSON || n != f.Lines) {
		out = append(out, Finding{1, eiep14aHeader[hdrRecordCount].Name, "count_mismatch", fmt.Sprintf("declares %d but the file has %d %s; set it to %d", n, want, what, want)})
	}
	if len(f.Records) == 0 {
		return append(out, Finding{1, "", "structure", "file has no RETAILER record; add the retailer and its plans after the header"})
	}

	var retailers [][]Record
	var prev string
	var customer, plan bool
	structure := func(r Record, msg string) { out = append(out, r.finding("", "structure", msg)) }
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

	descriptions := map[string][]planKey{}
	for _, recs := range retailers {
		out = append(out, checkRetailer(recs, descriptions)...)
	}
	return out
}

// checkRetailer checks identifiers are unique and references resolve within one retailer.
// descriptions collects plan descriptions across retailers, which must also be unique.
func checkRetailer(recs []Record, descriptions map[string][]planKey) []Finding {
	var out []Finding
	valid := func(r Record, i int) bool {
		fs := eiep14aRecords[r.Type]
		if len(r.V) != len(fs) || r.V[i] == "" {
			return false
		}
		c, _ := fs[i].check(r.V[i])
		return c == "" || warningCodes[c]
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
			out = append(out, r.finding(eiep14aRecords[r.Type][i].Name, "duplicate_id", fmt.Sprintf("%q is already used by the %s record on line %d; identifiers must be unique within the retailer, so rename one of them", r.V[i], r.Type, line)))
			continue
		}
		ids[r.Type][id] = r.Line
	}

	out = append(out, checkPlanDescriptions(recs, valid, descriptions)...)
	for _, r := range recs {
		for _, ref := range references {
			if ref.typ != r.Type || !valid(r, ref.field) {
				continue
			}
			for _, id := range strings.Split(r.V[ref.field], " ") {
				if _, ok := ids[ref.target][strings.ToUpper(id)]; !ok {
					out = append(out, r.finding(eiep14aRecords[r.Type][ref.field].Name, "unknown_reference", fmt.Sprintf("%q does not match any %s record under this RETAILER; add one with that identifier, or correct the reference", id, ref.target)))
				}
			}
		}
	}
	return out
}

type planKey struct {
	key  string
	line int
}

// checkPlanDescriptions reports a plan sharing its description with an earlier one: an error when the start date
// and the networks of its tariffs' regions also match, otherwise a warning, as they are likely different products.
func checkPlanDescriptions(recs []Record, valid func(Record, int) bool, descriptions map[string][]planKey) []Finding {
	networks := map[string][]string{}
	var region string
	for _, r := range recs {
		switch {
		case r.Type == "TARIFFREGION" && len(r.V) > 1:
			region = strings.ToUpper(r.V[1])
		case r.Type == "NETWORK":
			networks[region] = append(networks[region], strings.ToUpper(strings.Join(r.V[1:], ",")))
		}
	}

	type plan struct {
		r       Record
		regions map[string]bool
	}
	var plans []*plan
	var cur *plan
	for _, r := range recs {
		switch r.Type {
		case "PLAN":
			cur = &plan{r, map[string]bool{}}
			plans = append(plans, cur)
		case "CUSTOMER":
			cur = nil
		case "TARIFF":
			if cur != nil && len(r.V) > 2 {
				cur.regions[strings.ToUpper(r.V[2])] = true
			}
		}
	}

	var out []Finding
	for _, p := range plans {
		if !valid(p.r, 2) {
			continue
		}
		var nets []string
		for id := range p.regions {
			if ns, ok := networks[id]; ok {
				nets = append(nets, ns...)
			} else {
				nets = append(nets, "region "+id)
			}
		}
		slices.Sort(nets)
		key := strings.ToUpper(p.r.V[3]) + "|" + strings.Join(slices.Compact(nets), "|")
		d := strings.ToUpper(p.r.V[2])
		earlier := descriptions[d]
		descriptions[d] = append(earlier, planKey{key, p.r.Line})
		if len(earlier) == 0 {
			continue
		}
		f := p.r.finding("Plan description", "shared_description", fmt.Sprintf("%q is also used by the plan on line %d; the spec requires each plan's description to be unique across all retailers' plans", p.r.V[2], earlier[0].line))
		for _, e := range earlier {
			if e.key == key {
				f.Code, f.Msg = "duplicate_description", fmt.Sprintf("%q is already used by the plan on line %d with the same start date and networks; each plan's description must be unique across all retailers' plans", p.r.V[2], e.line)
				break
			}
		}
		out = append(out, f)
	}
	return out
}

// summarisePlans counts records and spans PLAN start (3) and end (4) dates, written in layout.
func summarisePlans(f File, parse func(string) (time.Time, error), layout string) string {
	n := map[string]int{}
	var first, last time.Time
	for _, r := range f.Records {
		n[r.Type]++
		if r.Type != "PLAN" {
			continue
		}
		if d, err := parse(r.V[3]); err == nil && (first.IsZero() || d.Before(first)) {
			first = d
		}
		if d, err := parse(r.V[4]); err == nil && d.After(last) {
			last = d
		}
	}
	var from, to string
	if !first.IsZero() {
		from = first.Format(layout)
	}
	if !last.IsZero() {
		to = last.Format(layout)
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
