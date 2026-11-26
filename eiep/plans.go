package eiep

import (
	"slices"
	"strings"
)

// Plan is an EIEP14A plan with the attributes that apply to it.
type Plan struct {
	Line        int         `json:"line"`
	Retailer    string      `json:"retailer"`
	Customer    string      `json:"customer"`
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Attributes  []Attribute `json:"attributes"`
}

type Attribute struct {
	Kind string `json:"kind"`
	// Level is where it is attached: customer, plan or tariff.
	Level       string `json:"level"`
	Code        string `json:"code"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
}

var kindOrder = []string{"requirement", "suited to", "perk", "cost", "customer", "info", "other"}

// attributeKinds groups the spec's attribute codes; a code ending in _ matches as a prefix.
var attributeKinds = map[string][]string{
	"requirement": {"BUNDLED_GAS", "BUNDLED_OTHER", "METER_TYPE_", "CREDIT_CHECK", "PREPAID_ONLY", "ELECTRONIC_BILL_ONLY", "KWH_MIN", "KWH_MAX", "PHASES", "FUSE_SIZE_", "KVA_MIN", "KVA_MAX"},
	"suited to":   {"ELECTRIC_VEHICLE_PLAN", "SOLAR_GENERATION_PLAN", "BATTERY_STORAGE_PLAN", "OTHER_GENERATION_PLAN", "GENERATION_ONLY"},
	"perk":        {"SIGNON_BONUS", "DISCOUNT_", "FREE_HOURS_", "CREDIT_CARD_ALLOWED"},
	"cost":        {"EXIT_FEE", "FIXED_TERM_", "FIXED_PRICE_", "CLAW BACK", "SIGNON_FEE", "SIGNON_BOND", "CREDIT_CARD_FEE", "DISCONNECTION_FEE_", "RECONNECTION_FEE", "LATE_PAYMENT_FEE", "PAPER_BILL_FEE", "SURCHARGE"},
	"customer":    {"ANZSIC"},
	"info":        {"TERMS_AND_CONDITIONS", "URL", "LOSS_ADJUSTMENT", "REPRESENTATIVE_TARIFFS", "WHOLESALE_REFERENCE_NODE"},
	"dates":       {"TARIFF_START_DATE", "TARIFF_END_DATE", "TARIFF_CLOSE_DATE"},
}

func attributeKind(code string) string {
	code = strings.ToUpper(code)
	for kind, codes := range attributeKinds {
		for _, c := range codes {
			if code == c || strings.HasSuffix(c, "_") && strings.HasPrefix(code, c) {
				return kind
			}
		}
	}
	return "other"
}

// eiep14aPlans lists each plan with its customer's, its own and its tariffs' attributes, leaving out tariff dates.
func eiep14aPlans(f File) []Plan {
	var plans []Plan
	attrs := map[string]Record{}
	var retailer, customer string
	var inherited []Attribute
	add := func(as []Attribute, level, ids string) []Attribute {
		for _, id := range strings.Fields(ids) {
			r, ok := attrs[strings.ToUpper(id)]
			if !ok {
				continue
			}
			a := Attribute{attributeKind(r.V[2]), level, r.V[2], or(r.V[3], or(r.V[4], r.V[5])), r.V[6]}
			if a.Kind == "dates" {
				continue
			}
			dup := false
			for _, b := range as {
				dup = dup || strings.EqualFold(b.Code, a.Code) && b.Value == a.Value && b.Description == a.Description
			}
			if !dup {
				as = append(as, a)
			}
		}
		return as
	}
	for _, r := range f.Records {
		if len(r.V) != len(eiep14aRecords[r.Type]) {
			continue
		}
		switch r.Type {
		case "RETAILER":
			retailer, attrs = r.V[3], map[string]Record{}
		case "ATTRIBUTE":
			attrs[strings.ToUpper(r.V[1])] = r
		case "CUSTOMER":
			customer, inherited = r.V[1], add(nil, "customer", r.V[2])
		case "PLAN":
			plans = append(plans, Plan{r.Line, retailer, customer, r.V[1], r.V[2], add(append([]Attribute(nil), inherited...), "plan", r.V[7])})
		case "TARIFF":
			if len(plans) > 0 {
				p := &plans[len(plans)-1]
				p.Attributes = add(p.Attributes, "tariff", r.V[5])
			}
		}
	}
	for _, p := range plans {
		slices.SortStableFunc(p.Attributes, func(a, b Attribute) int {
			return slices.Index(kindOrder, a.Kind) - slices.Index(kindOrder, b.Kind)
		})
	}
	return plans
}
