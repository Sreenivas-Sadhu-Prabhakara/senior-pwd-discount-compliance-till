package till

import (
	"errors"
	"fmt"
	"strings"
)

// Line is one thing on the bill, priced the way the customer sees it on the
// menu or price list.
//
// Whether the statute covers a line is NOT something this tool can work out.
// Coverage is set by Republic Act 9994 and Republic Act 10754 and by their
// implementing rules — which name the goods and services the discount and the
// exemption reach — so the person at the till marks each line and the tool
// does the arithmetic that follows.
type Line struct {
	// Description is what the line is, as it appears on the bill.
	Description string `json:"description"`

	// Amount is the price as posted, which in a VAT-registered establishment
	// already has the 12 per cent inside it.
	Amount Centavos `json:"amount"`

	// Covered says the statute covers this good or service for a senior
	// citizen or a person with disability: 20 per cent off, and the sale of it
	// to that buyer is exempt from value-added tax.
	Covered bool `json:"covered"`

	// VATExempt says this line carries no value-added tax for anybody —
	// because the item is exempt or zero-rated in its own right, not because
	// of who is buying it. There is then no tax to take out before the
	// discount, and the 20 per cent applies to the selling price itself.
	VATExempt bool `json:"vatExempt"`
}

// vatable reports whether the price of this line has value-added tax inside
// it, which needs both a VAT-registered establishment and a line that is not
// exempt in its own right.
func (l Line) vatable(r Rates) bool { return r.VATRegistered && !l.VATExempt }

// Buyer is one qualified customer on a bill, and the proof of entitlement the
// establishment is required to write into its sales record.
type Buyer struct {
	// Name is the buyer's name, as on the identification card.
	Name string `json:"name"`

	// Basis is the law claimed under: one buyer claims under one law.
	Basis Basis `json:"basis"`

	// IDNo is the OSCA or senior citizen identification number, or the
	// person-with-disability identification number.
	IDNo string `json:"idNo"`

	// Booklet is the purchase booklet or slip number where the
	// establishment's trade uses one.
	Booklet string `json:"booklet"`

	// Share is this buyer's own covered consumption, VAT-inclusive as posted.
	// It is used only when the bill is itemised; on a per-head bill it is
	// worked out instead.
	Share Centavos `json:"share"`
}

// Allocation is how the covered part of a shared bill is attributed to the
// qualified buyers on it.
type Allocation string

const (
	// AllocationPerHead divides the covered part of the bill by the number of
	// people sharing it and gives each qualified buyer one share. This is the
	// rule for a group bill that cannot be itemised: see
	// SharesPerPersonInGroup.
	AllocationPerHead Allocation = "per_head"

	// AllocationItemised uses what each qualified buyer actually consumed, as
	// stated on the bill. Where the bill itemises who had what, that is the
	// better evidence and this is the allocation to use.
	AllocationItemised Allocation = "itemised"
)

// Valid reports whether an allocation is one this tool knows.
func (a Allocation) Valid() bool {
	return a == AllocationPerHead || a == AllocationItemised
}

// Bill is one transaction at the till.
type Bill struct {
	// Date is the day of the transaction, for the sales record.
	Date Date `json:"date"`

	// Reference is the establishment's own receipt or invoice number.
	Reference string `json:"reference"`

	// Lines are the things bought, priced as posted.
	Lines []Line `json:"lines"`

	// Persons is how many people the bill is shared by, qualified buyers
	// included. One person eating alone is 1.
	Persons int `json:"persons"`

	// Buyers are the qualified customers on this bill.
	Buyers []Buyer `json:"buyers"`

	// Allocation is how the covered amount reaches those buyers.
	Allocation Allocation `json:"allocation"`

	// Promotion is a promotional or other offered discount available on the
	// same items, as a VAT-inclusive peso amount off the whole bill. The
	// statutory discount and a promotional discount are alternatives, not
	// things that add up, so this is used only to say which is the more
	// favourable: see PromotionalDiscountIsAnAlternative.
	Promotion Centavos `json:"promotion"`

	// Note is anything the till wants kept with the transaction.
	Note string `json:"note"`
}

// Step is one line of the order of operations, in the order it is done. The
// slip and the screen both show these, because the sequence is the thing that
// is got wrong.
type Step struct {
	Label     string   `json:"label"`
	Amount    Centavos `json:"amount"`
	Deduction bool     `json:"deduction"`
	Emphasis  bool     `json:"emphasis"`
	Authority string   `json:"authority"`
}

