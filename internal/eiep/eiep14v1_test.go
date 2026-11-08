package eiep

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// base14v1 is spec Example 3 abridged, with its TARRIF typos and NZDT daylight savings values corrected.
var base14v1 = []string{
	"HDR,PRCSCHD,1,ASRL,ASRL,ANY,23/05/2016,01:23:45,12345,%d,E,I",
	"RETAILER,ASRL,ASRL,Simple Retailer Ltd",
	"CUSTOMER,General domestic",
	"ATTRIBUTE,ANZSIC,,,0000000",
	"ATTRIBUTE,DISCOUNT_PROMPT_PAYMENT,10,%,",
	"PLAN,PRSU,Residential - standard user,01/04/2016,,30/06/2016,N",
	"NETWORK,CKHK,WIL0331CKHK CPK0331CKHK CPK0111CKHK,RSU,LCC1",
	"TARIFF,RSU-FIXED,Daily supply charge,F,2.1560,Day,B,Fixed,",
	"TARIFF,RSU-EVNITE,Electric vehicle - night only,V,0.1337,kWh,X,UN,10",
	"SCHEDULE,1,21:00,24:00,,",
	"SCHEDULE,1,00:00,07:00,,",
	"ATTRIBUTE,FIXED_TERM_DATE,31/12/2016,,,",
}

// doc14v1 returns base14v1 with 1-based lines replaced (an empty value deletes the line),
// extra lines appended, and the header count set to match.
func doc14v1(set map[int]string, extra ...string) string {
	var ls []string
	for i, l := range base14v1 {
		if s, ok := set[i+1]; ok {
			if s == "" {
				continue
			}
			l = s
		}
		ls = append(ls, l)
	}
	ls = append(ls, extra...)
	if strings.Contains(ls[0], "%d") {
		ls[0] = fmt.Sprintf(ls[0], len(ls)-1)
	}
	return file(ls...)
}

func TestEIEP14v1(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []at
	}{
		{"valid", doc14v1(nil), nil},
		{"valid 11-field header", doc14v1(map[int]string{1: strings.Replace(base14v1[0], ",E,I", ",I", 1)}), nil},
		{"valid 10-field header", doc14v1(map[int]string{1: strings.Replace(base14v1[0], ",E,I", "", 1)}), nil},
		{"valid second plan", doc14v1(nil, "PLAN,PRLU,Residential - low user,,,,Y", "NETWORK,CKHK,,RLU,", "TARIFF,RLU-FIXED,Daily supply charge,F,0.3333,Day,B,Fixed,"), nil},
		{"valid shared description", doc14v1(nil, "PLAN,PRSU2,Residential - standard user,01/04/2016,,30/06/2016,N", "NETWORK,CKHK,WIL0331CKHK CPK0331CKHK CPK0111CKHK,RSU2,LCC1", "TARIFF,X,X,F,1,,,,"), nil},
		{"bad run time", doc14v1(map[int]string{1: strings.Replace(base14v1[0], "01:23:45", "1:23", 1)}), []at{{1, "Report run time"}}},
		{"count mismatch", strings.Replace(doc14v1(nil), ",11,E", ",12,E", 1), []at{{1, "Number of detailed records"}}},
		{"plan date order", doc14v1(map[int]string{6: "PLAN,PRSU,Residential - standard user,01/04/2016,,01/03/2016,N"}), []at{{6, "Close date"}}},
		{"schedule rules", doc14v1(map[int]string{10: "SCHEDULE,1,22:00,06:00,,"}), []at{{10, "End time"}}},
		{"tariff attribute above tariff", doc14v1(map[int]string{5: "ATTRIBUTE,DISCOUNT_PROMPT_PAYMENT,,10,%,"}), []at{{5, ""}}},
		{"plan attribute under tariff", doc14v1(map[int]string{12: "ATTRIBUTE,URL,,,x"}), []at{{12, ""}}},
		{"tariff without network", doc14v1(map[int]string{7: ""}), []at{{6, ""}, {7, ""}, {8, ""}}},
		{"plan without tariff", doc14v1(map[int]string{8: "", 9: "", 10: "", 11: "", 12: ""}), []at{{6, ""}}},
		{"customer without plan", doc14v1(nil, "CUSTOMER,Business"), []at{{13, ""}}},
		{"schedule not under tariff", doc14v1(map[int]string{8: "", 9: "", 12: ""}, "TARIFF,X,X,F,1,,,,"), []at{{8, ""}, {9, ""}}},
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

func TestEIEP14v1Summary(t *testing.T) {
	r := Check([]byte(doc14v1(nil)))
	if want := "11 records (retailers: 1, plans: 1, tariffs: 2), plans from 01/04/2016 to open"; len(r.Findings) > 0 || r.Summary != want || r.Format != "EIEP14 1" || len(r.Warnings) != 1 || r.Warnings[0].Code != "deprecated" {
		t.Errorf("got %q %q %v", r.Format, r.Summary, r.Findings)
	}
}
