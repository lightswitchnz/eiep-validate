package eiep

import (
	"os"
	"reflect"
	"testing"
)

// The spec's Appendix C examples, extracted verbatim; each declares the draft version 2.011.
func TestJSONSpecExamples(t *testing.T) {
	tests := []struct {
		file     string
		want     []at
		warnings int
	}{
		{"example1.json", []at{{10, "$.RecordCount"}, {30, "$.Retailers[0].Attributes[2].DateValue"}}, 4},
		{"example2.json", nil, 4},
		{"example3.json", nil, 4},
		{"example4.json", []at{{10, "$.RecordCount"}, {283, "$.Retailers[0].CustomerGroups[1].Plans[0].Tariffs[4].TariffRegion"}}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + tt.file)
			if err != nil {
				t.Fatal(err)
			}
			r := Check(data)
			var got []at
			for _, f := range r.Findings {
				got = append(got, at{f.Line, f.Field})
			}
			if !reflect.DeepEqual(got, tt.want) || len(r.Warnings) != tt.warnings || r.Format != "EIEP14A 2.011 JSON" {
				t.Errorf("got %v and %d warnings, want %v and %d\n%+v\n%+v", got, len(r.Warnings), tt.want, tt.warnings, r.Findings, r.Warnings)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []at
	}{
		{"no header", `{"Retailers": [{"RetailerBrandName": "R", "CustomerGroups": [{"Plans": [{"PlanId": "P", "Description": "D", "LowFixedCharge": "N",
			"Tariffs": [{"TariffTypeId": "T", "Rate": 1}]}]}],
			"TariffTypes": [{"TariffTypeId": "T", "Description": "Anytime", "FixedVariable": "V", "RCC-POA": [["UN", 24], ["CN", 19]]}]}]}`, nil},
		{"minified", `{"Retailers":[{"RetailerBrandName":"R","CustomerGroups":[{"Plans":[{"PlanId":"P","Description":"D","LowFixedCharge":"N","Tariffs":[{"TariffTypeId":"T","Rate":1}]}]}]}]}`,
			[]at{{1, "$.Retailers[0].CustomerGroups[0].Plans[0].Tariffs[0].TariffTypeId"}}},
		{"syntax error", "{\n\"Retailers\": [\n}", []at{{3, ""}}},
		{"not an object", "{\"Retailers\": [1]}", []at{{1, "$.Retailers[0]"}, {1, ""}}},
		{"unknown element and bad type", "{\"Retailers\": [{\n\"RetailerBrandName\": true,\n\"Plans\": []}]}", []at{{1, "$.Retailers[0]"}, {2, "$.Retailers[0].RetailerBrandName"}, {2, "$.Retailers[0].RetailerBrandName"}, {3, "$.Retailers[0].Plans"}}},
		{"unsupported version", `{"Version": 3, "Retailers": []}`, []at{{1, "$.Version"}}},
		{"tariff count", `{"RecordCount": 1, "Retailers": [{"RetailerBrandName": "R"}]}`, []at{{1, "$.RecordCount"}, {1, "$.Retailers[0]"}}},
		{"plan without tariffs", `{"Retailers": [{"RetailerBrandName": "R", "CustomerGroups": [{"Plans": [{"PlanId": "P", "Description": "D", "LowFixedCharge": "N", "Tariffs": []}]}]}]}`,
			[]at{{1, "$.Retailers[0].CustomerGroups[0].Plans[0]"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []at
			for _, f := range Validate([]byte(tt.in)) {
				got = append(got, at{f.Line, f.Field})
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v\nfindings: %+v", got, tt.want, Validate([]byte(tt.in)))
			}
		})
	}
}

func TestPlans(t *testing.T) {
	data, err := os.ReadFile("testdata/example4.json")
	if err != nil {
		t.Fatal(err)
	}
	plans := Check(data).Plans()
	var got []string
	for _, a := range plans[0].Attributes {
		got = append(got, a.Kind+" "+a.Code)
	}
	want := []string{"requirement BUNDLED_GAS", "requirement CREDIT_CHECK", "requirement METER_TYPE_NHH", "perk DISCOUNT_GAS", "customer ANZSIC"}
	if len(plans) != 3 || plans[0].ID != "ASRL_RLU_GAS" || !reflect.DeepEqual(got, want) {
		t.Errorf("got %d plans, %s %v, want 3, ASRL_RLU_GAS %v", len(plans), plans[0].ID, got, want)
	}
}
