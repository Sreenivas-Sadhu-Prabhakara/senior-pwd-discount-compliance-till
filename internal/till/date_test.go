package till

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   string
		isZero bool
		wantE  bool
	}{
		{name: "a day", in: "2026-03-15", want: "2026-03-15"},
		{name: "the leap day", in: "2024-02-29", want: "2024-02-29"},
		{name: "surrounding space", in: " 2026-03-15 ", want: "2026-03-15"},
		{name: "empty is no date", in: "", isZero: true},
		{name: "spaces are no date", in: "   ", isZero: true},
		{name: "29 February of a common year", in: "2023-02-29", wantE: true},
		{name: "the wrong way round", in: "15-03-2026", wantE: true},
		{name: "a month that does not exist", in: "2026-13-01", wantE: true},
		{name: "not a date at all", in: "yesterday", wantE: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDate(c.in)
			if c.wantE {
				if err == nil {
					t.Fatalf("ParseDate(%q) = %v, wanted an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", c.in, err)
			}
			if c.isZero {
				if !got.IsZero() {
					t.Fatalf("ParseDate(%q) = %v, wanted no date", c.in, got)
				}
				if got.String() != "" {
					t.Fatalf("a zero date writes itself as %q", got.String())
				}
				return
			}
			if got.String() != c.want {
				t.Fatalf("ParseDate(%q).String() = %q, want %q", c.in, got.String(), c.want)
			}
		})
	}
}

func TestEndOfMonth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "February in a leap year has 29 days", in: "2024-02-01", want: "2024-02-29"},
		{name: "February in a common year has 28", in: "2023-02-14", want: "2023-02-28"},
		{name: "February in 2000, a leap year", in: "2000-02-03", want: "2000-02-29"},
		{name: "February in 2100, which is not", in: "2100-02-03", want: "2100-02-28"},
		{name: "a 30-day month", in: "2026-04-17", want: "2026-04-30"},
		{name: "a 31-day month", in: "2026-07-01", want: "2026-07-31"},
		{name: "December rolls the year", in: "2026-12-09", want: "2026-12-31"},
		{name: "the last day stays put", in: "2026-01-31", want: "2026-01-31"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MustParseDate(c.in).EndOfMonth()
			if got.String() != c.want {
				t.Fatalf("EndOfMonth(%s) = %s, want %s", c.in, got, c.want)
			}
			start := MustParseDate(c.in).StartOfMonth()
			if start.Day() != 1 {
				t.Fatalf("StartOfMonth(%s) = %s, which is not a first", c.in, start)
			}
		})
	}

	if got := (Date{}).EndOfMonth(); !got.IsZero() {
		t.Fatalf("the end of no month is %v", got)
	}
	if got := (Date{}).StartOfMonth(); !got.IsZero() {
		t.Fatalf("the start of no month is %v", got)
	}
}

func TestDateWithin(t *testing.T) {
	from := MustParseDate("2026-03-01")
	to := MustParseDate("2026-03-31")

	cases := []struct {
		name string
		date Date
		from Date
		to   Date
		want bool
	}{
		{name: "inside", date: MustParseDate("2026-03-15"), from: from, to: to, want: true},
		{name: "the first day counts", date: from, from: from, to: to, want: true},
		{name: "the last day counts", date: to, from: from, to: to, want: true},
		{name: "the day before does not", date: MustParseDate("2026-02-28"), from: from, to: to, want: false},
		{name: "the day after does not", date: MustParseDate("2026-04-01"), from: from, to: to, want: false},
		{name: "no start reaches back", date: MustParseDate("2019-01-01"), to: to, want: true},
		{name: "no end reaches forward", date: MustParseDate("2099-01-01"), from: from, want: true},
		{name: "no span at all takes everything", date: MustParseDate("2026-03-15"), want: true},
		{name: "no date is in nothing", date: Date{}, from: from, to: to, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.date.Within(c.from, c.to); got != c.want {
				t.Fatalf("%v.Within(%v, %v) = %v, want %v", c.date, c.from, c.to, got, c.want)
			}
		})
	}
}

