package till

import (
	"fmt"
	"strings"
	"time"
)

// Date is a calendar day with no time of day and no time zone: the day a sale
// happened, as the sales record writes it.
type Date struct {
	t time.Time
}

// dateLayout is the one form a date is written in, in the data file and in the
// interface.
const dateLayout = "2006-01-02"

// NewDate builds a date from a year, month and day.
func NewDate(year int, month time.Month, day int) Date {
	return Date{t: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// Today is the current day where the machine is.
func Today() Date {
	now := time.Now()
	return NewDate(now.Year(), now.Month(), now.Day())
}

// ParseDate reads a date written as YYYY-MM-DD. An empty string is the zero
// date, which the callers treat as "not given".
func ParseDate(s string) (Date, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return Date{}, nil
	}
	t, err := time.ParseInLocation(dateLayout, trimmed, time.UTC)
	if err != nil {
		return Date{}, fmt.Errorf("%q is not a date — write it as YYYY-MM-DD, such as 2026-03-15", trimmed)
	}
	return Date{t: t}, nil
}

// MustParseDate is ParseDate for worked examples and tests, where the date is
// written by hand and known good.
func MustParseDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// IsZero reports whether no date was given.
func (d Date) IsZero() bool { return d.t.IsZero() }

// String writes the date as YYYY-MM-DD, or empty when there is none.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(dateLayout)
}

// Year, Month and Day are the parts of the date.
func (d Date) Year() int         { return d.t.Year() }
func (d Date) Month() time.Month { return d.t.Month() }
func (d Date) Day() int          { return d.t.Day() }

// Before, After and Equal compare two dates.
func (d Date) Before(o Date) bool { return d.t.Before(o.t) }
func (d Date) After(o Date) bool  { return d.t.After(o.t) }
func (d Date) Equal(o Date) bool  { return d.t.Equal(o.t) }

// Within reports whether the date falls in a span, both ends included. A span
// with no start reaches back for ever, and one with no end reaches forward for
// ever, so a register can be asked for everything.
func (d Date) Within(from, to Date) bool {
	if d.IsZero() {
		return false
	}
	if !from.IsZero() && d.Before(from) {
		return false
	}
	if !to.IsZero() && d.After(to) {
		return false
	}
	return true
}

// MarshalJSON writes the date as a quoted YYYY-MM-DD string.
func (d Date) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }

// UnmarshalJSON reads a date written as a quoted YYYY-MM-DD string.
func (d *Date) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "null" {
		*d = Date{}
		return nil
	}
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	parsed, err := ParseDate(text)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// EndOfMonth is the last day of the month the date falls in. It is worked out
// by stepping to day zero of the next month, so February has 28 days or 29 as
// the year requires and no month length is written down anywhere.
func (d Date) EndOfMonth() Date {
	if d.IsZero() {
		return Date{}
	}
	return Date{t: time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, time.UTC)}
}

// StartOfMonth is the first day of the month the date falls in.
func (d Date) StartOfMonth() Date {
	if d.IsZero() {
		return Date{}
	}
	return NewDate(d.Year(), d.Month(), 1)
}

// PeriodKind is the kind of span a register is summarised over.
type PeriodKind string

const (
	// PeriodMonth is one calendar month — the span a value-added tax return
	// and a monthly sales report are made up over.
	PeriodMonth PeriodKind = "month"

	// PeriodQuarter is one calendar quarter.
	PeriodQuarter PeriodKind = "quarter"

	// PeriodYear is one calendar year, which is the taxable year the discounts
	// granted are claimed as a deduction in.
	PeriodYear PeriodKind = "year"

	// PeriodCustom is any span of dates the user asks for.
	PeriodCustom PeriodKind = "custom"
)

// Period is a span of days the register is read over, with the label it is
// known by.
type Period struct {
	Kind  PeriodKind `json:"kind"`
	Label string     `json:"label"`
	From  Date       `json:"from"`
	To    Date       `json:"to"`
}

// monthNames is only for labels.
var monthNames = [...]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// MonthPeriod is one calendar month, first day to last.
func MonthPeriod(year int, month time.Month) (Period, error) {
	if month < time.January || month > time.December {
		return Period{}, fmt.Errorf("%d is not a month", int(month))
	}
	if err := checkYear(year); err != nil {
		return Period{}, err
	}
	from := NewDate(year, month, 1)
	return Period{
		Kind:  PeriodMonth,
		Label: fmt.Sprintf("%s %d", monthNames[month-1], year),
		From:  from,
		To:    from.EndOfMonth(),
	}, nil
}

// QuarterPeriod is one calendar quarter, 1 to 4.
func QuarterPeriod(year, quarter int) (Period, error) {
	if quarter < 1 || quarter > 4 {
		return Period{}, fmt.Errorf("a quarter is 1 to 4, not %d", quarter)
	}
	if err := checkYear(year); err != nil {
		return Period{}, err
	}
	firstMonth := time.Month((quarter-1)*3 + 1)
	from := NewDate(year, firstMonth, 1)
	to := NewDate(year, firstMonth+2, 1).EndOfMonth()
	return Period{
		Kind:  PeriodQuarter,
		Label: fmt.Sprintf("Q%d %d (%s–%s)", quarter, year, monthNames[firstMonth-1], monthNames[firstMonth+1]),
		From:  from,
		To:    to,
	}, nil
}

// YearPeriod is one calendar year — the taxable year.
func YearPeriod(year int) (Period, error) {
	if err := checkYear(year); err != nil {
		return Period{}, err
	}
	return Period{
		Kind:  PeriodYear,
		Label: fmt.Sprintf("Year %d", year),
		From:  NewDate(year, time.January, 1),
		To:    NewDate(year, time.December, 31),
	}, nil
}

// CustomPeriod is any span the user names, in either order — a span typed
// backwards is read the way round it must have been meant.
func CustomPeriod(from, to Date) (Period, error) {
	if from.IsZero() || to.IsZero() {
		return Period{}, fmt.Errorf("a span of dates needs both a first and a last day")
	}
	if from.After(to) {
		from, to = to, from
	}
	return Period{
		Kind:  PeriodCustom,
		Label: from.String() + " to " + to.String(),
		From:  from,
		To:    to,
	}, nil
}

// checkYear refuses a year that cannot be a trading year, so a mistyped year
// does not produce a plausible-looking empty report.
func checkYear(year int) error {
	if year < 2010 || year > 2200 {
		return fmt.Errorf("%d is not a year this register covers", year)
	}
	return nil
}

// MonthsIn lists every calendar month a period touches, in order, so a
// summary can be broken down month by month.
func MonthsIn(p Period) []Period {
	if p.From.IsZero() || p.To.IsZero() || p.From.After(p.To) {
		return nil
	}
	var out []Period
	cursor := p.From.StartOfMonth()
	for !cursor.After(p.To) {
		month, err := MonthPeriod(cursor.Year(), cursor.Month())
		if err != nil {
			return out
		}
		out = append(out, month)
		cursor = Date{t: cursor.t.AddDate(0, 1, 0)}.StartOfMonth()
	}
	return out
}
