package till

import (
	"fmt"
	"sort"
	"strings"
)

// Tally is a set of entries added up.
type Tally struct {
	// Entries is how many entries went into the tally, and Buyers how many
	// distinct identification numbers they were spread over.
	Entries int `json:"entries"`
	Buyers  int `json:"buyers"`

	// Gross is the qualified buyers' shares as posted, VAT included.
	Gross Centavos `json:"gross"`

	// VATExempt is the VAT-exempt sales: the amount the discounts were taken
	// on. This is the figure a VAT return reports as exempt sales.
	VATExempt Centavos `json:"vatExempt"`

	// VATForgone is the value-added tax not charged on those sales. It is not
	// a deduction and not a credit: an establishment's input tax attributable
	// to an exempt sale is closed to cost or expense rather than claimed.
	VATForgone Centavos `json:"vatForgone"`

	// Discount is the total of the statutory discounts granted. This is the
	// figure the establishment claims as a deduction from gross income for the
	// taxable year the discounts were granted in.
	//
	// Authority: RA 9994 s. 4 and BIR RR 7-2010 for senior citizens; RA 10754
	// and BIR RR 5-2017 for persons with disability. The deduction is of the
	// cost of the discount, claimed in the same taxable year.
	Discount Centavos `json:"discount"`

	// Net is what those buyers paid.
	Net Centavos `json:"net"`
}

// add puts one entry into a tally. Distinct buyers are counted by the caller,
// which holds the set.
func (t *Tally) add(s Sale) {
	t.Entries++
	t.Gross += s.Gross
	t.VATExempt += s.VATExempt
	t.VATForgone += s.VATForgone()
	t.Discount += s.Discount
	t.Net += s.Net
}

// MonthTally is one calendar month of a summary, so a period can be read month
// by month as the monthly reports are made up.
type MonthTally struct {
	Label string `json:"label"`
	From  Date   `json:"from"`
	To    Date   `json:"to"`
	Tally Tally  `json:"tally"`
}

// Summary is a period of the sales record added up: the figures an
// establishment needs to report its exempt sales and to claim the discounts it
// granted as a deduction.
type Summary struct {
	Period Period `json:"period"`

	// Total is the whole period; Senior and PWD are the same period split by
	// the law claimed under, because the two records are kept separately.
	Total  Tally `json:"total"`
	Senior Tally `json:"senior"`
	PWD    Tally `json:"pwd"`

	// Months breaks the period down by calendar month.
	Months []MonthTally `json:"months"`

	// Rates lists every pair of rates the entries in this period were worked
	// out with. More than one means a rate changed inside the period, and the
	// summary says so rather than hiding it.
	Rates []string `json:"rates"`

	// Notes are the honest qualifications on the figures above.
	Notes []string `json:"notes"`
}

// Deduction is the figure claimed as a deduction from gross income: the total
// of the discounts granted in the period.
func (s Summary) Deduction() Centavos { return s.Total.Discount }

// InPeriod picks the entries of the register that fall inside a period, newest
// first, and within a day by receipt number then buyer, so the same register
// always reads the same way.
func InPeriod(sales []Sale, p Period) []Sale {
	out := make([]Sale, 0, len(sales))
	for _, s := range sales {
		if s.Date.Within(p.From, p.To) {
			out = append(out, s)
		}
	}
	SortSales(out)
	return out
}

// SortSales puts a register in its reading order: newest day first, and within
// a day by receipt number and then by buyer name.
func SortSales(sales []Sale) {
	sort.SliceStable(sales, func(i, j int) bool {
		a, b := sales[i], sales[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date)
		}
		if a.Reference != b.Reference {
			return a.Reference < b.Reference
		}
		return a.BuyerName < b.BuyerName
	})
}

