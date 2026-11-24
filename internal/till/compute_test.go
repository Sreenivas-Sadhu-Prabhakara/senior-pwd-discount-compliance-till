package till

import (
	"strings"
	"testing"
)

// senior and pwd build a qualified buyer for the worked examples below.
func senior(name, id string) Buyer {
	return Buyer{Name: name, Basis: BasisSenior, IDNo: id}
}

func pwd(name, id string) Buyer {
	return Buyer{Name: name, Basis: BasisPWD, IDNo: id}
}

func coveredLine(desc string, amount Centavos) Line {
	return Line{Description: desc, Amount: amount, Covered: true}
}

// TestComputeWorkedExamples is the heart of this package: the order of
// operations against examples worked out by hand.
//
// For a qualified buyer the sale is exempt from value-added tax FIRST, and the
// 20 per cent is taken on what is left. A bill of 1,120.00 is therefore
// 1,000.00 net of the 12 per cent, less 200.00, so 800.00 is due — not
// 1,120.00 less 224.00 (which would be 896.00), and not 1,000.00 less 200.00
// with the tax then added back on (which would be 896.00 as well).
func TestComputeWorkedExamples(t *testing.T) {
	cases := []struct {
		name  string
		bill  Bill
		rates Rates

		wantCoveredGross    Centavos
		wantNotCoveredGross Centavos
		wantQualifiedGross  Centavos
		wantVATRemoved      Centavos
		wantDiscountBase    Centavos
		wantDiscount        Centavos
		wantQualifiedDue    Centavos
		wantOthersDue       Centavos
		wantTotalDue        Centavos
		wantSaved           Centavos
		wantOutputVAT       Centavos
	}{
		{
			// The illustration in the regulations: one senior citizen, dining
			// alone, on a VAT-inclusive bill of 1,120.00.
			name: "one senior citizen alone, the regulations' own example",
			bill: Bill{
				Date:       MustParseDate("2026-03-15"),
				Reference:  "OR-1001",
				Lines:      []Line{coveredLine("Lunch", Pesos(1120))},
				Persons:    1,
				Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
				Allocation: AllocationPerHead,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   Pesos(1120),
			wantQualifiedGross: Pesos(1120),
			wantVATRemoved:     Pesos(120),
			wantDiscountBase:   Pesos(1000),
			wantDiscount:       Pesos(200),
			wantQualifiedDue:   Pesos(800),
			wantTotalDue:       Pesos(800),
			wantSaved:          Pesos(320),
		},
		{
			// The same arithmetic for a person with disability under RA 10754.
			name: "one person with disability alone",
			bill: Bill{
				Date:       MustParseDate("2026-03-15"),
				Reference:  "OR-1002",
				Lines:      []Line{coveredLine("Dinner", Pesos(1120))},
				Persons:    1,
				Buyers:     []Buyer{pwd("Noel Garcia", "PWD-77-0042")},
				Allocation: AllocationPerHead,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   Pesos(1120),
			wantQualifiedGross: Pesos(1120),
			wantVATRemoved:     Pesos(120),
			wantDiscountBase:   Pesos(1000),
			wantDiscount:       Pesos(200),
			wantQualifiedDue:   Pesos(800),
			wantTotalDue:       Pesos(800),
			wantSaved:          Pesos(320),
		},
		{
			// A shared bill where only one of four diners qualifies. The bill
			// is divided per head and only that one share is exempt and
			// discounted; the other three shares keep their tax and get
			// nothing off.
			name: "shared bill, four diners, one senior citizen",
			bill: Bill{
				Date:       MustParseDate("2026-03-15"),
				Reference:  "OR-1003",
				Lines:      []Line{coveredLine("Family set meal", Pesos(2240))},
				Persons:    4,
				Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
				Allocation: AllocationPerHead,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   Pesos(2240),
			wantQualifiedGross: Pesos(560),
			wantVATRemoved:     Pesos(60),
			wantDiscountBase:   Pesos(500),
			wantDiscount:       Pesos(100),
			wantQualifiedDue:   Pesos(400),
			wantOthersDue:      Pesos(1680),
			wantTotalDue:       Pesos(2080),
			wantSaved:          Pesos(160),
			wantOutputVAT:      Pesos(180),
		},
		{
			// Two qualified buyers in a group of five, one under each law, and
			// 500.00 of the bill is something the statute does not cover.
			name: "group of five, a senior and a PWD, plus items not covered",
			bill: Bill{
				Date:      MustParseDate("2026-03-16"),
				Reference: "OR-1004",
				Lines: []Line{
					coveredLine("Shared dishes", Pesos(3500)),
					{Description: "Bottled spirits", Amount: Pesos(500)},
				},
				Persons: 5,
				Buyers: []Buyer{
					senior("Corazon Reyes", "OSCA-12345"),
					pwd("Noel Garcia", "PWD-77-0042"),
				},
				Allocation: AllocationPerHead,
			},
			rates:               DefaultRates(),
			wantCoveredGross:    Pesos(3500),
			wantNotCoveredGross: Pesos(500),
			wantQualifiedGross:  Pesos(1400),
			wantVATRemoved:      Pesos(150),
			wantDiscountBase:    Pesos(1250),
			wantDiscount:        Pesos(250),
			wantQualifiedDue:    Pesos(1000),
			wantOthersDue:       Pesos(2600),
			wantTotalDue:        Pesos(3600),
			wantSaved:           Pesos(400),
			wantOutputVAT:       27857, // 2,600.00 is 2,321.43 net of 12%
		},
		{
			// A bill that divides into thirds, so the odd centavo and the
			// rounding both have to be right. 1,000.00 / 3 = 333.34 to the
			// senior; / 1.12 = 297.63; less 20% = 59.53; 238.10 due.
			name: "shared bill that does not divide evenly",
			bill: Bill{
				Date:       MustParseDate("2026-03-17"),
				Reference:  "OR-1005",
				Lines:      []Line{coveredLine("Merienda for three", Pesos(1000))},
				Persons:    3,
				Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
				Allocation: AllocationPerHead,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   Pesos(1000),
			wantQualifiedGross: 33334,
			wantVATRemoved:     3571,
			wantDiscountBase:   29763,
			wantDiscount:       5953,
			wantQualifiedDue:   23810,
			wantOthersDue:      66666,
			wantTotalDue:       90476,
			wantSaved:          9524,
			wantOutputVAT:      7143, // 666.66 is 595.23 net of 12%
		},
		{
			// Everybody at the table qualifies, so the whole covered bill is
			// exempt and discounted and nothing is left to collect tax on.
			name: "every diner qualifies",
			bill: Bill{
				Date:      MustParseDate("2026-03-18"),
				Reference: "OR-1006",
				Lines:     []Line{coveredLine("Two set meals", Pesos(2240))},
				Persons:   2,
				Buyers: []Buyer{
					senior("Corazon Reyes", "OSCA-12345"),
					senior("Benigno Reyes", "OSCA-12346"),
				},
				Allocation: AllocationPerHead,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   Pesos(2240),
			wantQualifiedGross: Pesos(2240),
			wantVATRemoved:     Pesos(240),
			wantDiscountBase:   Pesos(2000),
			wantDiscount:       Pesos(400),
			wantQualifiedDue:   Pesos(1600),
			wantTotalDue:       Pesos(1600),
			wantSaved:          Pesos(640),
		},
		{
			// The bill itemises what the senior citizen actually had, which is
			// better evidence than a per-head share.
			name: "itemised consumption on a shared bill",
			bill: Bill{
				Date:       MustParseDate("2026-03-19"),
				Reference:  "OR-1007",
				Lines:      []Line{coveredLine("Table bill", Pesos(1120))},
				Persons:    3,
				Buyers:     []Buyer{{Name: "Corazon Reyes", Basis: BasisSenior, IDNo: "OSCA-12345", Share: Pesos(560)}},
				Allocation: AllocationItemised,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   Pesos(1120),
			wantQualifiedGross: Pesos(560),
			wantVATRemoved:     Pesos(60),
			wantDiscountBase:   Pesos(500),
			wantDiscount:       Pesos(100),
			wantQualifiedDue:   Pesos(400),
			wantOthersDue:      Pesos(560),
			wantTotalDue:       Pesos(960),
			wantSaved:          Pesos(160),
			wantOutputVAT:      Pesos(60),
		},
		{
			// An establishment that is not registered for value-added tax
			// quotes prices with no tax inside them, so there is nothing to
			// take out and the 20 per cent falls on the selling price.
			name: "establishment not registered for VAT",
			bill: Bill{
				Date:       MustParseDate("2026-03-20"),
				Reference:  "OR-1008",
				Lines:      []Line{coveredLine("Carinderia meal", Pesos(1000))},
				Persons:    1,
				Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
				Allocation: AllocationPerHead,
			},
			rates:              Rates{VATPercent: 12, DiscountPercent: 20, VATRegistered: false},
			wantCoveredGross:   Pesos(1000),
			wantQualifiedGross: Pesos(1000),
			wantVATRemoved:     0,
			wantDiscountBase:   Pesos(1000),
			wantDiscount:       Pesos(200),
			wantQualifiedDue:   Pesos(800),
			wantTotalDue:       Pesos(800),
			wantSaved:          Pesos(200),
		},
		{
			// A line that carries no tax in its own right sits beside one that
			// does. Only the VATable half has tax taken out; the discount is
			// then taken on both.
			name: "a covered line that carries no VAT of its own",
			bill: Bill{
				Date:      MustParseDate("2026-03-21"),
				Reference: "OR-1009",
				Lines: []Line{
					coveredLine("Room charge", Pesos(1120)),
					{Description: "Prescribed medicine", Amount: Pesos(500), Covered: true, VATExempt: true},
				},
				Persons:    1,
				Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
				Allocation: AllocationPerHead,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   Pesos(1620),
			wantQualifiedGross: Pesos(1620),
			wantVATRemoved:     Pesos(120),
			wantDiscountBase:   Pesos(1500),
			wantDiscount:       Pesos(300),
			wantQualifiedDue:   Pesos(1200),
			wantTotalDue:       Pesos(1200),
			wantSaved:          Pesos(420),
		},
		{
			// Nobody at the table qualifies: the bill is charged in full and
			// the tool must not find a discount anywhere.
			name: "no qualified buyer at all",
			bill: Bill{
				Date:      MustParseDate("2026-03-22"),
				Reference: "OR-1010",
				Lines: []Line{
					coveredLine("Lunch", Pesos(1120)),
					{Description: "Beer", Amount: Pesos(280)},
				},
				Persons:    2,
				Allocation: AllocationPerHead,
			},
			rates:               DefaultRates(),
			wantCoveredGross:    Pesos(1120),
			wantNotCoveredGross: Pesos(280),
			wantOthersDue:       Pesos(1400),
			wantTotalDue:        Pesos(1400),
			wantOutputVAT:       Pesos(150),
		},
		{
			// A zero bill: a comped meal still recorded. Nothing anywhere, and
			// no division by zero on the way.
			name: "a bill of nothing",
			bill: Bill{
				Date:       MustParseDate("2026-03-23"),
				Reference:  "OR-1011",
				Lines:      []Line{coveredLine("Complimentary meal", 0)},
				Persons:    1,
				Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
				Allocation: AllocationPerHead,
			},
			rates: DefaultRates(),
		},
		{
			// One centavo, to show the rounding does not invent money.
			name: "a bill of one centavo",
			bill: Bill{
				Date:       MustParseDate("2026-03-24"),
				Reference:  "OR-1012",
				Lines:      []Line{coveredLine("Rounding", 1)},
				Persons:    1,
				Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
				Allocation: AllocationPerHead,
			},
			rates:              DefaultRates(),
			wantCoveredGross:   1,
			wantQualifiedGross: 1,
			wantVATRemoved:     0,
			wantDiscountBase:   1,
			wantDiscount:       0,
			wantQualifiedDue:   1,
			wantTotalDue:       1,
			wantSaved:          0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Compute(c.bill, c.rates)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}

			check := func(label string, got, want Centavos) {
				t.Helper()
				if got != want {
					t.Errorf("%s = %s, want %s", label, got.Format(), want.Format())
				}
			}
			check("covered gross", got.CoveredGross, c.wantCoveredGross)
			check("not-covered gross", got.NotCoveredGross, c.wantNotCoveredGross)
			check("qualified gross", got.QualifiedGross, c.wantQualifiedGross)
			check("VAT removed", got.VATRemoved, c.wantVATRemoved)
			check("discount base", got.DiscountBase, c.wantDiscountBase)
			check("discount", got.Discount, c.wantDiscount)
			check("qualified due", got.QualifiedDue, c.wantQualifiedDue)
			check("others due", got.OthersDue, c.wantOthersDue)
			check("total due", got.TotalDue, c.wantTotalDue)
			check("saved", got.Saved, c.wantSaved)
			check("output VAT", got.OutputVAT, c.wantOutputVAT)

			// Invariants that must hold for every bill, whatever the figures.
			if got.QualifiedDue+got.OthersDue != got.TotalDue {
				t.Errorf("the parts %s and %s do not add to the total %s",
					got.QualifiedDue.Format(), got.OthersDue.Format(), got.TotalDue.Format())
			}
			if got.TotalDue+got.Saved != got.BillGross {
				t.Errorf("the total due %s plus what was saved %s is not the bill as priced %s",
					got.TotalDue.Format(), got.Saved.Format(), got.BillGross.Format())
			}
			if got.DiscountBase+got.VATRemoved != got.QualifiedGross {
				t.Errorf("the discount base %s plus the tax taken out %s is not the qualified share %s",
					got.DiscountBase.Format(), got.VATRemoved.Format(), got.QualifiedGross.Format())
			}
			if want := got.DiscountBase.Percent(c.rates.Normalise().DiscountPercent); len(got.Shares) <= 1 && got.Discount != want {
				t.Errorf("the discount %s is not %d%% of the base %s (%s)",
					got.Discount.Format(), c.rates.Normalise().DiscountPercent, got.DiscountBase.Format(), want.Format())
			}
			if got.Discount > got.DiscountBase {
				t.Errorf("the discount %s is more than the base %s", got.Discount.Format(), got.DiscountBase.Format())
			}
			if len(got.Shares) != len(c.bill.Buyers) {
				t.Errorf("got %d buyer shares for %d buyers", len(got.Shares), len(c.bill.Buyers))
			}
			if len(got.Steps) == 0 {
				t.Error("the computation produced no working")
			}
		})
	}
}

// TestComputeDoesNotDiscountTheVATInclusivePrice guards the single mistake
// this tool exists to prevent.
func TestComputeDoesNotDiscountTheVATInclusivePrice(t *testing.T) {
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-2001",
		Lines:      []Line{coveredLine("Lunch", Pesos(1120))},
		Persons:    1,
		Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
		Allocation: AllocationPerHead,
	}
	got, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	wrongDiscount := Pesos(1120).Percent(StatutoryDiscountPercent) // 224.00
	if got.Discount == wrongDiscount {
		t.Fatalf("the discount was taken on the VAT-inclusive price: %s", got.Discount.Format())
	}
	if got.Discount != Pesos(200) {
		t.Fatalf("the discount is %s, want 200.00 — 20%% of the 1,000.00 net of VAT", got.Discount.Format())
	}
	if got.TotalDue != Pesos(800) {
		t.Fatalf("the amount due is %s, want 800.00", got.TotalDue.Format())
	}

	// The common wrong answers, so a regression lands on one of them loudly.
	for _, wrong := range []Centavos{Pesos(896), Pesos(1000), Pesos(1120)} {
		if got.TotalDue == wrong {
			t.Fatalf("the amount due came out as %s, which is one of the wrong answers", wrong.Format())
		}
	}
}

func TestComputeRatesAreOverridable(t *testing.T) {
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-2002",
		Lines:      []Line{coveredLine("Lunch", Pesos(1100))},
		Persons:    1,
		Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
		Allocation: AllocationPerHead,
	}

	cases := []struct {
		name         string
		rates        Rates
		wantBase     Centavos
		wantDiscount Centavos
		wantNote     bool
	}{
		{name: "the statutory figures", rates: Rates{VATPercent: 12, DiscountPercent: 20, VATRegistered: true}, wantBase: 98214, wantDiscount: 19643},
		{name: "a 10% tax, as it once was", rates: Rates{VATPercent: 10, DiscountPercent: 20, VATRegistered: true}, wantBase: Pesos(1000), wantDiscount: Pesos(200), wantNote: true},
		{name: "a 25% discount, were one legislated", rates: Rates{VATPercent: 12, DiscountPercent: 25, VATRegistered: true}, wantBase: 98214, wantDiscount: 24554, wantNote: true},
		{name: "nonsense falls back to the statute", rates: Rates{VATPercent: -4, DiscountPercent: 0, VATRegistered: true}, wantBase: 98214, wantDiscount: 19643},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Compute(bill, c.rates)
			if err != nil {
				t.Fatal(err)
			}
			if got.DiscountBase != c.wantBase {
				t.Errorf("base = %s, want %s", got.DiscountBase.Format(), c.wantBase.Format())
			}
			if got.Discount != c.wantDiscount {
				t.Errorf("discount = %s, want %s", got.Discount.Format(), c.wantDiscount.Format())
			}
			flagged := false
			for _, note := range got.Notes {
				if strings.Contains(note, "not the figures this tool ships with") {
					flagged = true
				}
			}
			if flagged != c.wantNote {
				t.Errorf("overridden-rates note present = %v, want %v (notes: %v)", flagged, c.wantNote, got.Notes)
			}
		})
	}
}

func TestComputeValidation(t *testing.T) {
	ok := func() Bill {
		return Bill{
			Date:       MustParseDate("2026-03-15"),
			Reference:  "OR-3001",
			Lines:      []Line{coveredLine("Lunch", Pesos(1120))},
			Persons:    2,
			Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
			Allocation: AllocationPerHead,
		}
	}

	cases := []struct {
		name    string
		mangle  func(b *Bill)
		wantErr string
	}{
		{name: "a valid bill", mangle: func(b *Bill) {}},
		{name: "no headcount", mangle: func(b *Bill) { b.Persons = 0 }, wantErr: "how many people"},
		{name: "a negative headcount", mangle: func(b *Bill) { b.Persons = -3 }, wantErr: "how many people"},
		{name: "an absurd headcount", mangle: func(b *Bill) { b.Persons = 10000 }, wantErr: "more than this tool will divide"},
		{name: "no lines", mangle: func(b *Bill) { b.Lines = nil }, wantErr: "no lines on it"},
		{name: "a negative line", mangle: func(b *Bill) { b.Lines[0].Amount = -100 }, wantErr: "negative amount"},
		{name: "an absurd line", mangle: func(b *Bill) { b.Lines[0].Amount = MaxAmount + 1 }, wantErr: "larger than this tool"},
		{name: "an unknown allocation", mangle: func(b *Bill) { b.Allocation = "halves" }, wantErr: "not a way of allocating"},
		{name: "more buyers than people", mangle: func(b *Bill) { b.Persons = 1; b.Buyers = append(b.Buyers, senior("B", "OSCA-2")) }, wantErr: "qualified buyers on a bill shared by"},
		{name: "nothing covered", mangle: func(b *Bill) { b.Lines[0].Covered = false }, wantErr: "no line on this bill is marked as covered"},
		{name: "a buyer with no name", mangle: func(b *Bill) { b.Buyers[0].Name = "  " }, wantErr: "no name"},
		{name: "a buyer with no basis", mangle: func(b *Bill) { b.Buyers[0].Basis = "" }, wantErr: "senior citizen or as a person with disability"},
		{name: "a basis that is not a law", mangle: func(b *Bill) { b.Buyers[0].Basis = "student" }, wantErr: "senior citizen or as a person with disability"},
		{name: "a buyer with no ID number", mangle: func(b *Bill) { b.Buyers[0].IDNo = "" }, wantErr: "identification number has to be taken"},
		{
			name: "the same card twice",
			mangle: func(b *Bill) {
				b.Persons = 3
				b.Buyers = append(b.Buyers, senior("Corazon Reyes", "osca-12345"))
			},
			wantErr: "appears twice",
		},
		{
			name: "itemised with no share stated",
			mangle: func(b *Bill) {
				b.Allocation = AllocationItemised
			},
			wantErr: "say what they consumed",
		},
		{name: "a negative promotion", mangle: func(b *Bill) { b.Promotion = -100 }, wantErr: "cannot be a negative amount"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bill := ok()
			c.mangle(&bill)

			_, err := Compute(bill, DefaultRates())
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Compute: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Compute accepted the bill, wanted %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error %q does not mention %q", err, c.wantErr)
			}
		})
	}
}

