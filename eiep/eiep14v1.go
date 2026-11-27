package eiep

import (
	"fmt"
	"strconv"
	"strings"
)

// EIEP 14 v1.0: Retailer tariff rate notification (PRCSCHD version 1), 20 September 2016.
// Records nest by position: RETAILER > CUSTOMER > PLAN > NETWORK, TARIFF > SCHEDULE,
// with ATTRIBUTE rows applying to the CUSTOMER, PLAN or TARIFF above them.
var eiep14v1 = Layout{
	Name:        "EIEP14",
	Header:      eiep14v1Header,
	Records:     eiep14v1Records,
	Alt:         map[string][][]Field{"HDR": {eiep14v1Header[:10], append(eiep14v1Header[:10:10], eiep14v1Header[11])}, "ATTRIBUTE": {eiep14v1TariffAttribute}},
	Quoted:      true,
	CheckRecord: checkEIEP14v1Record,
	CheckFile:   checkEIEP14v1File,
	Summary:     func(f File) string { return summarisePlans(f, parseDate, dateLayout) },
}

var eiep14v1Header = []Field{
	{Name: "Header record type", Kind: Char, Len: 3, Required: true, Codes: []string{"HDR"}},
	{Name: "File type", Kind: Char, Len: 7, Required: true, Codes: []string{"PRCSCHD"}},
	{Name: "Version", Kind: Num, Len: 4, Dec: 1, Required: true},
	{Name: "Sender", Kind: Char, Len: 20, Required: true, Hint: "participant identifier if the sender is a participant"},
	{Name: "Sent on behalf of", Kind: Char, Len: 4, Required: true, Hint: "participant identifier of the party the data is provided for"},
	{Name: "Recipient", Kind: Char, Len: 4, Hint: "party code, ANY for multiple recipients, or blank if none is assigned"},
	{Name: "Report run date", Kind: Date, Required: true},
	{Name: "Report run time", Kind: Time, Required: true},
	{Name: "Unique file identifier", Kind: Num, Len: 15, Required: true},
	{Name: "Number of detailed records", Kind: Num, Len: 8, Required: true, Hint: "the number of lines after the header"},
	{Name: "Utility type", Kind: Char, Len: 1, Required: true, Codes: []string{"E"}, Hint: "E = electricity"},
	{Name: "File status", Kind: Char, Len: 1, Required: true, Codes: []string{"I", "R", "X"}, Hint: "I = initial, R = replacement, X = replaces only the tariffs in this file"},
}

var eiep14v1TariffAttribute = []Field{
	recordType("ATTRIBUTE"),
	{Name: "Attribute code", Kind: Char, Len: 50},
	{Name: "Date value", Kind: Date},
	{Name: "Numerical value", Kind: Num, Len: 26, Dec: 6},
	{Name: "Text value", Kind: Char, Len: 2550},
	{Name: "Description", Kind: Char, Len: 1000},
}

var eiep14v1Records = map[string][]Field{
	"RETAILER": {
		recordType("RETAILER"),
		{Name: "Trader participant identifier", Kind: Char, Len: 4, Hint: "mandatory if the Authority has assigned one"},
		{Name: "Retailer participant identifier", Kind: Char, Len: 4},
		{Name: "Retailer name", Kind: Char, Len: 50, Required: true, Hint: "brand name"},
	},
	"CUSTOMER": {
		recordType("CUSTOMER"),
		{Name: "Customer text", Kind: Char, Len: 1000},
	},
	"ATTRIBUTE": {
		recordType("ATTRIBUTE"),
		{Name: "Attribute code", Kind: Char, Len: 50},
		{Name: "Numerical value", Kind: Num, Len: 26, Dec: 6},
		{Name: "Text value", Kind: Char, Len: 2550},
		{Name: "Description", Kind: Char, Len: 1000},
	},
	"PLAN": {
		recordType("PLAN"),
		{Name: "Retailers plan code", Kind: Char, Len: 50},
		{Name: "Retailers plan description", Kind: Char, Len: 50, Required: true, Hint: "unique to the plan"},
		{Name: "Start date of plan", Kind: Date},
		{Name: "End date of plan", Kind: Date},
		{Name: "Close date", Kind: Date},
		{Name: "Low fixed charge", Kind: Char, Len: 1, Required: true, Codes: []string{"Y", "N"}},
	},
	"NETWORK": {
		recordType("NETWORK"),
		{Name: "Distributor participant identifier", Kind: Char, Len: 4},
		{Name: "NSP", Kind: Char, Len: 200},
		{Name: "Distributor price category code", Kind: Char, Len: 200},
		{Name: "Distributor loss category code", Kind: Char, Len: 50},
	},
	"TARIFF": {
		recordType("TARIFF"),
		{Name: "Retailers tariff identifier", Kind: Char, Len: 50},
		{Name: "Tariff name", Kind: Char, Len: 50, Required: true},
		{Name: "Fixed / variable", Kind: Char, Len: 1, Required: true, Codes: []string{"F", "V"}, Hint: "F = fixed, V = variable"},
		{Name: "Tariff rate", Kind: Num, Len: 12, Dec: 6, Required: true},
		{Name: "Unit type", Kind: Char, Len: 25, Hint: "e.g. Day, kWh, kVA"},
		{Name: "Flow direction", Kind: Char, Len: 1, Codes: []string{"X", "I", "B"}, Hint: "X = consumption, I = generation, B = both"},
		{Name: "Register content code", Kind: Char, Len: 6},
		{Name: "Period of availability", Kind: Num, Len: 3, Dec: 1},
	},
	"SCHEDULE": {
		recordType("SCHEDULE"),
		{Name: "Daylight savings adjustment", Kind: Char, Len: 1, Codes: []string{"1", "2"}, Hint: "1 = adjusted to NZDT, 2 = remains NZST"},
		{Name: "Start time", Kind: Clock},
		{Name: "End time", Kind: Clock},
		{Name: "Day type", Kind: Char, Len: 50},
		{Name: "Month", Kind: Char, Len: 50},
	},
}