// BuyerShare is what one qualified buyer on the bill pays, and the figures the
// sales record needs for them.
type BuyerShare struct {
	Buyer Buyer `json:"buyer"`

	// Gross is their share of the covered items as posted, VAT included.
	Gross Centavos `json:"gross"`

	// VATRemoved is the value-added tax not charged, because the sale to them
	// is exempt.
	VATRemoved Centavos `json:"vatRemoved"`

	// DiscountBase is the VAT-exempt selling price: the amount the 20 per cent
	// is taken on. This is the figure the whole tool exists to get right.
	DiscountBase Centavos `json:"discountBase"`

	// Discount is the statutory discount granted to them.
	Discount Centavos `json:"discount"`

	// Due is what they pay: the base less the discount.
	Due Centavos `json:"due"`
}

// Saved is the whole benefit to this buyer: the tax not charged plus the
// discount granted.
func (b BuyerShare) Saved() Centavos { return b.VATRemoved + b.Discount }

// Computation is the worked-out bill.
type Computation struct {
	Rates      Rates      `json:"rates"`
	Allocation Allocation `json:"allocation"`
	Persons    int        `json:"persons"`
	Qualified  int        `json:"qualified"`

	// CoveredGross and NotCoveredGross split the bill as posted by whether the
	// statute covers the line.
	CoveredGross    Centavos `json:"coveredGross"`
	NotCoveredGross Centavos `json:"notCoveredGross"`
	BillGross       Centavos `json:"billGross"`

	// Shares is one entry per qualified buyer, in the order given.
	Shares []BuyerShare `json:"shares"`

	// QualifiedGross, VATRemoved, DiscountBase, Discount and QualifiedDue are
	// the buyers' figures added up.
	QualifiedGross Centavos `json:"qualifiedGross"`
	VATRemoved     Centavos `json:"vatRemoved"`
	DiscountBase   Centavos `json:"discountBase"`
	Discount       Centavos `json:"discount"`
	QualifiedDue   Centavos `json:"qualifiedDue"`

	// OthersDue is what everyone else on the bill pays: their share of the
	// covered items, still VAT-inclusive, plus everything the statute does not
	// cover.
	OthersDue Centavos `json:"othersDue"`

	// OutputVAT is the value-added tax still inside OthersDue, which the
	// establishment collects and remits as before.
	OutputVAT Centavos `json:"outputVat"`

	// TotalDue is what the table pays altogether, and Saved is what the
	// qualified buyers were spared.
	TotalDue Centavos `json:"totalDue"`
	Saved    Centavos `json:"saved"`

	// Steps is the order of operations, for the slip.
	Steps []Step `json:"steps"`

	// Promotion, where one was given, compares the statutory benefit with the
	// promotional discount the buyers could take instead.
	Promotion *PromotionChoice `json:"promotion,omitempty"`

	// Notes are things the till should know about this computation.
	Notes []string `json:"notes"`
}

// PromotionChoice compares the statutory discount with a promotional discount
// on the same items. They are alternatives: the qualified buyer takes
// whichever is higher and more favourable, never both.
type PromotionChoice struct {
	// PromotionOffered is the promotional discount as entered, VAT-inclusive.
	PromotionOffered Centavos `json:"promotionOffered"`

	// StatutoryBenefit is what the qualified buyers save under the statute:
	// the tax not charged plus the 20 per cent.
	StatutoryBenefit Centavos `json:"statutoryBenefit"`

	// PromotionBenefit is the part of the promotional discount that would fall
	// to the qualified buyers, on the same share of the bill.
	PromotionBenefit Centavos `json:"promotionBenefit"`

	// StatutoryIsHigher says the statutory discount is the more favourable of
	// the two for the qualified buyers.
	StatutoryIsHigher bool `json:"statutoryIsHigher"`

	// Advice is the sentence the till should read.
	Advice string `json:"advice"`

	// Authority names the rule.
	Authority string `json:"authority"`
}