func TestComputeItemisedCannotClaimMoreThanTheBill(t *testing.T) {
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-3002",
		Lines:      []Line{coveredLine("Table bill", Pesos(1000))},
		Persons:    2,
		Buyers:     []Buyer{{Name: "Corazon Reyes", Basis: BasisSenior, IDNo: "OSCA-12345", Share: Pesos(1500)}},
		Allocation: AllocationItemised,
	}
	_, err := Compute(bill, DefaultRates())
	if err == nil {
		t.Fatal("a buyer claiming more than the whole bill was accepted")
	}
	if !strings.Contains(err.Error(), "more than the") {
		t.Fatalf("error %q does not explain the over-claim", err)
	}
}

func TestComputePromotionIsAnAlternativeNotAnAddition(t *testing.T) {
	base := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-4001",
		Lines:      []Line{coveredLine("Set meal", Pesos(1120))},
		Persons:    1,
		Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
		Allocation: AllocationPerHead,
	}

	cases := []struct {
		name          string
		promotion     Centavos
		wantChoice    bool
		wantStatutory bool
		wantBenefit   Centavos
	}{
		{name: "no promotion at all", promotion: 0, wantChoice: false},
		{name: "a small promotion loses to the statute", promotion: Pesos(100), wantChoice: true, wantStatutory: true, wantBenefit: Pesos(100)},
		{name: "a large promotion wins", promotion: Pesos(400), wantChoice: true, wantStatutory: false, wantBenefit: Pesos(400)},
		{name: "an equal promotion leaves the statute standing", promotion: Pesos(320), wantChoice: true, wantStatutory: true, wantBenefit: Pesos(320)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bill := base
			bill.Promotion = c.promotion

			got, err := Compute(bill, DefaultRates())
			if err != nil {
				t.Fatal(err)
			}
			if (got.Promotion != nil) != c.wantChoice {
				t.Fatalf("promotion comparison present = %v, want %v", got.Promotion != nil, c.wantChoice)
			}

			// Whatever the promotion, the charged figures never change: the
			// two benefits are alternatives and must never be added up.
			if got.Discount != Pesos(200) || got.TotalDue != Pesos(800) {
				t.Fatalf("the promotion changed the statutory figures: discount %s, due %s",
					got.Discount.Format(), got.TotalDue.Format())
			}
			if !c.wantChoice {
				return
			}
			if got.Promotion.StatutoryIsHigher != c.wantStatutory {
				t.Errorf("statutory is higher = %v, want %v", got.Promotion.StatutoryIsHigher, c.wantStatutory)
			}
			if got.Promotion.PromotionBenefit != c.wantBenefit {
				t.Errorf("promotion benefit = %s, want %s", got.Promotion.PromotionBenefit.Format(), c.wantBenefit.Format())
			}
			if got.Promotion.StatutoryBenefit != Pesos(320) {
				t.Errorf("statutory benefit = %s, want 320.00", got.Promotion.StatutoryBenefit.Format())
			}
			if !strings.Contains(got.Promotion.Advice, "do not add up") {
				t.Errorf("the advice does not say the two do not add up: %q", got.Promotion.Advice)
			}
		})
	}
}

