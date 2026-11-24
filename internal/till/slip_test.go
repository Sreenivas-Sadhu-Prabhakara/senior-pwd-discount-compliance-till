package till

import (
	"strings"
	"testing"
	"time"
)

func testEstablishment() Establishment {
	return Establishment{
		Name:    "Dalisay Kitchen",
		Branch:  "Session Road",
		Address: "Baguio City",
		TIN:     "123-456-789-0000",
	}
}

func TestEstablishmentHeading(t *testing.T) {
	cases := []struct {
		name string
		in   Establishment
		want []string
	}{
		{
			name: "everything given",
			in:   testEstablishment(),
			want: []string{"Dalisay Kitchen", "Session Road", "Baguio City", "TIN 123-456-789-0000"},
		},
		{
			name: "only a name",
			in:   Establishment{Name: "Dalisay Kitchen"},
			want: []string{"Dalisay Kitchen"},
		},
		{
			name: "nothing set says so rather than printing a blank",
			in:   Establishment{},
			want: []string{"establishment name not set"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalise().Heading()
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("heading %q does not contain %q", got, want)
				}
			}
		})
	}
}

func TestEstablishmentNormalise(t *testing.T) {
	got := Establishment{
		Name:    "  Dalisay   Kitchen ",
		Branch:  " Session  Road ",
		Address: "  Baguio City ",
		TIN:     " 123-456-789-0000 ",
	}.Normalise()

	if got.Name != "Dalisay Kitchen" || got.Branch != "Session Road" || got.Address != "Baguio City" || got.TIN != "123-456-789-0000" {
		t.Fatalf("Normalise() = %+v", got)
	}
}

func TestSlipShowsTheOrderOfOperations(t *testing.T) {
	bill := Bill{
		Date:      MustParseDate("2026-03-15"),
		Reference: "OR-1003",
		Lines: []Line{
			coveredLine("Family set meal", Pesos(2240)),
			{Description: "Bottled beer", Amount: Pesos(280)},
		},
		Persons:    4,
		Buyers:     []Buyer{{Name: "Corazon Reyes", Basis: BasisSenior, IDNo: "OSCA-12345", Booklet: "BK-7"}},
		Allocation: AllocationPerHead,
	}
	computed, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	slip := Slip(testEstablishment(), bill, computed)

	for _, want := range []string{
		"Dalisay Kitchen",
		"SENIOR CITIZEN / PERSON WITH DISABILITY CHARGE SLIP",
		"OR-1003",
		"2026-03-15",
		"Family set meal",
		"Bottled beer",
		"Corazon Reyes",
		"OSCA / senior citizen ID no. OSCA-12345",
		"booklet BK-7",
		"RA 9994",
		"RA 10754",
		"BIR RR 7-2010",
		"560.00", // the qualified share of the covered items
		"500.00", // the VAT-exempt selling price
		"100.00", // the 20% discount
		"400.00", // what the senior citizen pays
	} {
		if !strings.Contains(slip, want) {
			t.Errorf("the slip does not contain %q\n%s", want, slip)
		}
	}

	// The deduction lines are bracketed the way a ledger writes them.
	if !strings.Contains(slip, "(60.00)") {
		t.Errorf("the tax taken out is not shown as a deduction\n%s", slip)
	}

	// Order matters more than presence.
	vatAt := strings.Index(slip, "Less value-added tax")
	exemptAt := strings.Index(slip, "VAT-exempt selling price")
	discountAt := strings.Index(slip, "Less statutory discount")
	if vatAt < 0 || exemptAt < 0 || discountAt < 0 {
		t.Fatalf("the slip is missing a step of the working\n%s", slip)
	}
	if !(vatAt < exemptAt && exemptAt < discountAt) {
		t.Fatalf("the slip has the steps out of order: tax at %d, exempt price at %d, discount at %d", vatAt, exemptAt, discountAt)
	}

	// Every line fits the slip width, so it prints without wrapping.
	for _, line := range strings.Split(slip, "\n") {
		if n := len([]rune(line)); n > slipWidth {
			t.Errorf("a slip line is %d characters wide: %q", n, line)
		}
	}
}

func TestSlipWithNoQualifiedBuyer(t *testing.T) {
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-1010",
		Lines:      []Line{coveredLine("Lunch", Pesos(1120))},
		Persons:    2,
		Allocation: AllocationPerHead,
	}
	computed, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	slip := Slip(testEstablishment(), bill, computed)
	if !strings.Contains(slip, "No qualified buyer on this bill") {
		t.Errorf("the slip does not say there was no qualified buyer\n%s", slip)
	}
	if strings.Contains(slip, "QUALIFIED BUYERS") {
		t.Errorf("the slip listed buyers where there were none\n%s", slip)
	}
	if !strings.Contains(slip, "1,120.00") {
		t.Errorf("the slip does not show the full amount due\n%s", slip)
	}
}

