package till

import (
	"strings"
	"testing"
	"time"
)

// entry builds a consistent register entry: gross net of 12% tax, less 20%.
func entry(date, ref, name string, basis Basis, id string, gross Centavos) Sale {
	exempt := gross.NetOfVAT(VATRatePercent)
	discount := exempt.Percent(StatutoryDiscountPercent)
	return Sale{
		ID:           ref + "-" + id,
		Date:         MustParseDate(date),
		Reference:    ref,
		BuyerName:    name,
		Basis:        basis,
		IDNo:         id,
		Persons:      1,
		Allocation:   AllocationPerHead,
		Gross:        gross,
		VATExempt:    exempt,
		Discount:     discount,
		Net:          exempt - discount,
		VATRate:      VATRatePercent,
		DiscountRate: StatutoryDiscountPercent,
	}
}

// march is a month of trading: four entries in March, one in February and one
// in April, so the period boundaries are actually tested.
func march() []Sale {
	return []Sale{
		entry("2026-02-28", "OR-0900", "Corazon Reyes", BasisSenior, "OSCA-12345", Pesos(1120)),
		entry("2026-03-01", "OR-1001", "Corazon Reyes", BasisSenior, "OSCA-12345", Pesos(1120)),
		entry("2026-03-15", "OR-1002", "Noel Garcia", BasisPWD, "PWD-77-0042", Pesos(560)),
		entry("2026-03-15", "OR-1003", "Corazon Reyes", BasisSenior, "OSCA-12345", Pesos(2240)),
		entry("2026-03-31", "OR-1004", "Benigno Reyes", BasisSenior, "OSCA-12346", Pesos(1120)),
		entry("2026-04-01", "OR-1100", "Noel Garcia", BasisPWD, "PWD-77-0042", Pesos(1120)),
	}
}

func TestInPeriodBoundaries(t *testing.T) {
	sales := march()

	cases := []struct {
		name    string
		period  Period
		wantRef []string
	}{
		{
			name:    "a month takes its first and last day and nothing else",
			period:  mustMonth(t, 2026, time.March),
			wantRef: []string{"OR-1004", "OR-1002", "OR-1003", "OR-1001"},
		},
		{
			name:    "the month before",
			period:  mustMonth(t, 2026, time.February),
			wantRef: []string{"OR-0900"},
		},
		{
			name:    "a quarter takes all three months",
			period:  mustQuarter(t, 2026, 1),
			wantRef: []string{"OR-1004", "OR-1002", "OR-1003", "OR-1001", "OR-0900"},
		},
		{
			name:    "a single day",
			period:  mustCustom(t, "2026-03-15", "2026-03-15"),
			wantRef: []string{"OR-1002", "OR-1003"},
		},
		{
			name:    "a month with nothing in it",
			period:  mustMonth(t, 2026, time.January),
			wantRef: nil,
		},
		{
			name:    "a whole year",
			period:  mustYear(t, 2026),
			wantRef: []string{"OR-1100", "OR-1004", "OR-1002", "OR-1003", "OR-1001", "OR-0900"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := InPeriod(sales, c.period)
			if len(got) != len(c.wantRef) {
				t.Fatalf("got %d entries, want %d: %v", len(got), len(c.wantRef), refs(got))
			}
			for i := range got {
				if got[i].Reference != c.wantRef[i] {
					t.Errorf("entry %d = %s, want %s (whole order %v)", i, got[i].Reference, c.wantRef[i], refs(got))
				}
			}
		})
	}
}

func TestInPeriodDoesNotDisturbTheRegister(t *testing.T) {
	sales := march()
	before := refs(sales)

	InPeriod(sales, mustMonth(t, 2026, time.March))

	after := refs(sales)
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("reading a period reordered the register: %v became %v", before, after)
		}
	}
}

func TestSummariseTheMonth(t *testing.T) {
	got := Summarise(march(), mustMonth(t, 2026, time.March))

	// 1,120.00 is 1,000.00 net of tax, less 200.00.
	//   560.00 is   500.00 net of tax, less 100.00.
	// 2,240.00 is 2,000.00 net of tax, less 400.00.
	// Four entries: 1,120 + 560 + 2,240 + 1,120 = 5,040.00 gross.
	cases := []struct {
		label string
		got   Centavos
		want  Centavos
	}{
		{label: "gross", got: got.Total.Gross, want: Pesos(5040)},
		{label: "VAT-exempt", got: got.Total.VATExempt, want: Pesos(4500)},
		{label: "VAT not charged", got: got.Total.VATForgone, want: Pesos(540)},
		{label: "discounts granted", got: got.Total.Discount, want: Pesos(900)},
		{label: "net collected", got: got.Total.Net, want: Pesos(3600)},
		{label: "the deduction claimed", got: got.Deduction(), want: Pesos(900)},

		{label: "senior gross", got: got.Senior.Gross, want: Pesos(4480)},
		{label: "senior discounts", got: got.Senior.Discount, want: Pesos(800)},
		{label: "PWD gross", got: got.PWD.Gross, want: Pesos(560)},
		{label: "PWD discounts", got: got.PWD.Discount, want: Pesos(100)},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %s, want %s", c.label, c.got.Format(), c.want.Format())
		}
	}

	if got.Total.Entries != 4 {
		t.Errorf("entries = %d, want 4", got.Total.Entries)
	}
	if got.Total.Buyers != 3 {
		t.Errorf("distinct buyers = %d, want 3 — the same card twice is one buyer", got.Total.Buyers)
	}
	if got.Senior.Entries != 3 || got.Senior.Buyers != 2 {
		t.Errorf("senior tally = %d entries over %d buyers, want 3 over 2", got.Senior.Entries, got.Senior.Buyers)
	}
	if got.PWD.Entries != 1 || got.PWD.Buyers != 1 {
		t.Errorf("PWD tally = %d entries over %d buyers, want 1 over 1", got.PWD.Entries, got.PWD.Buyers)
	}
	if got.Senior.Discount+got.PWD.Discount != got.Total.Discount {
		t.Errorf("the two records do not add up to the total discount")
	}
	if got.Total.VATExempt+got.Total.VATForgone != got.Total.Gross {
		t.Errorf("the exempt sales and the tax not charged do not add back to the gross")
	}
	if got.Total.VATExempt-got.Total.Discount != got.Total.Net {
		t.Errorf("the exempt sales less the discounts is not the net collected")
	}
}