// Validate reports everything wrong with a bill, as one error per problem on
// its own line, so a form can show them all at once instead of one at a time.
func (b Bill) Validate() error {
	var problems []string

	if b.Persons < 1 {
		problems = append(problems, "say how many people are sharing this bill — one person eating alone is 1")
	}
	if b.Persons > MaxPersonsSharingABill {
		problems = append(problems, fmt.Sprintf("%d people on one bill is more than this tool will divide between", b.Persons))
	}
	if len(b.Lines) == 0 {
		problems = append(problems, "the bill has no lines on it")
	}
	if !b.Allocation.Valid() {
		problems = append(problems, fmt.Sprintf("%q is not a way of allocating a shared bill", b.Allocation))
	}

	var total Centavos
	covered := 0
	for i, line := range b.Lines {
		if line.Amount < 0 {
			problems = append(problems, fmt.Sprintf("line %d (%s) is a negative amount", i+1, lineName(line, i)))
		}
		if line.Amount > MaxAmount {
			problems = append(problems, fmt.Sprintf("line %d (%s) is larger than this tool will work with", i+1, lineName(line, i)))
		}
		total += line.Amount
		if line.Covered {
			covered++
		}
	}
	if total > MaxAmount {
		problems = append(problems, "the bill is larger than this tool will work with")
	}

	if len(b.Buyers) > b.Persons && b.Persons >= 1 {
		problems = append(problems, fmt.Sprintf("there are %d qualified buyers on a bill shared by %d people", len(b.Buyers), b.Persons))
	}
	if len(b.Buyers) > 0 && covered == 0 {
		problems = append(problems, "no line on this bill is marked as covered by the statute, so there is nothing to discount")
	}

	seen := make(map[string]bool, len(b.Buyers))
	for i, buyer := range b.Buyers {
		who := strings.TrimSpace(buyer.Name)
		if who == "" {
			who = fmt.Sprintf("buyer %d", i+1)
		}
		if strings.TrimSpace(buyer.Name) == "" {
			problems = append(problems, fmt.Sprintf("buyer %d has no name — the sales record has to carry it", i+1))
		}
		if !buyer.Basis.Valid() {
			problems = append(problems, fmt.Sprintf("%s: say whether the claim is as a senior citizen or as a person with disability", who))
		}
		if strings.TrimSpace(buyer.IDNo) == "" {
			problems = append(problems, fmt.Sprintf("%s: the identification number has to be taken and recorded", who))
		}
		key := strings.ToUpper(strings.TrimSpace(buyer.IDNo))
		if key != "" && seen[key] {
			problems = append(problems, fmt.Sprintf("identification number %s appears twice on this bill — one buyer claims one discount", buyer.IDNo))
		}
		seen[key] = true

		if b.Allocation == AllocationItemised {
			if buyer.Share <= 0 {
				problems = append(problems, fmt.Sprintf("%s: on an itemised bill, say what they consumed", who))
			}
			if buyer.Share > MaxAmount {
				problems = append(problems, fmt.Sprintf("%s: that share is larger than this tool will work with", who))
			}
		}
	}

	if b.Promotion < 0 {
		problems = append(problems, "a promotional discount cannot be a negative amount")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// lineName is a line's description, or its position when it has none.
func lineName(l Line, i int) string {
	if d := strings.TrimSpace(l.Description); d != "" {
		return d
	}
	return fmt.Sprintf("line %d", i+1)
}

// Compute works out the bill.
//
// The order of operations is the rule, and it is this:
//
//  1. Add up the lines the statute covers, as posted.
//  2. Work out the qualified buyers' share of that — their own consumption on
//     an itemised bill, or one head's worth each on a group bill.
//  3. Take the 12 per cent value-added tax out of that share, because the sale
//     to a qualified buyer is exempt from it. What is left is the VAT-exempt
//     selling price.
//  4. Take the 20 per cent statutory discount on that VAT-exempt selling
//     price — never on the price with the tax still inside it.
//  5. The buyer pays the VAT-exempt selling price less the discount. Everyone
//     else pays their share with the tax still in it, and anything the statute
//     does not cover is untouched.
//
// See DiscountAppliesToVATExclusiveAmount for the authority.
func Compute(b Bill, rates Rates) (Computation, error) {
	if err := b.Validate(); err != nil {
		return Computation{}, err
	}
	rates = rates.Normalise()

	c := Computation{
		Rates:      rates,
		Allocation: b.Allocation,
		Persons:    b.Persons,
		Qualified:  len(b.Buyers),
	}

	// Step 1 — the bill as posted, split by whether the statute covers it.
	// The VATable and exempt halves of the covered amount are kept apart,
	// because only the VATable half has tax to take out.
	var coveredVATable, coveredVATFree Centavos
	for _, line := range b.Lines {
		c.BillGross += line.Amount
		if !line.Covered {
			c.NotCoveredGross += line.Amount
			continue
		}
		c.CoveredGross += line.Amount
		if line.vatable(rates) {
			coveredVATable += line.Amount
		} else {
			coveredVATFree += line.Amount
		}
	}

	// Step 2 — the qualified buyers' share of the covered amount.
	perBuyerVATable, perBuyerVATFree, err := allocate(b, coveredVATable, coveredVATFree)
	if err != nil {
		return Computation{}, err
	}

	// Steps 3 to 5, for each qualified buyer in turn.
	for i, buyer := range b.Buyers {
		vatableShare := perBuyerVATable[i]
		vatFreeShare := perBuyerVATFree[i]

		// Step 3 — out comes the tax. Only the VATable part has any.
		base := vatableShare.NetOfVAT(rates.VATPercent)
		removed := vatableShare - base
		base += vatFreeShare

		// Step 4 — the statutory discount, on that VAT-exempt amount.
		discount := base.Percent(rates.DiscountPercent)

		share := BuyerShare{
			Buyer:        buyer,
			Gross:        vatableShare + vatFreeShare,
			VATRemoved:   removed,
			DiscountBase: base,
			Discount:     discount,
			Due:          base - discount, // step 5
		}
		c.Shares = append(c.Shares, share)

		c.QualifiedGross += share.Gross
		c.VATRemoved += share.VATRemoved
		c.DiscountBase += share.DiscountBase
		c.Discount += share.Discount
		c.QualifiedDue += share.Due
	}

	// Everyone else pays their share of the covered items with the tax still
	// inside it, plus everything the statute does not cover.
	othersCovered := c.CoveredGross - c.QualifiedGross
	c.OthersDue = othersCovered + c.NotCoveredGross
	c.OutputVAT = outputVAT(b, rates, coveredVATable-qualifiedVATable(perBuyerVATable))

	c.TotalDue = c.QualifiedDue + c.OthersDue
	c.Saved = c.VATRemoved + c.Discount
	c.Steps = steps(b, rates, c, othersCovered)
	c.Promotion = promotionChoice(b, c)
	c.Notes = notes(b, rates, c)

	return c, nil
}

// qualifiedVATable adds up the VATable part allocated to the qualified buyers.
func qualifiedVATable(perBuyer []Centavos) Centavos {
	var sum Centavos
	for _, v := range perBuyer {
		sum += v
	}
	return sum
}

// allocate works out each qualified buyer's share of the covered amount,
// keeping the VATable and the VAT-free parts apart.
//
// On a group bill the covered amount is divided into one share per person and
// each qualified buyer takes one — SharesPerPersonInGroup. On an itemised bill
// the buyer's own stated consumption is used, apportioned between the VATable
// and VAT-free parts of the covered amount in the proportion the bill itself
// has them, because the itemised figure is a single amount.
func allocate(b Bill, coveredVATable, coveredVATFree Centavos) (vatable, vatFree []Centavos, err error) {
	n := len(b.Buyers)
	vatable = make([]Centavos, n)
	vatFree = make([]Centavos, n)
	if n == 0 {
		return vatable, vatFree, nil
	}

	switch b.Allocation {
	case AllocationPerHead:
		vatableShares := Split(coveredVATable, b.Persons)
		vatFreeShares := Split(coveredVATFree, b.Persons)
		for i := range b.Buyers {
			vatable[i] = vatableShares[i]
			vatFree[i] = vatFreeShares[i]
		}
		return vatable, vatFree, nil

	case AllocationItemised:
		covered := coveredVATable + coveredVATFree
		var claimed Centavos
		for _, buyer := range b.Buyers {
			claimed += buyer.Share
		}
		if claimed > covered {
			return nil, nil, fmt.Errorf(
				"the qualified buyers' itemised shares come to %s, which is more than the %s of covered items on the bill",
				claimed.Format(), covered.Format())
		}
		for i, buyer := range b.Buyers {
			if covered == 0 {
				continue
			}
			// Split the buyer's own amount in the proportion the covered part
			// of the bill is VATable, then give the remainder to the VAT-free
			// side so the two always add back to the buyer's share.
			vatable[i] = mulDivRound(buyer.Share, int64(coveredVATable), int64(covered))
			vatFree[i] = buyer.Share - vatable[i]
		}
		return vatable, vatFree, nil
	}
	return nil, nil, fmt.Errorf("%q is not a way of allocating a shared bill", b.Allocation)
}

// outputVAT is the tax still inside what the establishment collects on this
// bill: the other people's share of the VATable covered items, plus the
// VATable lines the statute does not cover.
func outputVAT(b Bill, rates Rates, othersVATableCovered Centavos) Centavos {
	if !rates.VATRegistered {
		return 0
	}
	vatable := othersVATableCovered
	for _, line := range b.Lines {
		if !line.Covered && line.vatable(rates) {
			vatable += line.Amount
		}
	}
	return vatable.VATWithin(rates.VATPercent)
}

// steps writes out the order of operations for the slip and the screen.
func steps(b Bill, rates Rates, c Computation, othersCovered Centavos) []Step {
	out := []Step{{
		Label:  "Covered goods and services, as priced",
		Amount: c.CoveredGross,
	}}

	if c.Qualified == 0 {
		out = append(out, Step{
			Label:    "No qualified buyer on this bill — nothing is exempt and nothing is discounted",
			Amount:   0,
			Emphasis: false,
		})
	} else if b.Allocation == AllocationPerHead && b.Persons > 1 {
		out = append(out, Step{
			Label:     fmt.Sprintf("Qualified share — %s of %d sharing the bill", countOf(c.Qualified), b.Persons),
			Amount:    c.QualifiedGross,
			Authority: "IRR of RA 9994, Rule IV Art. 7 — group bills are divided per head",
		})
	} else if b.Allocation == AllocationItemised {
		out = append(out, Step{
			Label:     fmt.Sprintf("Qualified share — itemised consumption of %s", countOf(c.Qualified)),
			Amount:    c.QualifiedGross,
			Authority: "the discount reaches only what the qualified buyer consumed",
		})
	}

	if c.Qualified > 0 {
		if c.VATRemoved > 0 {
			out = append(out, Step{
				Label:     fmt.Sprintf("Less value-added tax at %d%% — the sale is VAT-exempt", rates.VATPercent),
				Amount:    c.VATRemoved,
				Deduction: true,
				Authority: "RA 9994 s. 4(a); RA 10754; NIRC s. 106/108",
			})
		} else if !rates.VATRegistered {
			out = append(out, Step{
				Label:     "No value-added tax to take out — this establishment is not VAT-registered",
				Amount:    0,
				Authority: "the discount is then taken on the selling price itself",
			})
		}
		out = append(out, Step{
			Label:     "VAT-exempt selling price — the amount the discount is taken on",
			Amount:    c.DiscountBase,
			Emphasis:  true,
			Authority: "BIR RR 7-2010 s. 6; RR 5-2017",
		})
		out = append(out, Step{
			Label:     fmt.Sprintf("Less statutory discount at %d%%", rates.DiscountPercent),
			Amount:    c.Discount,
			Deduction: true,
			Authority: "RA 9994 s. 4(a) / RA 10754",
		})
		out = append(out, Step{
			Label:    fmt.Sprintf("Amount due from %s", countOf(c.Qualified)),
			Amount:   c.QualifiedDue,
			Emphasis: true,
		})
	}

	if othersCovered > 0 {
		out = append(out, Step{
			Label:     "Other customers' share — VAT-inclusive, no discount",
			Amount:    othersCovered,
			Authority: "the exemption is personal to the qualified buyer",
		})
	}
	if c.NotCoveredGross > 0 {
		out = append(out, Step{
			Label:     "Goods and services the statute does not cover",
			Amount:    c.NotCoveredGross,
			Authority: "coverage is set by the statutes and their implementing rules",
		})
	}
	out = append(out, Step{
		Label:    "Total amount due",
		Amount:   c.TotalDue,
		Emphasis: true,
	})
	return out
}

// countOf names a number of qualified buyers.
func countOf(n int) string {
	if n == 1 {
		return "1 qualified buyer"
	}
	return fmt.Sprintf("%d qualified buyers", n)
}

// promotionChoice compares the statutory benefit with a promotional discount
// on the same items, where one was entered. The two are alternatives.
func promotionChoice(b Bill, c Computation) *PromotionChoice {
	if b.Promotion <= 0 || c.Qualified == 0 {
		return nil
	}

	// Only the part of the promotion falling on the qualified buyers' share is
	// comparable with what the statute gives them.
	var promoBenefit Centavos
	switch {
	case c.BillGross == 0:
		promoBenefit = 0
	default:
		promoBenefit = mulDivRound(b.Promotion, int64(c.QualifiedGross), int64(c.BillGross))
	}

	choice := &PromotionChoice{
		PromotionOffered:  b.Promotion,
		StatutoryBenefit:  c.Saved,
		PromotionBenefit:  promoBenefit,
		StatutoryIsHigher: c.Saved >= promoBenefit,
		Authority:         "IRR of RA 9994, Rule IV Art. 7 — on a promotional discount the buyer takes whichever is higher",
	}
	if choice.StatutoryIsHigher {
		choice.Advice = fmt.Sprintf(
			"The statutory discount and exemption are worth %s to the qualified buyers against %s from the promotion, so the statutory figures above are the ones to charge. The two do not add up.",
			c.Saved.Format(), promoBenefit.Format())
	} else {
		choice.Advice = fmt.Sprintf(
			"The promotion is worth %s to the qualified buyers against %s under the statute, so they are entitled to take the promotion instead. That is then an ordinary VATable sale and does not belong in the senior citizen or PWD sales record. The two do not add up.",
			promoBenefit.Format(), c.Saved.Format())
	}
	return choice
}

// notes are the things about this computation the person at the till should
// know, and the honest limits of what the tool decided.
func notes(b Bill, rates Rates, c Computation) []string {
	out := []string{
		"Which goods and services are covered is set by RA 9994, RA 10754 and their implementing rules, not by this tool — the lines marked covered on this bill were marked by hand.",
	}
	if !rates.IsStatutory() {
		out = append(out, fmt.Sprintf(
			"Worked out at %d%% VAT and a %d%% discount, which are not the figures this tool ships with (%d%% and %d%%) — check the override in settings.",
			rates.VATPercent, rates.DiscountPercent, VATRatePercent, StatutoryDiscountPercent))
	}
	if !rates.VATRegistered {
		out = append(out, "This establishment is recorded as not VAT-registered, so there is no tax to take out and the discount is taken on the selling price.")
	}
	if b.Allocation == AllocationPerHead && b.Persons > 1 && c.Qualified > 0 {
		out = append(out, fmt.Sprintf(
			"The bill was divided between %d people. Where the bill itemises what the qualified buyer actually consumed, itemise it instead — that is the better evidence.",
			b.Persons))
	}
	if b.Allocation == AllocationItemised && c.QualifiedGross < c.CoveredGross {
		out = append(out, "Only the itemised consumption of the qualified buyers was exempted and discounted; the rest of the covered items stay VAT-inclusive.")
	}
	if c.Qualified > 1 {
		out = append(out, fmt.Sprintf("%d separate entries go into the sales record, one for each qualified buyer.", c.Qualified))
	}
	if c.NotCoveredGross > 0 && c.Qualified > 0 {
		out = append(out, fmt.Sprintf(
			"%s of this bill was marked as not covered, so it keeps its VAT and gets no discount.",
			c.NotCoveredGross.Format()))
	}
	return out
}

// SalesFrom turns a computation into the entries the establishment's sales
// record needs: one per qualified buyer, all carrying the same date and
// receipt number.
func SalesFrom(b Bill, c Computation) []Sale {
	out := make([]Sale, 0, len(c.Shares))
	for _, share := range c.Shares {
		out = append(out, Sale{
			Date:         b.Date,
			Reference:    strings.TrimSpace(b.Reference),
			BuyerName:    strings.TrimSpace(share.Buyer.Name),
			Basis:        share.Buyer.Basis,
			IDNo:         strings.TrimSpace(share.Buyer.IDNo),
			Booklet:      strings.TrimSpace(share.Buyer.Booklet),
			Persons:      b.Persons,
			Allocation:   b.Allocation,
			Gross:        share.Gross,
			VATExempt:    share.DiscountBase,
			Discount:     share.Discount,
			Net:          share.Due,
			VATRate:      c.Rates.VATPercent,
			DiscountRate: c.Rates.DiscountPercent,
			Note:         strings.TrimSpace(b.Note),
		})
	}
	return out
}
