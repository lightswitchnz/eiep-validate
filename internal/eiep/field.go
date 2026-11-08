package eiep

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Kind int

const (
	Char Kind = iota
	Num
	Date
	DateTime
	ISODate     // YYYY-MM-DD
	ISODateTime // YYYY-MM-DDTHH:MM:SS with a UTC offset
	Clock       // HH:MM, 00:00 to 24:00
	Time        // HH:MM:SS, 24-hour
	UUID
	Ident     // letters, digits and underscores
	IdentList // Idents separated by single spaces
)

// Field is one column of a record, as defined in an EIEP's data inputs table.
// Len/Dec follow the spec's notation: Char n, Num n.d.
type Field struct {
	Name     string
	Kind     Kind
	Len      int
	Dec      int
	Required bool
	Codes    []string
	Hint     string
}

type Problem struct {
	Field string
	Code  string
	Msg   string
}

const (
	isoDate        = "2006-01-02"
	dateLayout     = "02/01/2006"
	dateTimeLayout = "02/01/2006 15:04:05"
)

var (
	numRe      = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	leadZeroRe = regexp.MustCompile(`^-?0[0-9]`)
	dateRe     = regexp.MustCompile(`^[0-9]{2}/[0-9]{2}/[0-9]{4}$`)
	dateTimeRe = regexp.MustCompile(`^[0-9]{2}/[0-9]{2}/[0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2}$`)
	isoDateRe  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	isoDTRe    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}(:?[0-9]{2})?)$`)
	timeRe     = regexp.MustCompile(`^[0-9]{2}:[0-9]{2}:[0-9]{2}$`)
	clockRe    = regexp.MustCompile(`^([0-9]{2}):([0-9]{2})$`)
	uuidRe     = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)
	identRe    = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

	// Common non-spec spellings, used only to suggest the corrected value.
	looseDates     = []string{"2006-01-02", "2/1/2006", "02-01-2006", "2006/01/02"}
	looseISODates  = []string{"2006-01-02", "02/01/2006", "2/1/2006", "2006/01/02"}
	looseDateTimes = []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02T15:04", "02/01/2006 15:04", "2/1/2006 15:04:05", "2/1/2006 15:04"}
)

func parseDate(v string) (time.Time, error) {
	if !dateRe.MatchString(v) {
		return time.Time{}, fmt.Errorf("bad date")
	}
	return time.Parse(dateLayout, v)
}

// parseDateTime accepts 24:00:00 as the end of the given day, as the spec allows.
func parseDateTime(v string) (time.Time, error) {
	if !dateTimeRe.MatchString(v) {
		return time.Time{}, fmt.Errorf("bad datetime")
	}
	if strings.HasSuffix(v, " 24:00:00") {
		d, err := parseDate(v[:10])
		return d.AddDate(0, 0, 1), err
	}
	return time.Parse(dateTimeLayout, v)
}

func parseISODate(v string) (time.Time, error) {
	if !isoDateRe.MatchString(v) {
		return time.Time{}, fmt.Errorf("bad date")
	}
	return time.Parse(isoDate, v)
}

func parseISODateTime(v string) (t time.Time, err error) {
	if !isoDTRe.MatchString(v) {
		return t, fmt.Errorf("bad datetime")
	}
	for _, l := range []string{"2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05Z0700", "2006-01-02T15:04:05Z07"} {
		if t, err = time.Parse(l, v); err == nil {
			return t, nil
		}
	}
	return t, err
}

// parseClock returns minutes since midnight; 24:00 is the end of the day.
func parseClock(v string) (int, bool) {
	m := clockRe.FindStringSubmatch(v)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	if min > 59 || h > 24 || h == 24 && min != 0 {
		return 0, false
	}
	return h*60 + min, true
}

func suggest(v string, layouts []string, out string) string {
	for _, l := range layouts {
		if t, err := time.Parse(l, v); err == nil {
			return fmt.Sprintf("; write it as %q", t.Format(out))
		}
	}
	return ""
}

func (f Field) hint() string {
	if f.Hint == "" {
		return ""
	}
	return " (" + f.Hint + ")"
}

func (f Field) check(v string) (code, msg string) {
	if (f.Kind == Char || f.Kind == Ident || f.Kind == IdentList) && len(v) > f.Len {
		g := f
		g.Len = len(v)
		if code, msg = g.check(v); code != "" {
			return code, msg
		}
		return "too_long", fmt.Sprintf("%q is %d characters; the spec allows at most %d%s", v, len(v), f.Len, f.hint())
	}
	switch f.Kind {
	case Char:
		for i := 0; i < len(v); i++ {
			if v[i] < 32 || v[i] > 126 {
				return "bad_char", fmt.Sprintf("%q contains a disallowed character at position %d; use only printable ASCII (no tabs, accents or smart quotes)", v, i+1)
			}
		}
		if strings.TrimSpace(v) != v {
			return "edge_spaces", fmt.Sprintf("%q has leading or trailing spaces; remove them", v)
		}
		if len(f.Codes) > 0 && !foldIn(v, f.Codes) {
			return "bad_code", fmt.Sprintf("%q is not a valid code; use one of %s%s", v, strings.Join(f.Codes, ", "), f.hint())
		}
	case Num:
		m := numRe.FindStringSubmatch(v)
		switch {
		case m != nil:
		case strings.HasPrefix(v, "+"):
			return "bad_number", fmt.Sprintf("%q has a + sign; remove it (only negatives carry a sign)", v)
		case strings.HasPrefix(strings.TrimPrefix(v, "-"), "."):
			return "bad_number", fmt.Sprintf("%q has no digit before the decimal point; write a leading 0, e.g. %q", v, strings.Replace(v, ".", "0.", 1))
		case leadZeroRe.MatchString(v):
			neg := strings.HasPrefix(v, "-")
			t := strings.TrimLeft(strings.TrimPrefix(v, "-"), "0")
			if t == "" || t[0] == '.' {
				t = "0" + t
			}
			if neg {
				t = "-" + t
			}
			if code, msg := f.check(t); code != "" {
				return code, msg
			}
			return "leading_zeros", fmt.Sprintf("%q has leading zeros; write it as %q", v, t)
		default:
			return "bad_number", fmt.Sprintf("%q is not a number; use digits with an optional leading - and decimal point, e.g. 123.45 (no spaces, units or thousands separators)", v)
		}
		if len(m[1]) > f.Len-f.Dec {
			return "too_many_digits", fmt.Sprintf("%q has %d digits before the decimal point; at most %d are allowed", v, len(m[1]), f.Len-f.Dec)
		}
		if dec := len(strings.TrimPrefix(m[2], ".")); dec > f.Dec {
			if f.Dec == 0 {
				return "too_many_decimals", fmt.Sprintf("%q must be a whole number; remove the decimal part", v)
			}
			n, _ := strconv.ParseFloat(v, 64)
			return "extra_decimals", fmt.Sprintf("%q has %d decimal places; the spec allows at most %d, e.g. %q", v, dec, f.Dec, strconv.FormatFloat(n, 'f', f.Dec, 64))
		}
	case Date:
		if _, err := parseDate(v); err != nil {
			if dateRe.MatchString(v) {
				return "bad_date", fmt.Sprintf("%q is not a real calendar date", v)
			}
			return "bad_date", fmt.Sprintf("%q is not in DD/MM/YYYY format%s", v, suggest(v, looseDates, dateLayout))
		}
	case DateTime:
		if _, err := parseDateTime(v); err != nil {
			if dateTimeRe.MatchString(v) {
				return "bad_datetime", fmt.Sprintf("%q is not a real date and time", v)
			}
			s := suggest(v, looseDateTimes, dateTimeLayout)
			if s == "" {
				s = " (24-hour clock, seconds included)"
			}
			return "bad_datetime", fmt.Sprintf("%q is not in DD/MM/YYYY HH:MM:SS format%s", v, s)
		}
	case ISODate:
		if _, err := parseISODate(v); err != nil {
			if isoDateRe.MatchString(v) {
				return "bad_date", fmt.Sprintf("%q is not a real calendar date", v)
			}
			return "bad_date", fmt.Sprintf("%q is not in YYYY-MM-DD format%s", v, suggest(strings.TrimSpace(v), looseISODates, isoDate))
		}
	case ISODateTime:
		if _, err := parseISODateTime(v); err != nil {
			if isoDTRe.MatchString(v) {
				return "bad_datetime", fmt.Sprintf("%q is not a real date and time", v)
			}
			return "bad_datetime", fmt.Sprintf("%q is not an ISO 8601 date and time with a UTC offset; write it as YYYY-MM-DDTHH:MM:SS+HH, e.g. \"2026-10-19T14:53:17+13\" (+12 in NZST, +13 in NZDT)", v)
		}
	case Clock:
		if _, ok := parseClock(v); !ok {
			msg := fmt.Sprintf("%q is not a time in HH:MM format (24-hour clock, 00:00 to 24:00)", v)
			if t, err := time.Parse("15:04", strings.Replace(strings.TrimSpace(v), ".", ":", 1)); err == nil {
				msg += fmt.Sprintf("; write it as %q", t.Format("15:04"))
			}
			return "bad_time", msg
		}
	case Time:
		if _, err := time.Parse("15:04:05", v); err != nil || !timeRe.MatchString(v) {
			return "bad_time", fmt.Sprintf("%q is not a time in HH:MM:SS format (24-hour clock, seconds included)", v)
		}
	case UUID:
		if !uuidRe.MatchString(v) {
			return "bad_uuid", fmt.Sprintf("%q is not a UUID; use 36 characters, hex digits in 8-4-4-4-12 groups, e.g. \"a9d48b18-6b1e-439c-9d05-9b144543a96e\"", v)
		}
	case Ident:
		if !identRe.MatchString(v) {
			return "bad_identifier", fmt.Sprintf("%q may contain only letters, digits and underscores (no spaces or punctuation)", v)
		}
	case IdentList:
		for _, id := range strings.Split(v, " ") {
			if id == "" {
				return "spaces", fmt.Sprintf("%q has leading, trailing or double spaces; separate identifiers with single spaces", v)
			}
			if !identRe.MatchString(id) {
				return "bad_identifier", fmt.Sprintf("%q in %q is not an identifier; identifiers contain only letters, digits and underscores and are separated by single spaces", id, v)
			}
		}
	}
	return "", ""
}

func foldIn(v string, codes []string) bool {
	for _, c := range codes {
		if strings.EqualFold(v, c) {
			return true
		}
	}
	return false
}

func checkFields(fs []Field, v []string) []Problem {
	var p []Problem
	for i, f := range fs {
		if v[i] == "" {
			if f.Required {
				p = append(p, Problem{f.Name, "blank", "mandatory field is blank; fill it in" + f.hint()})
			}
			continue
		}
		if code, msg := f.check(v[i]); code != "" {
			p = append(p, Problem{f.Name, code, msg})
		}
	}
	return p
}

func names(fs []Field) string {
	n := make([]string, len(fs))
	for i, f := range fs {
		n[i] = f.Name
	}
	return strings.Join(n, ", ")
}

func expectedGot(fs []Field, v []string) string {
	q := make([]string, len(v))
	for i, s := range v {
		q[i] = strconv.Quote(s)
	}
	return "\n  Expected: " + names(fs) + "\n  Got:      " + strings.Join(q, ", ")
}
