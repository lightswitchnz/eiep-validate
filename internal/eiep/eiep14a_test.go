package eiep

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// base14 is spec Example 1 abridged, with a schedule added and its invalid 2025-04-31 corrected.
var base14 = []string{
	"HDR,PRCSCHD,1.0,ASRL,ASRL,ANY,2016-05-22T12:23:45+12,,c7d90f3a-d62b-4923-95c4-4aef120d831e,%d,E,I",
	"RETAILER,ASRL,ASRL,Simple Retailer Ltd",
	"ATTRIBUTE,AT_anzsic_000000,ANZSIC,,,000000,",
	"ATTRIBUTE,AT_START_20250401,TARIFF_START_DATE,2025-04-01,,,",
	"ATTRIBUTE,AT_END_20250331,TARIFF_END_DATE,2025-03-31,,,",
	"SCHEDULE,SC_night,1,21:00,24:00,ANY,ANY",
	"TARIFFREGION,TR_CKHK_RSU,",
	"NETWORK,CKHK,,RSU,",
	"TARIFFTYPE,TT_dly_sup_chg,Daily supply charge,F,Day,B,,,",
	"TARIFFTYPE,TT_any,Anytime,V,kWh,X,UN-24,SC_night,",
	"CUSTOMER,General domestic,AT_anzsic_000000",
	"PLAN,PRSU,Residential - standard user,,,,N,",
	"TARIFF,RSU-FIXED,TR_CKHK_RSU,TT_dly_sup_chg,2.053,AT_END_20250331",
	"TARIFF,RSU-24UN,TR_CKHK_RSU,TT_any,0.1937,",
}

// doc14 returns base14 with 1-based lines replaced (an empty value deletes the line),
// extra lines appended, and the header count set to match.
func doc14(set map[int]string, extra ...string) string {
	var ls []string
	for i, l := range base14 {
		if s, ok := set[i+1]; ok {
			if s == "" {
				continue
			}
			l = s
		}
		ls = append(ls, l)
	}
	ls = append(ls, extra...)
	if !strings.Contains(ls[0], "%d") {
		return file(ls...)
	}
	ls[0] = fmt.Sprintf(ls[0], len(ls))
	return file(ls...)
}

