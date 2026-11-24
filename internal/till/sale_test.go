package till

import (
	"strings"
	"testing"
)

// goodSale is an entry that adds up: 700.00 gross, 625.00 net of 12% tax,
// 125.00 off, 500.00 paid.
func goodSale() Sale {
	return Sale{
		ID:           "s1",
		Date:         MustParseDate("2026-03-16"),
		Reference:    "OR-1004",
		BuyerName:    "Corazon Reyes",
		Basis:        BasisSenior,
		IDNo:         "OSCA-12345",
		Persons:      5,
		Allocation:   AllocationPerHead,
		Gross:        Pesos(700),
		VATExempt:    Pesos(625),
		Discount:     Pesos(125),
		Net:          Pesos(500),
		VATRate:      12,
		DiscountRate: 20,
	}
}

func TestSaleValidate(t *testing.T) {
	today := MustParseDate("2026-03-31")

	cases := []struct {
		name    string
		mangle  func(s *Sale)
		wantErr string
	}{
		{name: "an entry that adds up", mangle: func(s *Sale) {}},
		{name: "no date", mangle: func(s *Sale) { s.Date = Date{} }, wantErr: "needs the date"},
		{name: "a date in the future", mangle: func(s *Sale) { s.Date = MustParseDate("2026-04-01") }, wantErr: "in the future"},
		{name: "today is not the future", mangle: func(s *Sale) { s.Date = today }},
		{name: "no buyer name", mangle: func(s *Sale) { s.BuyerName = "" }, wantErr: "needs the buyer's name"},
		{name: "no basis", mangle: func(s *Sale) { s.Basis = "" }, wantErr: "senior citizen or as a person with disability"},
		{name: "no ID number", mangle: func(s *Sale) { s.IDNo = "" }, wantErr: "needs the osca"},
		{
			name:    "no PWD ID number names the right card",
			mangle:  func(s *Sale) { s.Basis = BasisPWD; s.IDNo = "" },
			wantErr: "needs the pwd id no.",
		},
		{name: "nothing sold", mangle: func(s *Sale) { s.Gross = 0 }, wantErr: "more than nothing"},
		{name: "a negative gross", mangle: func(s *Sale) { s.Gross = -100 }, wantErr: "more than nothing"},
		{name: "an absurd gross", mangle: func(s *Sale) { s.Gross = MaxAmount + 1 }, wantErr: "larger than this tool"},
		{name: "a negative discount", mangle: func(s *Sale) { s.Discount = -100 }, wantErr: "no amount on the entry can be negative"},
		{
			name:    "the exempt amount larger than the gross",
			mangle:  func(s *Sale) { s.VATExempt = Pesos(800); s.Discount = Pesos(160); s.Net = Pesos(640) },
			wantErr: "is more than the gross",
		},
		{
			name:    "a discount larger than the base it is taken on",
			mangle:  func(s *Sale) { s.Discount = Pesos(700); s.Net = Pesos(-75) },
			wantErr: "no amount on the entry can be negative",
		},
		{
			name:    "the net does not follow from the discount",
			mangle:  func(s *Sale) { s.Net = Pesos(550) },
			wantErr: "do not add up",
		},
		{
			// The mistake the whole tool exists to catch, caught again here:
			// 20 per cent of the VAT-inclusive 700.00 is 140.00, not 125.00.
			name:    "the discount taken on the VAT-inclusive price",
			mangle:  func(s *Sale) { s.Discount = Pesos(140); s.Net = Pesos(485) },
			wantErr: "at 20% is 125.00, not 140.00",
		},
		{
			name:    "a discount that is not the stated percentage",
			mangle:  func(s *Sale) { s.Discount = Pesos(100); s.Net = Pesos(525) },
			wantErr: "at 20% is 125.00, not 100.00",
		},
		{
			name: "an entry at an overridden rate, consistent with itself",
			mangle: func(s *Sale) {
				s.DiscountRate = 25
				s.Discount = Pesos(156) + 25
				s.Net = s.VATExempt - s.Discount
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sale := goodSale()
			c.mangle(&sale)

			err := sale.Validate(today)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate accepted the entry, wanted %q", c.wantErr)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.wantErr)) {
				t.Fatalf("error %q does not mention %q", err, c.wantErr)
			}
		})
	}
}

func TestSaleValidateReportsEveryProblemAtOnce(t *testing.T) {
	sale := Sale{Date: Date{}, Gross: 0}
	err := sale.Validate(MustParseDate("2026-03-31"))
	if err == nil {
		t.Fatal("an empty entry was accepted")
	}
	lines := strings.Split(err.Error(), "\n")
	if len(lines) < 4 {
		t.Fatalf("got %d problems from an empty entry, expected the date, the name, the basis, the ID and the amount: %v", len(lines), lines)
	}
}