func TestSummariseByMonth(t *testing.T) {
	got := Summarise(march(), mustQuarter(t, 2026, 1))

	if len(got.Months) != 3 {
		t.Fatalf("a quarter broke into %d months", len(got.Months))
	}
	want := []struct {
		label    string
		entries  int
		discount Centavos
	}{
		{label: "January 2026", entries: 0, discount: 0},
		{label: "February 2026", entries: 1, discount: Pesos(200)},
		{label: "March 2026", entries: 4, discount: Pesos(900)},
	}
	var total Centavos
	for i, w := range want {
		month := got.Months[i]
		if month.Label != w.label {
			t.Errorf("month %d = %q, want %q", i, month.Label, w.label)
		}
		if month.Tally.Entries != w.entries {
			t.Errorf("%s has %d entries, want %d", month.Label, month.Tally.Entries, w.entries)
		}
		if month.Tally.Discount != w.discount {
			t.Errorf("%s discounts = %s, want %s", month.Label, month.Tally.Discount.Format(), w.discount.Format())
		}
		total += month.Tally.Discount
	}
	if total != got.Total.Discount {
		t.Errorf("the months total %s, the period says %s", total.Format(), got.Total.Discount.Format())
	}
}

func TestSummariseEmptyPeriod(t *testing.T) {
	got := Summarise(march(), mustMonth(t, 2026, time.January))

	if got.Total.Entries != 0 || got.Total.Discount != 0 || got.Deduction() != 0 {
		t.Fatalf("an empty month summarised to %+v", got.Total)
	}
	if got.Total.Buyers != 0 {
		t.Fatalf("an empty month had %d buyers", got.Total.Buyers)
	}
	if !containsSubstring(got.Notes, "No entries fall in this period") {
		t.Fatalf("an empty month does not say so: %v", got.Notes)
	}
}

func TestSummariseNoSalesAtAll(t *testing.T) {
	got := Summarise(nil, mustYear(t, 2026))
	if got.Total.Entries != 0 {
		t.Fatalf("an empty register summarised to %d entries", got.Total.Entries)
	}
	if len(got.Months) != 12 {
		t.Fatalf("a year broke into %d months", len(got.Months))
	}
}

func TestSummariseFlagsMixedRates(t *testing.T) {
	sales := march()

	t.Run("one set of rates passes quietly", func(t *testing.T) {
		got := Summarise(sales, mustMonth(t, 2026, time.March))
		if len(got.Rates) != 1 {
			t.Fatalf("rates = %v, want one entry", got.Rates)
		}
		if containsSubstring(got.Notes, "More than one pair of rates") {
			t.Fatalf("a single-rate period was flagged: %v", got.Notes)
		}
	})

	t.Run("a rate change inside the period is said out loud", func(t *testing.T) {
		mixed := append([]Sale(nil), sales...)
		odd := entry("2026-03-20", "OR-1050", "Corazon Reyes", BasisSenior, "OSCA-12345", Pesos(1100))
		odd.VATRate = 10
		mixed = append(mixed, odd)

		got := Summarise(mixed, mustMonth(t, 2026, time.March))
		if len(got.Rates) != 2 {
			t.Fatalf("rates = %v, want two entries", got.Rates)
		}
		if !containsSubstring(got.Notes, "More than one pair of rates") {
			t.Fatalf("a mixed-rate period was not flagged: %v", got.Notes)
		}
	})
}

func TestSummaryNotesNameTheDeduction(t *testing.T) {
	got := Summarise(march(), mustMonth(t, 2026, time.March))
	joined := strings.Join(got.Notes, " | ")
	for _, want := range []string{"deduction from gross income", "RA 9994", "RA 10754", "not a tax credit"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the notes do not mention %q: %s", want, joined)
		}
	}
}