// Summarise adds up the entries that fall in a period.
//
// Nothing is estimated and nothing is apportioned: an entry is in the period
// or it is not, by the date of the transaction.
func Summarise(sales []Sale, p Period) Summary {
	picked := InPeriod(sales, p)

	sum := Summary{Period: p}
	totalBuyers := make(map[string]bool)
	seniorBuyers := make(map[string]bool)
	pwdBuyers := make(map[string]bool)
	rates := make(map[string]bool)

	for _, s := range picked {
		sum.Total.add(s)
		totalBuyers[s.IDNo] = true
		rates[fmt.Sprintf("%d%% VAT, %d%% discount", s.VATRate, s.DiscountRate)] = true

		switch s.Basis {
		case BasisSenior:
			sum.Senior.add(s)
			seniorBuyers[s.IDNo] = true
		case BasisPWD:
			sum.PWD.add(s)
			pwdBuyers[s.IDNo] = true
		}
	}
	sum.Total.Buyers = len(totalBuyers)
	sum.Senior.Buyers = len(seniorBuyers)
	sum.PWD.Buyers = len(pwdBuyers)

	for _, month := range MonthsIn(p) {
		tally := Tally{}
		buyers := make(map[string]bool)
		for _, s := range picked {
			if s.Date.Within(month.From, month.To) {
				tally.add(s)
				buyers[s.IDNo] = true
			}
		}
		tally.Buyers = len(buyers)
		sum.Months = append(sum.Months, MonthTally{
			Label: month.Label,
			From:  month.From,
			To:    month.To,
			Tally: tally,
		})
	}

	sum.Rates = make([]string, 0, len(rates))
	for r := range rates {
		sum.Rates = append(sum.Rates, r)
	}
	sort.Strings(sum.Rates)

	sum.Notes = summaryNotes(sum)
	return sum
}

// summaryNotes says what the figures are and are not.
func summaryNotes(s Summary) []string {
	out := []string{
		"The discounts granted are claimed as a deduction from gross income for the taxable year they were granted in — RA 9994 s. 4 with BIR RR 7-2010, and RA 10754 with BIR RR 5-2017. They are not a tax credit.",
		"The VAT-exempt figure is the amount the discounts were taken on. Input tax attributable to an exempt sale is not creditable; confirm with your accountant how it is treated in your books.",
	}
	if len(s.Rates) > 1 {
		out = append(out, "More than one pair of rates appears in this period ("+strings.Join(s.Rates, "; ")+"), so entries here were not all worked out on the same figures.")
	}
	if s.Total.Entries == 0 {
		out = append(out, "No entries fall in this period.")
	}
	return out
}

// RegisterColumns are the headings of the printable register and of the
// exported file, in the order the regulations list them, with the tool's own
// working columns after.
func RegisterColumns() []string {
	return []string{
		"Date",
		"Receipt no.",
		"Buyer name",
		"Basis",
		"ID no.",
		"Booklet no.",
		"Gross sales (VAT-incl.)",
		"VAT not charged",
		"VAT-exempt sales",
		"Discount granted",
		"Net sales",
		"Persons on bill",
		"Share basis",
		"Rates applied",
		"Note",
	}
}

// RegisterRow is one entry as the printable register and the exported file
// write it: every amount already formatted, so the page, the file and a
// printout can never differ.
func RegisterRow(s Sale) []string {
	return []string{
		s.Date.String(),
		s.Reference,
		s.BuyerName,
		s.Basis.Label(),
		s.IDNo,
		s.Booklet,
		s.Gross.String(),
		s.VATForgone().String(),
		s.VATExempt.String(),
		s.Discount.String(),
		s.Net.String(),
		fmt.Sprintf("%d", s.Persons),
		allocationLabel(s.Allocation),
		fmt.Sprintf("%d%% VAT / %d%% discount", s.VATRate, s.DiscountRate),
		s.Note,
	}
}

// RegisterRows is every entry of a register in reading order.
func RegisterRows(sales []Sale) [][]string {
	out := make([][]string, 0, len(sales))
	for _, s := range sales {
		out = append(out, RegisterRow(s))
	}
	return out
}

// RegisterTotalsRow closes the register with the column totals, so a printed
// page carries its own footing.
func RegisterTotalsRow(t Tally) []string {
	return []string{
		"", "", fmt.Sprintf("Total — %d entries", t.Entries), "", "", "",
		t.Gross.String(),
		t.VATForgone.String(),
		t.VATExempt.String(),
		t.Discount.String(),
		t.Net.String(),
		"", "", "", "",
	}
}

// allocationLabel names an allocation in words for a register column.
func allocationLabel(a Allocation) string {
	switch a {
	case AllocationPerHead:
		return "Per head"
	case AllocationItemised:
		return "Itemised"
	default:
		return string(a)
	}
}
