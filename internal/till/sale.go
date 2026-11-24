package till

import (
	"errors"
	"fmt"
	"strings"
)

// Sale is one entry in the sales record an establishment is required to keep
// for its senior citizen and person-with-disability transactions — a separate
// record from its ordinary sales, kept so that the exempt sales and the
// discounts granted can be substantiated.
//
// The columns below are the ones the regulations call for: the date of the
// transaction, the name of the buyer, the identification number on the card
// shown, the gross sales, the discount granted and the net sales. The
// establishment's own receipt number ties the entry back to the receipt.
//
// Authority: BIR Revenue Regulations No. 7-2010 (senior citizens' sales book)
// and No. 5-2017 (persons with disability). Confirm the columns and the
// retention period required of your establishment with the Bureau of Internal
// Revenue; this tool keeps a record, it does not define the requirement.
type Sale struct {
	// ID is this tool's own handle on the entry, so it can be removed again.
	// It is not a statutory field.
	ID string `json:"id"`

	// Date is the day of the transaction.
	Date Date `json:"date"`

	// Reference is the establishment's receipt or invoice number.
	Reference string `json:"reference"`

	// BuyerName is the buyer's name as on the identification card.
	BuyerName string `json:"buyerName"`

	// Basis is the law claimed under, which decides which record the entry
	// belongs in.
	Basis Basis `json:"basis"`

	// IDNo is the OSCA or senior citizen identification number, or the
	// person-with-disability identification number.
	IDNo string `json:"idNo"`

	// Booklet is the purchase booklet or slip number, where one is used.
	Booklet string `json:"booklet"`

	// Persons is how many people shared the bill, and Allocation how the
	// buyer's share of it was arrived at. Neither is a statutory column; both
	// are kept because a share of a group bill is the figure most often
	// questioned, and this is the working.
	Persons    int        `json:"persons"`
	Allocation Allocation `json:"allocation"`

	// Gross is the buyer's share of the covered items as posted, with VAT
	// still in it.
	Gross Centavos `json:"gross"`

	// VATExempt is the VAT-exempt selling price: the gross less the tax that
	// was not charged. It is the amount the discount was taken on.
	VATExempt Centavos `json:"vatExempt"`

	// Discount is the statutory discount granted.
	Discount Centavos `json:"discount"`

	// Net is what the buyer paid.
	Net Centavos `json:"net"`

	// VATRate and DiscountRate are the figures the entry was worked out with,
	// kept with the entry so that a rate change later does not silently
	// restate a past month.
	VATRate      int `json:"vatRate"`
	DiscountRate int `json:"discountRate"`

	// Note is anything the till wanted kept with the transaction.
	Note string `json:"note"`
}

// VATForgone is the value-added tax the establishment did not charge on this
// sale because it was exempt.
func (s Sale) VATForgone() Centavos { return s.Gross - s.VATExempt }

// Normalise tidies an entry: whitespace off the text, identification numbers
// upper-cased so the same card always reads the same way, and sensible values
// where something was left out.
func (s Sale) Normalise() Sale {
	s.ID = strings.TrimSpace(s.ID)
	s.Reference = strings.TrimSpace(s.Reference)
	s.BuyerName = strings.Join(strings.Fields(s.BuyerName), " ")
	s.IDNo = strings.ToUpper(strings.Join(strings.Fields(s.IDNo), " "))
	s.Booklet = strings.ToUpper(strings.Join(strings.Fields(s.Booklet), " "))
	s.Note = strings.TrimSpace(s.Note)
	if s.Persons < 1 {
		s.Persons = 1
	}
	if !s.Allocation.Valid() {
		s.Allocation = AllocationPerHead
	}
	if s.VATRate <= 0 {
		s.VATRate = VATRatePercent
	}
	if s.DiscountRate <= 0 {
		s.DiscountRate = StatutoryDiscountPercent
	}
	return s
}

// Key is how two entries are told apart when the same bill is recorded twice:
// the same card, the same receipt number and the same day is the same sale.
func (s Sale) Key() string {
	return strings.Join([]string{s.Date.String(), strings.ToUpper(s.Reference), s.IDNo}, "|")
}

// Validate reports everything wrong with an entry, one problem per line.
//
// The arithmetic is checked as well as the fields: an entry whose net does not
// equal its VAT-exempt amount less its discount, or whose discount is not the
// statutory percentage of that amount, is refused. A sales record that does
// not add up substantiates nothing.
func (s Sale) Validate(today Date) error {
	var problems []string

	if s.Date.IsZero() {
		problems = append(problems, "the entry needs the date of the transaction")
	} else if !today.IsZero() && s.Date.After(today) {
		problems = append(problems, fmt.Sprintf("%s is in the future", s.Date))
	}
	if s.BuyerName == "" {
		problems = append(problems, "the entry needs the buyer's name")
	}
	if !s.Basis.Valid() {
		problems = append(problems, "the entry needs to say whether the claim was as a senior citizen or as a person with disability")
	}
	if s.IDNo == "" {
		problems = append(problems, fmt.Sprintf("the entry needs the %s", strings.ToLower(s.Basis.IDLabel())))
	}
	if s.Gross <= 0 {
		problems = append(problems, "the gross amount has to be more than nothing")
	}
	if s.Gross > MaxAmount {
		problems = append(problems, "the gross amount is larger than this tool will work with")
	}
	if s.VATExempt < 0 || s.Discount < 0 || s.Net < 0 {
		problems = append(problems, "no amount on the entry can be negative")
	}
	if s.VATExempt > s.Gross {
		problems = append(problems, fmt.Sprintf(
			"the VAT-exempt amount %s is more than the gross %s — the tax comes out of the gross, it is not added to it",
			s.VATExempt.Format(), s.Gross.Format()))
	}
	if s.Discount > s.VATExempt {
		problems = append(problems, fmt.Sprintf(
			"the discount %s is more than the VAT-exempt amount %s it is taken on",
			s.Discount.Format(), s.VATExempt.Format()))
	}
	if s.VATExempt-s.Discount != s.Net {
		problems = append(problems, fmt.Sprintf(
			"the amounts do not add up: %s VAT-exempt less %s discount is %s, not the %s recorded as paid",
			s.VATExempt.Format(), s.Discount.Format(), (s.VATExempt-s.Discount).Format(), s.Net.Format()))
	}
	if expected := s.VATExempt.Percent(s.DiscountRate); s.Discount != expected {
		problems = append(problems, fmt.Sprintf(
			"the discount on %s at %d%% is %s, not %s",
			s.VATExempt.Format(), s.DiscountRate, expected.Format(), s.Discount.Format()))
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}