func TestSlipShowsThePromotionalChoice(t *testing.T) {
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-4001",
		Lines:      []Line{coveredLine("Set meal", Pesos(1120))},
		Persons:    1,
		Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
		Allocation: AllocationPerHead,
		Promotion:  Pesos(400),
	}
	computed, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	slip := Slip(testEstablishment(), bill, computed)
	if !strings.Contains(slip, "PROMOTIONAL DISCOUNT") {
		t.Errorf("the slip does not mention the promotion\n%s", slip)
	}
	// The advice is wrapped to the slip width, so look for the sentence with
	// its line breaks taken back out.
	flat := flatten(slip)
	for _, want := range []string{"The two do not add up", "whichever is higher"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the slip does not say %q\n%s", want, slip)
		}
	}
}

// flatten runs a wrapped block of text back into one line, so a test can look
// for a sentence without caring where it was broken.
func flatten(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestSummaryText(t *testing.T) {
	summary := Summarise(march(), mustQuarter(t, 2026, 1))
	text := SummaryText(testEstablishment(), summary)

	for _, want := range []string{
		"SENIOR CITIZEN AND PWD DISCOUNTS GRANTED",
		"Q1 2026",
		"Senior citizens (RA 9994)",
		"Persons with disability (RA 10754)",
		"BY MONTH",
		"March 2026",
		"DISCOUNTS GRANTED",
		"deduction from gross income",
		"1,100.00", // 200.00 in February plus 900.00 in March
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the summary does not contain %q\n%s", want, text)
		}
	}
}

func TestRegisterText(t *testing.T) {
	period := mustMonth(t, 2026, time.March)
	sales := InPeriod(march(), period)
	text := RegisterText(testEstablishment(), sales, period)

	for _, want := range []string{
		"SENIOR CITIZEN AND PWD SALES RECORD",
		"March 2026",
		"Buyer name",
		"Discount granted",
		"OSCA-12345",
		"PWD-77-0042",
		"Total — 4 entries",
		"BIR RR 7-2010",
		"RR 5-2017",
		"indicative",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the register does not contain %q\n%s", want, text)
		}
	}

	// A register of nothing still prints its headings and its footing.
	empty := RegisterText(testEstablishment(), nil, mustMonth(t, 2026, time.January))
	if !strings.Contains(empty, "Buyer name") || !strings.Contains(empty, "Total — 0 entries") {
		t.Errorf("an empty register does not print its own frame\n%s", empty)
	}
}

func TestTable(t *testing.T) {
	cases := []struct {
		name  string
		rows  [][]string
		check func(t *testing.T, out string)
	}{
		{
			name: "columns are as wide as their widest cell",
			rows: [][]string{
				{"Date", "Buyer"},
				{"2026-03-15", "Corazon Reyes"},
				{"2026-03-16", "Noel"},
			},
			check: func(t *testing.T, out string) {
				lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
				if len(lines) != 4 {
					t.Fatalf("got %d lines, want a heading, a rule and two rows:\n%s", len(lines), out)
				}
				if !strings.HasPrefix(lines[1], "----------  ") {
					t.Errorf("the rule is not as wide as the date column: %q", lines[1])
				}
				if !strings.HasPrefix(lines[2], "2026-03-15  Corazon Reyes") {
					t.Errorf("row 1 = %q", lines[2])
				}
				if !strings.HasPrefix(lines[3], "2026-03-16  Noel") {
					t.Errorf("row 2 = %q", lines[3])
				}
			},
		},
		{
			name: "a short row is padded out rather than dropping cells",
			rows: [][]string{
				{"A", "B", "C"},
				{"1"},
			},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "\n1\n") {
					t.Errorf("a short row came out as %q", out)
				}
			},
		},
		{name: "nothing at all", rows: nil, check: func(t *testing.T, out string) {
			if out != "" {
				t.Errorf("an empty table came out as %q", out)
			}
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, Table(c.rows))
		})
	}
}

func TestWrap(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{name: "nothing", text: "", width: 20, want: nil},
		{name: "spaces only", text: "   ", width: 20, want: nil},
		{name: "a short line is left alone", text: "senior citizen", width: 20, want: []string{"senior citizen"}},
		{
			name:  "breaking at spaces",
			text:  "the discount is taken on the amount net of value-added tax",
			width: 20,
			want:  []string{"the discount is", "taken on the amount", "net of value-added", "tax"},
		},
		{
			name:  "a word longer than the width is left long rather than cut",
			text:  "incontrovertibility matters",
			width: 10,
			want:  []string{"incontrovertibility", "matters"},
		},
		{
			name:  "runs of whitespace collapse",
			text:  "a   b    c",
			width: 20,
			want:  []string{"a b c"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := wrap(c.text, c.width)
			if len(got) != len(c.want) {
				t.Fatalf("got %d lines %v, want %d %v", len(got), got, len(c.want), c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestPadAndOrDash(t *testing.T) {
	if got := pad("ab", 5); got != "ab   " {
		t.Errorf("pad(\"ab\", 5) = %q", got)
	}
	if got := pad("abcdef", 3); got != "abcdef" {
		t.Errorf("pad does not truncate: %q", got)
	}
	if got := orDash("  "); got != "—" {
		t.Errorf("orDash(\"  \") = %q", got)
	}
	if got := orDash("OR-1"); got != "OR-1" {
		t.Errorf("orDash(\"OR-1\") = %q", got)
	}
}