func TestPeriods(t *testing.T) {
	cases := []struct {
		name     string
		build    func() (Period, error)
		wantFrom string
		wantTo   string
		wantKind PeriodKind
		wantE    bool
	}{
		{
			name:     "a month",
			build:    func() (Period, error) { return MonthPeriod(2026, time.March) },
			wantFrom: "2026-03-01", wantTo: "2026-03-31", wantKind: PeriodMonth,
		},
		{
			name:     "February in a leap year",
			build:    func() (Period, error) { return MonthPeriod(2024, time.February) },
			wantFrom: "2024-02-01", wantTo: "2024-02-29", wantKind: PeriodMonth,
		},
		{
			name:     "the first quarter",
			build:    func() (Period, error) { return QuarterPeriod(2026, 1) },
			wantFrom: "2026-01-01", wantTo: "2026-03-31", wantKind: PeriodQuarter,
		},
		{
			name:     "the last quarter",
			build:    func() (Period, error) { return QuarterPeriod(2026, 4) },
			wantFrom: "2026-10-01", wantTo: "2026-12-31", wantKind: PeriodQuarter,
		},
		{
			name:     "a quarter ending in a leap February",
			build:    func() (Period, error) { return QuarterPeriod(2024, 1) },
			wantFrom: "2024-01-01", wantTo: "2024-03-31", wantKind: PeriodQuarter,
		},
		{
			name:     "a taxable year",
			build:    func() (Period, error) { return YearPeriod(2026) },
			wantFrom: "2026-01-01", wantTo: "2026-12-31", wantKind: PeriodYear,
		},
		{
			name: "a span of days",
			build: func() (Period, error) {
				return CustomPeriod(MustParseDate("2026-03-05"), MustParseDate("2026-03-09"))
			},
			wantFrom: "2026-03-05", wantTo: "2026-03-09", wantKind: PeriodCustom,
		},
		{
			name: "a span typed backwards is read the right way round",
			build: func() (Period, error) {
				return CustomPeriod(MustParseDate("2026-03-09"), MustParseDate("2026-03-05"))
			},
			wantFrom: "2026-03-05", wantTo: "2026-03-09", wantKind: PeriodCustom,
		},
		{name: "month 0", build: func() (Period, error) { return MonthPeriod(2026, 0) }, wantE: true},
		{name: "month 13", build: func() (Period, error) { return MonthPeriod(2026, 13) }, wantE: true},
		{name: "quarter 0", build: func() (Period, error) { return QuarterPeriod(2026, 0) }, wantE: true},
		{name: "quarter 5", build: func() (Period, error) { return QuarterPeriod(2026, 5) }, wantE: true},
		{name: "year 1990", build: func() (Period, error) { return YearPeriod(1990) }, wantE: true},
		{name: "a span with no end", build: func() (Period, error) { return CustomPeriod(MustParseDate("2026-01-01"), Date{}) }, wantE: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.build()
			if c.wantE {
				if err == nil {
					t.Fatalf("built %v, wanted an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.From.String() != c.wantFrom || got.To.String() != c.wantTo {
				t.Fatalf("period runs %s to %s, want %s to %s", got.From, got.To, c.wantFrom, c.wantTo)
			}
			if got.Kind != c.wantKind {
				t.Fatalf("kind = %q, want %q", got.Kind, c.wantKind)
			}
			if got.Label == "" {
				t.Fatal("the period has no label")
			}
		})
	}
}

func TestMonthsIn(t *testing.T) {
	cases := []struct {
		name   string
		period Period
		want   []string
	}{
		{
			name:   "a single month is itself",
			period: mustMonth(t, 2026, time.March),
			want:   []string{"March 2026"},
		},
		{
			name:   "a quarter is three months",
			period: mustQuarter(t, 2026, 2),
			want:   []string{"April 2026", "May 2026", "June 2026"},
		},
		{
			name:   "a year is twelve",
			period: mustYear(t, 2026),
			want: []string{
				"January 2026", "February 2026", "March 2026", "April 2026", "May 2026", "June 2026",
				"July 2026", "August 2026", "September 2026", "October 2026", "November 2026", "December 2026",
			},
		},
		{
			name:   "a span inside one month is that month",
			period: mustCustom(t, "2026-03-05", "2026-03-09"),
			want:   []string{"March 2026"},
		},
		{
			name:   "a span across a year end covers both months",
			period: mustCustom(t, "2025-12-20", "2026-01-05"),
			want:   []string{"December 2025", "January 2026"},
		},
		{
			name:   "a span across a leap February",
			period: mustCustom(t, "2024-01-31", "2024-03-01"),
			want:   []string{"January 2024", "February 2024", "March 2024"},
		},
		{name: "a period with no dates has no months", period: Period{}, want: nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MonthsIn(c.period)
			if len(got) != len(c.want) {
				t.Fatalf("got %d months, want %d: %v", len(got), len(c.want), labels(got))
			}
			for i := range got {
				if got[i].Label != c.want[i] {
					t.Errorf("month %d = %q, want %q", i, got[i].Label, c.want[i])
				}
			}
		})
	}
}

func TestDateJSON(t *testing.T) {
	type holder struct {
		Day Date `json:"day"`
	}

	body, err := json.Marshal(holder{Day: MustParseDate("2026-03-15")})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"day":"2026-03-15"}` {
		t.Fatalf("marshalled to %s", body)
	}

	cases := []struct {
		name  string
		in    string
		want  string
		wantE bool
	}{
		{name: "a day", in: `{"day":"2026-03-15"}`, want: "2026-03-15"},
		{name: "null is no day", in: `{"day":null}`, want: ""},
		{name: "empty is no day", in: `{"day":""}`, want: ""},
		{name: "nonsense is refused", in: `{"day":"15/03/2026"}`, wantE: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var h holder
			err := json.Unmarshal([]byte(c.in), &h)
			if c.wantE {
				if err == nil {
					t.Fatalf("unmarshalling %s gave %v, wanted an error", c.in, h.Day)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshalling %s: %v", c.in, err)
			}
			if h.Day.String() != c.want {
				t.Fatalf("unmarshalling %s gave %q, want %q", c.in, h.Day.String(), c.want)
			}
		})
	}
}

func TestTodayIsADay(t *testing.T) {
	today := Today()
	if today.IsZero() {
		t.Fatal("Today() has no date")
	}
	if today.Day() < 1 || today.Day() > 31 {
		t.Fatalf("Today() is day %d of the month", today.Day())
	}
	if parsed := MustParseDate(today.String()); !parsed.Equal(today) {
		t.Fatalf("Today() does not survive being written and read back: %v vs %v", today, parsed)
	}
}

// --- helpers -------------------------------------------------------------

func mustMonth(t *testing.T, year int, month time.Month) Period {
	t.Helper()
	p, err := MonthPeriod(year, month)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustQuarter(t *testing.T, year, q int) Period {
	t.Helper()
	p, err := QuarterPeriod(year, q)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustYear(t *testing.T, year int) Period {
	t.Helper()
	p, err := YearPeriod(year)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustCustom(t *testing.T, from, to string) Period {
	t.Helper()
	p, err := CustomPeriod(MustParseDate(from), MustParseDate(to))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func labels(ps []Period) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Label)
	}
	return out
}