func TestRegisterRowsAndTotals(t *testing.T) {
	sales := InPeriod(march(), mustMonth(t, 2026, time.March))
	columns := RegisterColumns()
	rows := RegisterRows(sales)

	if len(rows) != 4 {
		t.Fatalf("got %d rows for 4 entries", len(rows))
	}
	for i, row := range rows {
		if len(row) != len(columns) {
			t.Fatalf("row %d has %d cells for %d columns", i, len(row), len(columns))
		}
	}

	// The statutory columns, in the order the regulations list them.
	for i, want := range []string{"Date", "Receipt no.", "Buyer name", "Basis", "ID no."} {
		if columns[i] != want {
			t.Errorf("column %d = %q, want %q", i, columns[i], want)
		}
	}
	for _, want := range []string{"Gross sales (VAT-incl.)", "VAT-exempt sales", "Discount granted", "Net sales"} {
		if !containsString(columns, want) {
			t.Errorf("the register has no %q column: %v", want, columns)
		}
	}

	// The first row is the latest day, 31 March, 1,120.00 gross.
	first := rows[0]
	if first[0] != "2026-03-31" || first[1] != "OR-1004" || first[2] != "Benigno Reyes" {
		t.Errorf("the first row is %v", first[:3])
	}
	if first[3] != "Senior citizen" {
		t.Errorf("basis column = %q", first[3])
	}
	if first[6] != "1120.00" || first[8] != "1000.00" || first[9] != "200.00" || first[10] != "800.00" {
		t.Errorf("the amounts in the first row are %v", first[6:11])
	}

	totals := RegisterTotalsRow(Summarise(march(), mustMonth(t, 2026, time.March)).Total)
	if len(totals) != len(columns) {
		t.Fatalf("the totals row has %d cells for %d columns", len(totals), len(columns))
	}
	if !strings.Contains(totals[2], "4 entries") {
		t.Errorf("the totals row does not count the entries: %q", totals[2])
	}
	if totals[6] != "5040.00" || totals[8] != "4500.00" || totals[9] != "900.00" || totals[10] != "3600.00" {
		t.Errorf("the totals row reads %v", totals[6:11])
	}
}

func TestRegisterRowCarriesTheWorking(t *testing.T) {
	sale := entry("2026-03-15", "OR-1003", "Corazon Reyes", BasisSenior, "OSCA-12345", Pesos(2240))
	sale.Persons = 4
	sale.Allocation = AllocationItemised
	sale.Booklet = "BK-7"
	sale.Note = "two cards shown"

	row := RegisterRow(sale)
	if !containsString(row, "4") {
		t.Errorf("the row does not carry the headcount: %v", row)
	}
	if !containsString(row, "Itemised") {
		t.Errorf("the row does not carry the share basis: %v", row)
	}
	if !containsString(row, "BK-7") {
		t.Errorf("the row does not carry the booklet number: %v", row)
	}
	if !containsString(row, "12% VAT / 20% discount") {
		t.Errorf("the row does not carry the rates it was worked out on: %v", row)
	}
	if !containsString(row, "two cards shown") {
		t.Errorf("the row does not carry the note: %v", row)
	}
}

func TestSortSalesIsStableAndOrdered(t *testing.T) {
	sales := []Sale{
		entry("2026-03-15", "OR-1003", "Zoraida Cruz", BasisSenior, "OSCA-3", Pesos(1120)),
		entry("2026-03-15", "OR-1003", "Amado Cruz", BasisSenior, "OSCA-4", Pesos(1120)),
		entry("2026-03-15", "OR-1002", "Noel Garcia", BasisPWD, "PWD-1", Pesos(1120)),
		entry("2026-03-16", "OR-1004", "Corazon Reyes", BasisSenior, "OSCA-1", Pesos(1120)),
	}
	SortSales(sales)

	want := []string{"OR-1004", "OR-1002", "OR-1003", "OR-1003"}
	for i := range want {
		if sales[i].Reference != want[i] {
			t.Fatalf("order = %v, want %v", refs(sales), want)
		}
	}
	// Within one receipt, by buyer name.
	if sales[2].BuyerName != "Amado Cruz" || sales[3].BuyerName != "Zoraida Cruz" {
		t.Fatalf("the two buyers on OR-1003 came out as %q then %q", sales[2].BuyerName, sales[3].BuyerName)
	}
}

func TestAllocationLabel(t *testing.T) {
	cases := []struct {
		in   Allocation
		want string
	}{
		{in: AllocationPerHead, want: "Per head"},
		{in: AllocationItemised, want: "Itemised"},
		{in: "halves", want: "halves"},
	}
	for _, c := range cases {
		if got := allocationLabel(c.in); got != c.want {
			t.Errorf("allocationLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if AllocationPerHead.Valid() != true || AllocationItemised.Valid() != true || Allocation("halves").Valid() {
		t.Error("Allocation.Valid() admits the wrong set")
	}
}

// --- helpers -------------------------------------------------------------

func refs(sales []Sale) []string {
	out := make([]string, 0, len(sales))
	for _, s := range sales {
		out = append(out, s.Reference)
	}
	return out
}

func containsString(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}

func containsSubstring(all []string, want string) bool {
	for _, s := range all {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
