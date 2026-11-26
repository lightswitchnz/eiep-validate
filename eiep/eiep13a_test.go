package eiep

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const goodDET = "DET,AUTH1,0001423030UND35,000,,SERIAL1,X,UN,24,01/08/2026 00:00:01,01/08/2026 00:30:00,RD,1.23,"

func hdr(version, uid string, count int) string {
	return fmt.Sprintf("HDR,ICPCONS,%s,GENE,GENE,3ECL,06/08/2026,%s,%d,01/08/2026,06/08/2026", version, uid, count)
}

// det returns goodDET with the given fields replaced.
func det(set map[int]string) string {
	v := strings.Split(goodDET, ",")
	for i, s := range set {
		v[i] = s
	}
	return strings.Join(v, ",")
}

func file(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

// dstDET is a reading inside the hour repeated when daylight saving ended in 2026.
func dstDET(nzdt string) string {
	return det(map[int]string{4: nzdt, 9: "05/04/2026 02:00:01", 10: "05/04/2026 02:30:00"})
}

type at struct {
	Line  int
	Field string
}

func TestEIEP13A(t *testing.T) {
	h1 := hdr("1.4", "3473b1e40cb64855ae8404f9d2830bea", 1)
	tests := []struct {
		name string
		in   string
		want []at
	}{
		{"valid", file(h1, goodDET), nil},
		{"valid CRLF", h1 + "\r\n" + goodDET + "\r\n", nil},
		{"valid CR no trailing", h1 + "\r" + goodDET, nil},
		{"valid no details", file(hdr("1.4", "u", 0)), nil},
		{"valid 1.3", file(hdr("1.3", "uid", 1), goodDET), nil},
		{"valid 1.2", file(hdr("1.2", "uid", 1), goodDET), nil},
		{"valid 1.1", file(hdr("1.1", "uid", 1), goodDET), nil},
		{"valid lowercase codes", file(h1, det(map[int]string{0: "det", 6: "x", 11: "es"})), nil},
		{"valid 24:00:00 end", file(h1, det(map[int]string{9: "01/08/2026 23:30:01", 10: "01/08/2026 24:00:00"})), nil},
		{"valid NZST and reactive", file(h1, det(map[int]string{4: "NZST", 13: "-0.5"})), nil},
		{"valid rejected with blanks", file(h1, "DET,AUTH1,0001423030UND35,001,,,,,,,,,,"), nil},

		{"empty", "", []at{{1, ""}}},
		{"no HDR", file(goodDET), []at{{1, ""}}},
		{"wrong file type", file("HDR,REQCONS,1.4"), []at{{1, "File type"}}},
		{"unsupported version", file(hdr("1.0", "u", 0)), []at{{1, "Version"}}},
		{"1.2 no response code 005", file(hdr("1.2", "u", 1), det(map[int]string{3: "005"})), []at{{2, "Response code"}}},
		{"header field count", file(h1+",extra", goodDET), []at{{1, ""}}},
		{"count mismatch", file(hdr("1.4", "u", 2), goodDET), []at{{1, "Number of detail records"}}},
		{"period reversed", file("HDR,ICPCONS,1.4,GENE,GENE,3ECL,06/08/2026,u,0,06/08/2026,01/08/2026"), []at{{1, "Report period end date"}}},
		{"bad run date", file("HDR,ICPCONS,1.4,GENE,GENE,3ECL,2026-08-06,u,0,01/08/2026,06/08/2026"), []at{{1, "Report run date"}}},
		{"second HDR", file(h1, goodDET, h1), []at{{3, ""}}},
		{"unknown record", file(h1, goodDET, "TRL,1"), []at{{3, ""}}},
		{"blank line", file(h1, "", goodDET), []at{{2, ""}}},
		{"detail field count", file(h1, goodDET+",0"), []at{{2, ""}}},
		{"extra empty fields with error", file(h1, det(map[int]string{6: "Z"})+",,"), []at{{2, "Energy flow direction"}}},
		{"ICP blank", file(h1, det(map[int]string{2: ""})), []at{{2, "ICP identifier"}}},
		{"non-ASCII", file(h1, det(map[int]string{5: "SÉRIAL"})), []at{{2, "Metering component serial number"}}},
		{"bad response code", file(h1, det(map[int]string{3: "0"})), []at{{2, "Response code"}}},
		{"bad direction", file(h1, det(map[int]string{6: "Z"})), []at{{2, "Energy flow direction"}}},
		{"bad NZDT", file(h1, det(map[int]string{4: "NZDT"})), []at{{2, "NZDT adjustment"}}},
		{"period of availability blank", file(h1, det(map[int]string{8: ""})), []at{{2, "Period of availability"}}},
		{"ISO datetime", file(h1, det(map[int]string{9: "2026-08-01 00:00:01"})), []at{{2, "Read period start date and time"}}},
		{"no seconds", file(h1, det(map[int]string{10: "01/08/2026 00:30"})), []at{{2, "Read period end date and time"}}},
		{"impossible date", file(h1, det(map[int]string{9: "31/02/2026 00:00:01"})), []at{{2, "Read period start date and time"}}},
		{"end before start", file(h1, det(map[int]string{10: "01/08/2026 00:00:00"})), []at{{2, "Read period end date and time"}}},
		{"lone point", file(h1, det(map[int]string{12: "."})), []at{{2, "Unit quantity active energy volume"}}},
		{"too many digits", file(h1, det(map[int]string{12: "12345678901"})), []at{{2, "Unit quantity active energy volume"}}},
		{"rejected with data", file(h1, det(map[int]string{3: "002"})), []at{
			{2, "Metering component serial number"}, {2, "Energy flow direction"}, {2, "Register content code"},
			{2, "Period of availability"}, {2, "Read period start date and time"}, {2, "Read period end date and time"},
			{2, "Read status"}, {2, "Unit quantity active energy volume"},
		}},
		{"005 blanks allowed", file(h1, "DET,AUTH1,0001423030UND35,005,,,,,,,,,,"), nil},
		{"two registers same period", file(hdr("1.4", "u", 2), goodDET, det(map[int]string{7: "CN"})), nil},
		{"same register twice (Contact)", file(hdr("1.4", "u", 4), goodDET, det(map[int]string{9: "01/08/2026 00:30:01", 10: "01/08/2026 01:00:00"}), goodDET, det(map[int]string{9: "01/08/2026 00:30:01", 10: "01/08/2026 01:00:00"})), []at{{4, "Register content code"}}},
		{"register identity is case-insensitive", file(hdr("1.4", "u", 2), goodDET, det(map[int]string{6: "x", 7: "un"})), []at{{3, "Register content code"}}},
		{"DST repeated hour allowed", file(hdr("1.4", "u", 2), dstDET(""), dstDET("")), nil},
		{"DST repeated hour at most twice", file(hdr("1.4", "u", 3), dstDET(""), dstDET(""), dstDET("")), []at{{4, "Register content code"}}},
		{"DST repeat not allowed for NZST", file(hdr("1.4", "u", 2), dstDET("NZST"), dstDET("NZST")), []at{{3, "Register content code"}}},
		{"005 still format-checked", file(h1, "DET,AUTH1,0001423030UND35,005,,,Z,,,,,,,"), []at{{2, "Energy flow direction"}}},
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

func TestMessagesSuggestFix(t *testing.T) {
	h1 := hdr("1.4", "u", 1)
	tests := []struct{ in, want string }{
		{file(h1, det(map[int]string{9: "2026-08-01 00:00:01"})), `write it as "01/08/2026 00:00:01"`},
		{file(h1, det(map[int]string{10: "01/08/2026 00:30"})), `write it as "01/08/2026 00:30:00"`},
		{file(h1, det(map[int]string{12: ".5"})), `"0.5"`},
		{file(h1, det(map[int]string{12: "00123456789012.5"})), "at most"},
		{file(h1, det(map[int]string{8: ""})), "fill it in"},
		{file(hdr("1.4", "u", 3), goodDET), "set it to 1"},
		{file(h1, goodDET+",0"), "remove the extra fields"},
	}
	for _, tt := range tests {
		fs := Validate([]byte(tt.in))
		if len(fs) != 1 || !strings.Contains(fs[0].Msg, tt.want) {
			t.Errorf("want one finding containing %q, got %+v", tt.want, fs)
		}
	}
}

func TestWarnings(t *testing.T) {
	tests := []struct {
		in   string
		want at
	}{
		{file(hdr("1.4", "u", 1), det(map[int]string{12: "1.234"})), at{2, "Unit quantity active energy volume"}},
		{file(hdr("1.4", "u", 1), det(map[int]string{12: "01.5"})), at{2, "Unit quantity active energy volume"}},
		{file(hdr("1.4", "u", 1), det(map[int]string{5: " SERIAL1"})), at{2, "Metering component serial number"}},
		{doc14v1(map[int]string{1: strings.Replace(base14v1[0], ",12345,", ",0001,", 1)}), at{1, "Unique file identifier"}},
		{doc14v1(nil, "PLAN,PRLU,residential - STANDARD user,01/04/2016,,30/06/2016,Y", "NETWORK,ckhk,WIL0331CKHK CPK0331CKHK CPK0111CKHK,RSU,LCC1", "TARIFF,X,X,F,1,,,,"), at{13, "Retailers plan description"}},
		{doc14(map[int]string{14: "TARIFF,RSU-24UN,TR_CKHK_RSU,TT_any,0.1234567,"}), at{14, "Tariff rate"}},
		{file(hdr("1.4", "u", 1), goodDET+",,"), at{2, ""}},
		{doc14v1(map[int]string{2: base14v1[1] + ",,,"}), at{2, ""}},
		{doc14(nil, "PLAN,P2,Residential - standard user,2025-04-01,,,N,", "TARIFF,X,TR_CKHK_RSU,TT_any,1,"), at{15, "Plan description"}},
		{doc14(nil, "PLAN,P2,residential - STANDARD user,,,,N,", "TARIFF,X,TR_CKHK_RSU,TT_any,1,"), at{15, "Plan description"}},
		{file(hdr("1.1", "u", 1), det(map[int]string{5: strings.Repeat("S", 16)})), at{2, "Metering component serial number"}},
		{file(hdr("1.1", strings.Repeat("a", 36), 0)), at{1, "Unique request identifier"}},
		{file(hdr("1.3", strings.Repeat("a", 36), 0)), at{1, "Unique request identifier"}},
		{file(hdr("1.4", "u", 1), det(map[int]string{2: "0001423030UND35X"})), at{2, "ICP identifier"}},
		{doc14(map[int]string{5: "ATTRIBUTE,AT_END_202503310000000,TARIFF_END_DATE,2025-03-31,,,", 13: "TARIFF,RSU-FIXED,TR_CKHK_RSU,TT_dly_sup_chg,2.053,AT_END_202503310000000"}), at{5, "Attribute record identifier"}},
		{doc14(map[int]string{7: "TARIFFREGION,TR_CKHK_RSU," + strings.Repeat("d", 51)}), at{7, "Description"}},
		{doc14v1(map[int]string{8: "TARIFF,RSU-FIXED,Daily supply charge,F,2.15600001,Day,B,Fixed,"}), at{8, "Tariff rate"}},
		{doc14(map[int]string{1: strings.Replace(base14[0], "1.0", "2.011", 1)}), at{1, "Version"}},
	}
	for _, tt := range tests {
		r := Check([]byte(tt.in))
		var ws []Finding
		for _, w := range r.Warnings {
			if w.Code != "deprecated" {
				ws = append(ws, w)
			}
		}
		if len(r.Findings) > 0 || len(ws) != 1 || (at{ws[0].Line, ws[0].Field}) != tt.want || r.Summary == "" {
			t.Errorf("want only a warning at %v and a summary, got %+v", tt.want, r)
		}
	}
}

func TestEIEP13ASummary(t *testing.T) {
	later := det(map[int]string{9: "02/08/2026 00:00:01", 10: "02/08/2026 00:30:00"})
	cn := det(map[int]string{7: "CN"})
	got := Check([]byte(file(hdr("1.4", "u", 3), later, goodDET, cn))).Summary
	want := "3 DET records, 1 ICP, report period 01/08/2026 to 06/08/2026, reads 01/08/2026 00:00:01 to 02/08/2026 00:30:00" +
		"\n  ICP 0001423030UND35, meter \"SERIAL1\", register UN/24, flow X (IN): 2 DET records, reads 01/08/2026 00:00:01 to 02/08/2026 00:30:00" +
		"\n  ICP 0001423030UND35, meter \"SERIAL1\", register CN/24, flow X (IN): 1 DET records, reads 01/08/2026 00:00:01 to 01/08/2026 00:30:00"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