func TestSaleNormalise(t *testing.T) {
	cases := []struct {
		name   string
		in     Sale
		assert func(t *testing.T, got Sale)
	}{
		{
			name: "whitespace and case",
			in: Sale{
				BuyerName: "  Corazon   Reyes  ",
				IDNo:      " osca-12345 ",
				Booklet:   " bk-7 ",
				Reference: "  OR-1  ",
				Note:      "  two cards  ",
			},
			assert: func(t *testing.T, got Sale) {
				if got.BuyerName != "Corazon Reyes" {
					t.Errorf("name = %q", got.BuyerName)
				}
				if got.IDNo != "OSCA-12345" {
					t.Errorf("ID = %q", got.IDNo)
				}
				if got.Booklet != "BK-7" {
					t.Errorf("booklet = %q", got.Booklet)
				}
				if got.Reference != "OR-1" {
					t.Errorf("reference = %q", got.Reference)
				}
				if got.Note != "two cards" {
					t.Errorf("note = %q", got.Note)
				}
			},
		},
		{
			name: "missing figures fall back to the statute",
			in:   Sale{},
			assert: func(t *testing.T, got Sale) {
				if got.Persons != 1 {
					t.Errorf("persons = %d, want 1", got.Persons)
				}
				if got.Allocation != AllocationPerHead {
					t.Errorf("allocation = %q", got.Allocation)
				}
				if got.VATRate != VATRatePercent || got.DiscountRate != StatutoryDiscountPercent {
					t.Errorf("rates = %d%%/%d%%", got.VATRate, got.DiscountRate)
				}
			},
		},
		{
			name: "an impossible headcount is brought back to one",
			in:   Sale{Persons: -4},
			assert: func(t *testing.T, got Sale) {
				if got.Persons != 1 {
					t.Errorf("persons = %d, want 1", got.Persons)
				}
			},
		},
		{
			name: "an unknown allocation falls back to per head",
			in:   Sale{Allocation: "halves"},
			assert: func(t *testing.T, got Sale) {
				if got.Allocation != AllocationPerHead {
					t.Errorf("allocation = %q", got.Allocation)
				}
			},
		},
		{
			name: "figures already set are left alone",
			in:   Sale{Persons: 4, Allocation: AllocationItemised, VATRate: 10, DiscountRate: 25},
			assert: func(t *testing.T, got Sale) {
				if got.Persons != 4 || got.Allocation != AllocationItemised || got.VATRate != 10 || got.DiscountRate != 25 {
					t.Errorf("Normalise changed settled figures: %+v", got)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.assert(t, c.in.Normalise())
		})
	}
}

func TestSaleKeyIdentifiesTheSameSale(t *testing.T) {
	a := goodSale()
	b := goodSale()
	if a.Key() != b.Key() {
		t.Fatal("the same sale keyed differently")
	}

	cases := []struct {
		name   string
		mangle func(s *Sale)
		same   bool
	}{
		{name: "a different amount is still the same sale", mangle: func(s *Sale) { s.Gross = Pesos(900) }, same: true},
		{name: "a different name is still the same card", mangle: func(s *Sale) { s.BuyerName = "Someone Else" }, same: true},
		{name: "a different day is a different sale", mangle: func(s *Sale) { s.Date = MustParseDate("2026-03-17") }},
		{name: "a different receipt is a different sale", mangle: func(s *Sale) { s.Reference = "OR-9999" }},
		{name: "a different card is a different sale", mangle: func(s *Sale) { s.IDNo = "OSCA-99999" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			other := goodSale()
			c.mangle(&other)
			if (other.Key() == a.Key()) != c.same {
				t.Fatalf("keys matched = %v, want %v (%q vs %q)", other.Key() == a.Key(), c.same, other.Key(), a.Key())
			}
		})
	}
}

func TestSaleVATForgone(t *testing.T) {
	cases := []struct {
		name string
		sale Sale
		want Centavos
	}{
		{name: "the tax taken out", sale: goodSale(), want: Pesos(75)},
		{
			name: "nothing taken out where there was none",
			sale: Sale{Gross: Pesos(1000), VATExempt: Pesos(1000)},
			want: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sale.VATForgone(); got != c.want {
				t.Fatalf("VATForgone() = %s, want %s", got.Format(), c.want.Format())
			}
		})
	}
}