func TestComputePromotionIsApportionedToTheQualifiedShare(t *testing.T) {
	// A promotion is offered on the whole bill, but only the part of it
	// falling on the qualified buyer's share is comparable with the statute.
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-4002",
		Lines:      []Line{coveredLine("Family set meal", Pesos(2240))},
		Persons:    4,
		Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
		Allocation: AllocationPerHead,
		Promotion:  Pesos(400),
	}
	got, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}
	if got.Promotion == nil {
		t.Fatal("no promotion comparison")
	}
	if want := Pesos(100); got.Promotion.PromotionBenefit != want {
		t.Fatalf("promotion benefit = %s, want %s (a quarter of 400.00)",
			got.Promotion.PromotionBenefit.Format(), want.Format())
	}
	if !got.Promotion.StatutoryIsHigher {
		t.Fatalf("160.00 under the statute should beat 100.00 from the promotion")
	}
}

func TestComputeStepsFollowTheStatutoryOrder(t *testing.T) {
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-5001",
		Lines:      []Line{coveredLine("Family set meal", Pesos(2240))},
		Persons:    4,
		Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
		Allocation: AllocationPerHead,
	}
	got, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	wantOrder := []struct {
		fragment string
		amount   Centavos
	}{
		{fragment: "Covered goods and services, as priced", amount: Pesos(2240)},
		{fragment: "Qualified share", amount: Pesos(560)},
		{fragment: "Less value-added tax", amount: Pesos(60)},
		{fragment: "VAT-exempt selling price", amount: Pesos(500)},
		{fragment: "Less statutory discount", amount: Pesos(100)},
		{fragment: "Amount due from", amount: Pesos(400)},
		{fragment: "Other customers' share", amount: Pesos(1680)},
		{fragment: "Total amount due", amount: Pesos(2080)},
	}
	if len(got.Steps) != len(wantOrder) {
		t.Fatalf("got %d steps, want %d: %v", len(got.Steps), len(wantOrder), stepLabels(got.Steps))
	}
	for i, want := range wantOrder {
		if !strings.Contains(got.Steps[i].Label, want.fragment) {
			t.Errorf("step %d is %q, want it to mention %q", i, got.Steps[i].Label, want.fragment)
		}
		if got.Steps[i].Amount != want.amount {
			t.Errorf("step %d (%s) = %s, want %s", i, got.Steps[i].Label, got.Steps[i].Amount.Format(), want.amount.Format())
		}
	}

	// The tax must come out before the discount goes on, or the whole thing is
	// wrong. Assert the positions, not just the presence.
	vatStep, discountStep := -1, -1
	for i, step := range got.Steps {
		if strings.Contains(step.Label, "Less value-added tax") {
			vatStep = i
		}
		if strings.Contains(step.Label, "Less statutory discount") {
			discountStep = i
		}
	}
	if vatStep < 0 || discountStep < 0 || vatStep > discountStep {
		t.Fatalf("the VAT step is at %d and the discount step at %d — the tax must come out first", vatStep, discountStep)
	}
}

