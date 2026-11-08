package eiep

import (
	"os"
	"reflect"
	"testing"
)

// The spec's Appendix C examples, extracted verbatim.
func TestJSONSpecExamples(t *testing.T) {
	tests := []struct {
		file     string
		want     []at
		warnings int
	}{
		{"example1.json", []at{{10, "$.RecordCount"}, {30, "$.Retailers[0].Attributes[2].DateValue"}}, 5},
		{"example2.json", nil, 9},
		{"example3.json", nil, 3},
		{"example4.json", []at{{10, "$.RecordCount"}, {283, "$.Retailers[0].CustomerGroups[1].Plans[0].Tariffs[4].TariffRegion"}}, 3},
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
		{"unknown element and bad type", "{\"Retailers\": [{\n\"RetailerBrandName\": true,\n\"Plans\": []}]}", []at{{2, "$.Retailers[0].RetailerBrandName"}, {2, "$.Retailers[0].RetailerBrandName"}, {3, "$.Retailers[0].Plans"}}},
		{"unsupported version", `{"Version": 1, "Retailers": []}`, []at{{1, "$.Version"}}},
		{"tariff count", `{"RecordCount": 1, "Retailers": [{"RetailerBrandName": "R"}]}`, []at{{1, "$.RecordCount"}}},
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