func checkEIEP14v1Record(typ string, v []string) []Problem {
	fs := eiep14v1Records[typ]
	if typ == "ATTRIBUTE" && len(v) == len(eiep14v1TariffAttribute) {
		fs = eiep14v1TariffAttribute
	}
	p := checkFields(fs, v)
	switch typ {
	case "SCHEDULE":
		p = append(p, checkSchedule(fs[1:], v[1:])...)
	case "PLAN":
		p = append(p, checkPlanDates(fs, v, parseDate)...)
	}
	return p
}

func checkEIEP14v1File(f File) []Finding {
	out := []Finding{{1, "Version", "deprecated", "EIEP14 is superseded by EIEP14A (version 2) from 30 October 2026; move to the EIEP14A format"}}
	if n, err := strconv.Atoi(f.Header[hdrRecordCount]); err == nil && n != len(f.Records) {
		out = append(out, Finding{1, eiep14v1Header[hdrRecordCount].Name, "count_mismatch", fmt.Sprintf("declares %d but the file has %d lines after the header; set it to %d", n, len(f.Records), len(f.Records))})
	}
	structure := func(line int, msg string) { out = append(out, Finding{line, "", "structure", msg}) }
	if len(f.Records) == 0 {
		structure(1, "file has no RETAILER record; add the retailer and its plans after the header")
		return out
	}

	// Lines of the current parents; 0 when there is none.
	var retailer, customer, plan, network, tariff int
	var plans, networks, tariffs int
	// A plan matching an earlier one on description, start date and networks is likely a duplicate.
	descriptions := map[string]int{}
	var desc, key string
	closePlan := func() {
		if plan != 0 && desc != "" {
			if line, dup := descriptions[key]; dup {
				out = append(out, Finding{plan, "Retailers plan description", "duplicate_plan", fmt.Sprintf("%q matches the plan on line %d with the same start date and networks; give each plan its own description, or remove the duplicate", desc, line)})
			} else {
				descriptions[key] = plan
			}
		}
		switch {
		case plan == 0:
		case networks == 0:
			structure(plan, "PLAN has no NETWORK record; add one for each network the plan applies to, directly after the plan's attributes")
		case tariffs == 0:
			structure(plan, "PLAN has no TARIFF records; add the plan's rates after its NETWORK records")
		}
		plan, network, tariff, networks, tariffs, desc = 0, 0, 0, 0, 0, ""
	}
	closeCustomer := func() {
		closePlan()
		if customer != 0 && plans == 0 {
			structure(customer, "CUSTOMER has no PLAN records; add the plans for this customer type after it")
		}
		customer, plans = 0, 0
	}
	for _, r := range f.Records {
		switch r.Type {
		case "RETAILER":
			closeCustomer()
			retailer = r.Line
			descriptions = map[string]int{}
		case "CUSTOMER":
			closeCustomer()
			if retailer == 0 {
				structure(r.Line, "CUSTOMER record has no RETAILER record above it; add the RETAILER first")
			}
			customer = r.Line
		case "PLAN":
			closePlan()
			if customer == 0 {
				structure(r.Line, "PLAN record has no CUSTOMER record above it; plans belong to the customer type above them, so add a CUSTOMER record first")
			}
			plan, plans = r.Line, plans+1
			if len(r.V) > 3 {
				desc, key = r.V[2], strings.ToUpper(r.V[2]+"|"+r.V[3])
			}
		case "NETWORK":
			switch {
			case plan == 0:
				structure(r.Line, "NETWORK record has no PLAN record above it; move it under its PLAN")
			case tariff != 0:
				structure(r.Line, "NETWORK record comes after the plan's TARIFF records; move it above the first TARIFF")
			}
			network, networks = r.Line, networks+1
			key += "|" + strings.ToUpper(strings.Join(r.V, ","))
		case "TARIFF":
			switch {
			case plan == 0:
				structure(r.Line, "TARIFF record has no PLAN record above it; tariffs belong to the plan above them, so move it under its PLAN")
			case network == 0:
				structure(r.Line, "TARIFF record comes before any NETWORK record of its plan; add the plan's NETWORK records above its tariffs")
			}
			tariff, tariffs = r.Line, tariffs+1
		case "SCHEDULE":
			if tariff == 0 {
				structure(r.Line, "SCHEDULE record is not under a TARIFF; schedules limit the tariff above them, so move it directly after its TARIFF")
			}
		case "ATTRIBUTE":
			switch tariffAttr := len(r.V) == len(eiep14v1TariffAttribute); {
			case customer == 0:
				structure(r.Line, "ATTRIBUTE record has no CUSTOMER, PLAN or TARIFF record above it; move it under the record it applies to")
			case tariff != 0 && !tariffAttr && r.V != nil:
				out = append(out, Finding{r.Line, "", "field_count", fmt.Sprintf("ATTRIBUTE under a TARIFF must have %d fields, with a Date value after the Attribute code.%s", len(eiep14v1TariffAttribute), expectedGot(eiep14v1TariffAttribute, r.V))})
			case tariff == 0 && tariffAttr:
				out = append(out, Finding{r.Line, "", "field_count", fmt.Sprintf("ATTRIBUTE of a CUSTOMER or PLAN must have %d fields (only tariff attributes have a Date value).%s", len(eiep14v1Records["ATTRIBUTE"]), expectedGot(eiep14v1Records["ATTRIBUTE"], r.V))})
			}
		}
	}
	closeCustomer()
	return out
}