func TestEIEP14A(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []at
	}{
		{"valid", doc14(nil), nil},
		{"valid count excluding header", strings.Replace(doc14(nil), ",14,E", ",13,E", 1), nil},
		{"valid CRLF", strings.ReplaceAll(doc14(nil), "\n", "\r\n"), nil},
		{"valid draft version 2.0", doc14(map[int]string{1: strings.Replace(base14[0], "1.0", "2.0", 1)}), nil},
		{"valid version 1", doc14(map[int]string{1: strings.Replace(base14[0], "1.0", "1", 1)}), nil},
		{"valid quoted comma", doc14(map[int]string{2: `RETAILER,ASRL,ASRL,"Simple Retailer, ""Ltd"""`}), nil},
		{"valid lowercase", doc14(map[int]string{8: "network,CKHK,,RSU,", 12: "plan,PRSU,Residential - standard user,,,,n,"}), nil},
		{"valid day names", doc14(map[int]string{6: "SCHEDULE,SC_night,2,00:00,07:00,MON TUE,JUN JUL"}), nil},
		{"valid all-day schedule", doc14(map[int]string{6: "SCHEDULE,SC_night,,,,WE,"}), nil},
		{"valid plan dates", doc14(map[int]string{12: "PLAN,PRSU,Residential - standard user,2016-04-01,2016-12-31,2016-06-30,N,"}), nil},
		{"valid all networks", doc14(map[int]string{14: "TARIFF,RSU-24UN,,TT_any,0.1937,"}), nil},
		{"valid second retailer", doc14(nil, "RETAILER,BRTL,,Other", "TARIFFTYPE,TT_any,Anytime,V,kWh,X,,,", "CUSTOMER,,", "PLAN,PRSU,Other plan,,,,N,", "TARIFF,,,TT_any,0.2,"), nil},

		{"unsupported version", doc14(map[int]string{1: strings.Replace(base14[0], "1.0", "3.0", 1)}), []at{{1, "Version"}}},
		{"count mismatch", strings.Replace(doc14(nil), ",14,E", ",26,E", 1), []at{{1, "Number of detailed records"}}},
		{"run date without offset", doc14(map[int]string{1: strings.Replace(base14[0], "+12", "", 1)}), []at{{1, "Report run date and time"}}},
		{"bad UUID", doc14(map[int]string{1: strings.Replace(base14[0], "c7d90f3a-", "c7d90f3a", 1)}), []at{{1, "Unique file identifier"}}},
		{"bad utility type", doc14(map[int]string{1: strings.Replace(base14[0], ",E,I", ",W,I", 1)}), []at{{1, "Utility type"}}},
		{"unclosed quote", doc14(map[int]string{2: `RETAILER,ASRL,ASRL,"Simple`}), []at{{2, ""}}},
		{"text after quote", doc14(map[int]string{2: `RETAILER,ASRL,ASRL,"Simple" Ltd`}), []at{{2, ""}}},
		{"bare quote", doc14(map[int]string{2: `RETAILER,ASRL,ASRL,Simple "Ltd"`}), []at{{2, ""}}},
		{"unknown record", doc14(nil, "TARIFFGROUP,x"), []at{{15, ""}}},
		{"missing trailing field", doc14(map[int]string{14: "TARIFF,RSU-24UN,TR_CKHK_RSU,TT_any,0.1937"}), []at{{14, ""}}},
		{"impossible date", doc14(map[int]string{5: "ATTRIBUTE,AT_END_20250331,TARIFF_END_DATE,2025-04-31,,,"}), []at{{5, "Date value"}}},
		{"spaced date", doc14(map[int]string{12: "PLAN,PRSU,Residential - standard user, 2016-04-01,,,N,"}), []at{{12, "Start date"}}},
		{"identifier with space", doc14(map[int]string{3: "ATTRIBUTE,AT anzsic,ANZSIC,,,000000,"}), []at{{3, "Attribute record identifier"}, {11, "Attribute identifiers"}}},
		{"double space in list", doc14(map[int]string{11: "CUSTOMER,General domestic,AT_anzsic_000000  AT_START_20250401"}), []at{{11, "Attribute identifiers"}}},
		{"comma list", doc14(map[int]string{11: `CUSTOMER,General domestic,"AT_anzsic_000000,AT_START_20250401"`}), []at{{11, "Attribute identifiers"}}},
		{"rate too many digits", doc14(map[int]string{14: "TARIFF,RSU-24UN,TR_CKHK_RSU,TT_any,1234567,"}), []at{{14, "Tariff rate"}}},
		{"bad fixed/variable", doc14(map[int]string{9: "TARIFFTYPE,TT_dly_sup_chg,Daily supply charge,X,Day,B,,,"}), []at{{9, "Fixed / variable"}}},
		{"bad RCC-POA", doc14(map[int]string{10: "TARIFFTYPE,TT_any,Anytime,V,kWh,X,UN24,SC_night,"}), []at{{10, "RCC-POA"}}},
		{"RCC-POA over 24", doc14(map[int]string{10: "TARIFFTYPE,TT_any,Anytime,V,kWh,X,UN-25,SC_night,"}), []at{{10, "RCC-POA"}}},
		{"bad time", doc14(map[int]string{6: "SCHEDULE,SC_night,1,21.00,24:00,ANY,ANY"}), []at{{6, "Start time"}}},
		{"spans midnight", doc14(map[int]string{6: "SCHEDULE,SC_night,1,22:00,06:00,ANY,ANY"}), []at{{6, "End time"}}},
		{"start 24:00", doc14(map[int]string{6: "SCHEDULE,SC_night,1,24:00,24:00,ANY,ANY"}), []at{{6, "Start time"}, {6, "End time"}}},
		{"times without DST flag", doc14(map[int]string{6: "SCHEDULE,SC_night,,21:00,24:00,ANY,ANY"}), []at{{6, "Daylight savings adjustment"}}},
		{"bad day type", doc14(map[int]string{6: "SCHEDULE,SC_night,1,21:00,24:00,MONDAY,ANY"}), []at{{6, "Day type"}}},
		{"mixed day type", doc14(map[int]string{6: "SCHEDULE,SC_night,1,21:00,24:00,WD SAT,ANY"}), []at{{6, "Day type"}}},
		{"bad month", doc14(map[int]string{6: "SCHEDULE,SC_night,1,21:00,24:00,ANY,JANUARY"}), []at{{6, "Month"}}},
		{"plan ends before start", doc14(map[int]string{12: "PLAN,PRSU,Residential - standard user,2016-04-01,2016-03-01,,N,"}), []at{{12, "End date"}}},
		{"close after end", doc14(map[int]string{12: "PLAN,PRSU,Residential - standard user,,2016-03-01,2016-04-01,N,"}), []at{{12, "Close date"}}},
		{"close equals end", doc14(map[int]string{12: "PLAN,PRSU,Residential - standard user,,2016-03-01,2016-03-01,N,"}), []at{{12, "Close date"}}},
		{"record before retailer", doc14(map[int]string{2: ""}), []at{{2, ""}, {3, ""}, {4, ""}, {5, ""}, {6, ""}, {7, ""}, {8, ""}, {9, ""}, {10, ""}, {11, ""}, {12, ""}, {13, ""}}},
		{"no retailer", file("HDR,PRCSCHD,1.0,ASRL,ASRL,ANY,2016-05-22T12:23:45+12,,c7d90f3a-d62b-4923-95c4-4aef120d831e,1,E,I"), []at{{1, ""}}},
		{"network without region", doc14(map[int]string{7: ""}, "NETWORK,CKHK,,RSU,"), []at{{7, ""}, {12, "Tariff region identifier"}, {13, "Tariff region identifier"}, {14, ""}}},
		{"plan without customer", doc14(map[int]string{11: ""}), []at{{11, ""}}},
		{"tariff without plan", doc14(map[int]string{12: ""}), []at{{12, ""}, {13, ""}}},
		{"attribute after customer", doc14(nil, "ATTRIBUTE,AT_x,URL,,,,"), []at{{15, ""}}},
		{"duplicate identifier", doc14(nil, "TARIFF,X,TR_CKHK_RSU,TT_any,1,", "TARIFFTYPE,tt_ANY,Anytime,V,kWh,X,,,"), []at{{16, ""}, {16, "Tariff type identifier"}}},
		{"duplicate plan id", doc14(nil, "PLAN,prsu,Other,,,,N,"), []at{{15, "Product identification code"}}},
		{"shared plan description, other networks", doc14(nil, "RETAILER,BRTL,,Other", "CUSTOMER,,", "PLAN,P2,residential - STANDARD user,,,,N,"), nil},
		{"unknown tariff type", doc14(map[int]string{14: "TARIFF,RSU-24UN,TR_CKHK_RSU,TT_nope,0.1937,"}), []at{{14, "Tariff type identifier"}}},
		{"unknown region", doc14(map[int]string{14: "TARIFF,RSU-24UN,TR_CKHK,TT_any,0.1937,"}), []at{{14, "Tariff region identifier"}}},
		{"unknown schedule in list", doc14(map[int]string{10: "TARIFFTYPE,TT_any,Anytime,V,kWh,X,UN-24,SC_night SC_day,"}), []at{{10, "Schedule identifiers"}}},
		{"reference to other retailer", doc14(nil, "RETAILER,BRTL,,Other", "CUSTOMER,,AT_anzsic_000000"), []at{{16, "Attribute identifiers"}}},
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

func TestEIEP14AMessagesSuggestFix(t *testing.T) {
	tests := []struct{ in, want string }{
		{doc14(map[int]string{12: "PLAN,PRSU,Residential - standard user, 2016-04-01,,,N,"}), `write it as "2016-04-01"`},
		{doc14(map[int]string{6: "SCHEDULE,SC_night,1,21.00,24:00,ANY,ANY"}), `write it as "21:00"`},
		{strings.Replace(doc14(nil), ",14,E", ",26,E", 1), "set it to 13"},
		{doc14(map[int]string{14: "TARIFF,RSU-24UN,TR_CKHK_RSU,TT_nope,0.1937,"}), "does not match any TARIFFTYPE record under this RETAILER; add one"},
		{doc14(map[int]string{2: `RETAILER,ASRL,ASRL,"Simple`}), "add the closing quote"},
		{doc14(map[int]string{6: "SCHEDULE,SC_night,1,22:00,06:00,ANY,ANY"}), "split it into two SCHEDULE records"},
	}
	for _, tt := range tests {
		fs := Validate([]byte(tt.in))
		if len(fs) != 1 || !strings.Contains(fs[0].Msg, tt.want) {
			t.Errorf("want one finding containing %q, got %+v", tt.want, fs)
		}
	}
}

func TestEIEP14ASummary(t *testing.T) {
	r := Check([]byte(doc14(map[int]string{12: "PLAN,PRSU,Residential - standard user,2025-04-01,,,N,"})))
	fs, got := r.Findings, r.Summary
	if len(fs) > 0 || got != "13 records (retailers: 1, plans: 1, tariffs: 2), plans from 2025-04-01 to open" {
		t.Errorf("got %q %v", got, fs)
	}
}