func TestSalesFromOneEntryPerQualifiedBuyer(t *testing.T) {
	bill := Bill{
		Date:      MustParseDate("2026-03-16"),
		Reference: "OR-6001",
		Lines:     []Line{coveredLine("Shared dishes", Pesos(3500))},
		Persons:   5,
		Buyers: []Buyer{
			senior("Corazon Reyes", "OSCA-12345"),
			pwd("Noel Garcia", "PWD-77-0042"),
		},
		Allocation: AllocationPerHead,
		Note:       "two cards shown",
	}
	got, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	sales := SalesFrom(bill, got)
	if len(sales) != 2 {
		t.Fatalf("got %d entries for 2 qualified buyers", len(sales))
	}

	var discount Centavos
	for _, sale := range sales {
		if sale.Reference != "OR-6001" || sale.Date.String() != "2026-03-16" {
			t.Errorf("entry lost its receipt or date: %+v", sale)
		}
		if sale.Persons != 5 || sale.Allocation != AllocationPerHead {
			t.Errorf("entry lost the working: %d persons, %q", sale.Persons, sale.Allocation)
		}
		if sale.VATRate != VATRatePercent || sale.DiscountRate != StatutoryDiscountPercent {
			t.Errorf("entry did not keep the rates it was worked out on: %+v", sale)
		}
		if sale.Gross != Pesos(700) || sale.VATExempt != Pesos(625) || sale.Discount != Pesos(125) || sale.Net != Pesos(500) {
			t.Errorf("entry figures wrong: gross %s, exempt %s, discount %s, net %s",
				sale.Gross.Format(), sale.VATExempt.Format(), sale.Discount.Format(), sale.Net.Format())
		}
		if sale.VATForgone() != Pesos(75) {
			t.Errorf("VAT forgone = %s, want 75.00", sale.VATForgone().Format())
		}
		if err := sale.Validate(MustParseDate("2026-03-31")); err != nil {
			t.Errorf("an entry this tool produced does not pass its own validation: %v", err)
		}
		discount += sale.Discount
	}
	if discount != got.Discount {
		t.Fatalf("the entries total %s of discount, the computation said %s", discount.Format(), got.Discount.Format())
	}

	if sales[0].Basis != BasisSenior || sales[1].Basis != BasisPWD {
		t.Fatalf("the entries lost which law was claimed under: %q, %q", sales[0].Basis, sales[1].Basis)
	}
}

