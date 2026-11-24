package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/senior-pwd-discount-compliance-till/internal/till"
)

func TestResolvePeriod(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		wantFrom string
		wantTo   string
		wantKind till.PeriodKind
		wantErr  string
	}{
		{name: "a month", query: "kind=month&year=2026&index=3", wantFrom: "2026-03-01", wantTo: "2026-03-31", wantKind: till.PeriodMonth},
		{name: "a leap February", query: "kind=month&year=2024&index=2", wantFrom: "2024-02-01", wantTo: "2024-02-29", wantKind: till.PeriodMonth},
		{name: "a quarter", query: "kind=quarter&year=2026&index=2", wantFrom: "2026-04-01", wantTo: "2026-06-30", wantKind: till.PeriodQuarter},
		{name: "a taxable year", query: "kind=year&year=2026", wantFrom: "2026-01-01", wantTo: "2026-12-31", wantKind: till.PeriodYear},
		{name: "a span of dates", query: "from=2026-03-05&to=2026-03-09", wantFrom: "2026-03-05", wantTo: "2026-03-09", wantKind: till.PeriodCustom},
		{name: "a span typed backwards", query: "from=2026-03-09&to=2026-03-05", wantFrom: "2026-03-05", wantTo: "2026-03-09", wantKind: till.PeriodCustom},
		{name: "no kind is a month", query: "year=2026&index=7", wantFrom: "2026-07-01", wantTo: "2026-07-31", wantKind: till.PeriodMonth},

		{name: "an unknown kind", query: "kind=fortnight", wantErr: "not a kind of period"},
		{name: "a month that is not a month", query: "kind=month&year=2026&index=13", wantErr: "not a month"},
		{name: "a quarter that is not a quarter", query: "kind=quarter&year=2026&index=9", wantErr: "a quarter is 1 to 4"},
		{name: "a year outside the register", query: "kind=year&year=1990", wantErr: "not a year this register covers"},
		{name: "a year that is not a number", query: "kind=year&year=soon", wantErr: "year:"},
		{name: "a month index that is not a number", query: "kind=month&index=March", wantErr: "month:"},
		{name: "a span with a broken date", query: "from=yesterday&to=2026-03-09", wantErr: "from:"},
		{name: "a span with no end", query: "from=2026-03-05", wantErr: "needs both a first and a last day"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/summary?"+c.query, nil)

			got, err := resolvePeriod(r)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("resolvePeriod gave %v, wanted %q", got, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("error %q does not mention %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePeriod: %v", err)
			}
			if got.From.String() != c.wantFrom || got.To.String() != c.wantTo {
				t.Fatalf("period runs %s to %s, want %s to %s", got.From, got.To, c.wantFrom, c.wantTo)
			}
			if got.Kind != c.wantKind {
				t.Fatalf("kind = %q, want %q", got.Kind, c.wantKind)
			}
		})
	}
}

func TestResolvePeriodDefaultsToTheMonthWeAreIn(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/summary", nil)

	got, err := resolvePeriod(r)
	if err != nil {
		t.Fatalf("resolvePeriod: %v", err)
	}
	today := till.Today()
	if got.Kind != till.PeriodMonth || got.From.Year() != today.Year() || got.From.Month() != today.Month() {
		t.Fatalf("with nothing asked for, the period is %v, want this month", got)
	}
	if !today.Within(got.From, got.To) {
		t.Fatalf("today is not inside the default period %v", got)
	}
}

func TestPeriodOptionsCoverThreeYears(t *testing.T) {
	options := periodOptions(till.MustParseDate("2026-03-15"))

	// Three years of: the year, four quarters and twelve months.
	if want := 3 * (1 + 4 + 12); len(options) != want {
		t.Fatalf("got %d periods, want %d", len(options), want)
	}
	if options[0].Kind != till.PeriodYear || options[0].Year != 2026 {
		t.Errorf("the first option is %+v, want this year", options[0])
	}

	years := map[int]bool{}
	for _, option := range options {
		years[option.Year] = true
		if option.Label == "" {
			t.Errorf("a period has no label: %+v", option)
		}
		if option.From.IsZero() || option.To.IsZero() || option.From.After(option.To) {
			t.Errorf("a period has impossible dates: %+v", option)
		}
	}
	for _, year := range []int{2026, 2025, 2024} {
		if !years[year] {
			t.Errorf("%d is missing from the periods offered", year)
		}
	}
	if years[2023] {
		t.Error("the periods offered reach further back than three years")
	}
}

func TestSafeFileName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "March 2026", want: "march-2026"},
		{in: "Q1 2026 (January–March)", want: "q1-2026-january-march"},
		{in: "2026-03-05 to 2026-03-09", want: "2026-03-05-to-2026-03-09"},
		{in: "../../etc/passwd", want: "etc-passwd"},
		{in: "  ", want: "period"},
		{in: "", want: "period"},
		{in: "///", want: "period"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := safeFileName(c.in); got != c.want {
				t.Fatalf("safeFileName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestIntParam(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		fallback int
		want     int
		wantErr  bool
	}{
		{name: "a number", in: "7", fallback: 1, want: 7},
		{name: "empty falls back", in: "", fallback: 3, want: 3},
		{name: "space falls back", in: "  ", fallback: 3, want: 3},
		{name: "negative is read, the caller judges it", in: "-2", fallback: 1, want: -2},
		{name: "not a number", in: "three", fallback: 1, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := intParam(c.in, c.fallback)
			if c.wantErr {
				if err == nil {
					t.Fatalf("intParam(%q) = %d, wanted an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("intParam(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("intParam(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestShippedFiguresAreTheStatutoryOnes(t *testing.T) {
	got := shippedFigures()

	if got.VATRatePercent != till.VATRatePercent || got.DiscountPercent != till.StatutoryDiscountPercent {
		t.Errorf("the interface is told %d%% VAT and %d%% discount", got.VATRatePercent, got.DiscountPercent)
	}
	if !got.DiscountOnVATExclusive {
		t.Error("the interface is not told that the discount applies to the VAT-exclusive amount")
	}
	if !strings.Contains(got.SeniorStatute, "RA 9994") || !strings.Contains(got.PWDStatute, "RA 10754") {
		t.Errorf("the statutes are not named: %q, %q", got.SeniorStatute, got.PWDStatute)
	}
	if got.InForceAsAt == "" {
		t.Error("the interface is not told the figures need confirming")
	}
}

func TestSameOrigin(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{name: "the same host and port", origin: "http://127.0.0.1:8080", host: "127.0.0.1:8080", want: true},
		{name: "case does not matter", origin: "http://LOCALHOST:8080", host: "localhost:8080", want: true},
		{name: "another port is another origin", origin: "http://127.0.0.1:9999", host: "127.0.0.1:8080"},
		{name: "another site", origin: "https://example.test", host: "127.0.0.1:8080"},
		{name: "nonsense", origin: "not a url", host: "127.0.0.1:8080"},
		{name: "empty", origin: "", host: "127.0.0.1:8080"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sameOrigin(c.origin, c.host); got != c.want {
				t.Fatalf("sameOrigin(%q, %q) = %v, want %v", c.origin, c.host, got, c.want)
			}
		})
	}
}