func TestComputeNotesAreHonestAboutCoverage(t *testing.T) {
	bill := Bill{
		Date:       MustParseDate("2026-03-15"),
		Reference:  "OR-7001",
		Lines:      []Line{coveredLine("Lunch", Pesos(1120)), {Description: "Beer", Amount: Pesos(280)}},
		Persons:    2,
		Buyers:     []Buyer{senior("Corazon Reyes", "OSCA-12345")},
		Allocation: AllocationPerHead,
	}
	got, err := Compute(bill, DefaultRates())
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(got.Notes, " | ")
	for _, want := range []string{"set by RA 9994, RA 10754 and their implementing rules", "marked as not covered", "divided between 2 people"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the notes do not say %q: %s", want, joined)
		}
	}
}

func TestBasisLabels(t *testing.T) {
	cases := []struct {
		basis     Basis
		valid     bool
		label     string
		statute   string
		idMention string
	}{
		{basis: BasisSenior, valid: true, label: "Senior citizen", statute: "RA 9994", idMention: "OSCA"},
		{basis: BasisPWD, valid: true, label: "Person with disability", statute: "RA 10754", idMention: "PWD"},
		{basis: "student", valid: false, label: "student", statute: "", idMention: "ID no."},
	}
	for _, c := range cases {
		t.Run(string(c.basis), func(t *testing.T) {
			if c.basis.Valid() != c.valid {
				t.Errorf("Valid() = %v, want %v", c.basis.Valid(), c.valid)
			}
			if c.basis.Label() != c.label {
				t.Errorf("Label() = %q, want %q", c.basis.Label(), c.label)
			}
			if c.basis.Statute() != c.statute {
				t.Errorf("Statute() = %q, want %q", c.basis.Statute(), c.statute)
			}
			if !strings.Contains(c.basis.IDLabel(), c.idMention) {
				t.Errorf("IDLabel() = %q, want it to mention %q", c.basis.IDLabel(), c.idMention)
			}
		})
	}
}

func TestStatutoryConstantsAreTheFiguresInForce(t *testing.T) {
	if VATRatePercent != 12 {
		t.Errorf("the VAT rate constant is %d, not the 12%% in force", VATRatePercent)
	}
	if StatutoryDiscountPercent != 20 {
		t.Errorf("the discount constant is %d, not the 20%% in force", StatutoryDiscountPercent)
	}
	if SharesPerPersonInGroup != 1 {
		t.Errorf("a shared bill is divided into one share per person, not %d", SharesPerPersonInGroup)
	}
	if DiscountsPerQualifiedBuyer != 1 {
		t.Errorf("a buyer claims one discount, not %d", DiscountsPerQualifiedBuyer)
	}
	if !DiscountAppliesToVATExclusiveAmount {
		t.Error("the discount applies to the VAT-exclusive amount")
	}
	if !PromotionalDiscountIsAnAlternative {
		t.Error("a promotional discount is an alternative to the statutory one")
	}
	if !DiscountIsADeductionFromGrossIncome {
		t.Error("the discounts granted are a deduction from gross income")
	}

	d := DefaultRates()
	if !d.IsStatutory() || d.VATPercent != VATRatePercent || d.DiscountPercent != StatutoryDiscountPercent || !d.VATRegistered {
		t.Fatalf("DefaultRates() = %+v, which are not the statutory figures", d)
	}
	if (Rates{VATPercent: 10, DiscountPercent: 20}).IsStatutory() {
		t.Error("overridden rates reported themselves as statutory")
	}
}

func TestRatesNormalise(t *testing.T) {
	cases := []struct {
		name         string
		in           Rates
		wantVAT      int
		wantDiscount int
	}{
		{name: "left alone", in: Rates{VATPercent: 12, DiscountPercent: 20}, wantVAT: 12, wantDiscount: 20},
		{name: "a zero tax rate is allowed", in: Rates{VATPercent: 0, DiscountPercent: 20}, wantVAT: 0, wantDiscount: 20},
		{name: "a missing discount falls back", in: Rates{VATPercent: 12}, wantVAT: 12, wantDiscount: 20},
		{name: "a negative tax rate falls back", in: Rates{VATPercent: -1, DiscountPercent: 20}, wantVAT: 12, wantDiscount: 20},
		{name: "an impossible tax rate falls back", in: Rates{VATPercent: 140, DiscountPercent: 20}, wantVAT: 12, wantDiscount: 20},
		{name: "an impossible discount falls back", in: Rates{VATPercent: 12, DiscountPercent: 300}, wantVAT: 12, wantDiscount: 20},
		{name: "nothing at all falls back to the statute", in: Rates{}, wantVAT: 0, wantDiscount: 20},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.in.Normalise()
			if got.VATPercent != c.wantVAT || got.DiscountPercent != c.wantDiscount {
				t.Fatalf("Normalise() = %d%% VAT, %d%% discount; want %d%% and %d%%",
					got.VATPercent, got.DiscountPercent, c.wantVAT, c.wantDiscount)
			}
		})
	}
}

func stepLabels(steps []Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Label)
	}
	return out
}
