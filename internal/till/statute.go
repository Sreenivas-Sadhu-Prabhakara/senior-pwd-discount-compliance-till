// Package till works out the statutory discount and VAT exemption a
// Philippine establishment must grant a senior citizen or a person with
// disability, keeps the sales record the establishment has to maintain for
// those transactions, and totals the discounts granted over a period.
//
// Everything here is a pure function over values. Nothing in this package
// reads a file, serves a request or contacts anything.
package till

// ---------------------------------------------------------------------------
// Statutory figures and rules — every one this tool encodes, in this one file.
//
// These are the figures in force when this tool was written. Rates and
// thresholds change; the Rates value below carries them through the rest of
// the package so an establishment can override them without touching code,
// and the tool records which figures a sale was worked out with.
//
// Confirm the current figures and, above all, which goods and services are
// covered with the Bureau of Internal Revenue, the Office for Senior Citizens
// Affairs of your city or municipality, and the National Council on Disability
// Affairs. The statutes and issuing rules named here are the authority; this
// tool is not.
// ---------------------------------------------------------------------------

const (
	// VATRatePercent is the rate of value-added tax on a sale of goods or
	// services in the Philippines: 12 per cent.
	//
	// Authority: National Internal Revenue Code of 1997, sections 106 and 108,
	// as amended (the rate moved from 10 to 12 per cent in 2006 under the
	// authority given by Republic Act 9337).
	//
	// A price on a Philippine menu or price list is ordinarily quoted with this
	// tax already inside it, which is why the tool has to take it back out
	// before it can apply the discount.
	VATRatePercent = 12

	// StatutoryDiscountPercent is the discount a qualified buyer is entitled
	// to on covered goods and services: 20 per cent.
	//
	// Authority:
	//   Senior citizens — Republic Act 9994 (Expanded Senior Citizens Act of
	//     2010), section 4(a), implemented by its Implementing Rules and
	//     Regulations and by BIR Revenue Regulations No. 7-2010.
	//   Persons with disability — Republic Act 10754 (2016), amending section
	//     32 of Republic Act 7277 (Magna Carta for Persons with Disability),
	//     implemented by BIR Revenue Regulations No. 5-2017.
	//
	// Both laws grant the same 20 per cent, and both grant it together with
	// exemption from value-added tax on the same sale.
	StatutoryDiscountPercent = 20

	// DiscountAppliesToVATExclusiveAmount records the order of operations,
	// which is the whole point of this tool: the sale to a qualified buyer is
	// first treated as exempt from value-added tax, and the 20 per cent is
	// then taken on that VAT-exclusive (net of VAT) amount — never on the
	// VAT-inclusive price the rest of the counter pays.
	//
	//	VAT-exempt selling price = VAT-inclusive price / (1 + VAT rate)
	//	statutory discount       = VAT-exempt selling price x 20%
	//	amount due               = VAT-exempt selling price - statutory discount
	//
	// Applying the 20 per cent to the VAT-inclusive price instead overcharges
	// nobody but it understates the exemption and the deduction; applying the
	// VAT on top of the discounted amount overcharges the buyer. Both are
	// common and both are wrong.
	//
	// Authority: BIR Revenue Regulations No. 7-2010, section 6 (senior
	// citizens) and No. 5-2017 (persons with disability), and the worked
	// illustrations in those regulations.
	DiscountAppliesToVATExclusiveAmount = true

	// SharesPerPersonInGroup is how many equal shares of a shared bill each
	// person at the table counts for: one.
	//
	// Where a bill is shared, the discount and the exemption reach only what
	// the qualified buyer actually consumed. Where that cannot be itemised,
	// the bill is divided by the number of people sharing it and the qualified
	// buyer's quotient is the share that is exempt and discounted. The other
	// diners' shares stay VAT-inclusive and carry no discount.
	//
	// Authority: Implementing Rules and Regulations of Republic Act 9994, Rule
	// IV, Article 7 (group meals and group purchases); the same split is
	// applied to a person with disability under Republic Act 10754.
	SharesPerPersonInGroup = 1

	// DiscountsPerQualifiedBuyer is how many 20 per cent discounts one buyer
	// may claim on one transaction: one.
	//
	// A buyer who is both a senior citizen and a person with disability claims
	// the discount once, under one law, not twice.
	//
	// Authority: Implementing Rules and Regulations of Republic Act 10754
	// (no double discount on the same transaction).
	DiscountsPerQualifiedBuyer = 1

	// PromotionalDiscountIsAnAlternative records that a promotional or other
	// offered discount and the statutory 20 per cent are alternatives, not
	// things that add up. Where an item is already on promotional discount the
	// qualified buyer takes whichever is higher and more favourable to them.
	//
	// Authority: Implementing Rules and Regulations of Republic Act 9994, Rule
	// IV, Article 7 (purchases on promotional discount).
	PromotionalDiscountIsAnAlternative = true

	// DiscountIsADeductionFromGrossIncome records what the establishment does
	// with the discounts it has granted: it claims their cost as a deduction
	// from gross income for the same taxable year, and it may not claim it as
	// a tax credit. The period summary this package produces totals exactly
	// that figure.
	//
	// Authority: Republic Act 9994, section 4 (as amended by that Act, the
	// treatment changed from a tax credit to a deduction), and BIR Revenue
	// Regulations No. 7-2010.
	DiscountIsADeductionFromGrossIncome = true
)

// MaxAmount is the largest amount this tool will work with: one hundred
// million pesos on a single line. It is not a statutory figure. It is here so
// that a mistyped amount is refused at the door rather than overflowing the
// arithmetic further in.
const MaxAmount = Centavos(100_000_000_00)

// MaxPersonsSharingABill is the largest group this tool will divide a bill
// between. It is not a statutory figure either — it only keeps a mistyped
// headcount from producing nonsense.
const MaxPersonsSharingABill = 500

// Rates are the figures a computation is made with. They start at the
// statutory values above and can be overridden by the establishment if a rate
// changes, so that this tool does not have to be rebuilt to stay correct.
type Rates struct {
	// VATPercent is the value-added tax rate applied to VATable sales.
	VATPercent int `json:"vatPercent"`

	// DiscountPercent is the statutory discount granted to a qualified buyer.
	DiscountPercent int `json:"discountPercent"`

	// VATRegistered says whether the establishment is registered for
	// value-added tax. An establishment that is not registered quotes prices
	// with no VAT inside them, so there is nothing to take out: the 20 per
	// cent is then taken on the selling price itself.
	VATRegistered bool `json:"vatRegistered"`
}

// DefaultRates are the statutory figures above, for a VAT-registered
// establishment.
func DefaultRates() Rates {
	return Rates{
		VATPercent:      VATRatePercent,
		DiscountPercent: StatutoryDiscountPercent,
		VATRegistered:   true,
	}
}

// Normalise replaces anything missing or impossible with the statutory value,
// so a half-filled settings file can never silently produce a wrong figure.
func (r Rates) Normalise() Rates {
	if r.VATPercent < 0 || r.VATPercent > 100 {
		r.VATPercent = VATRatePercent
	}
	if r.DiscountPercent <= 0 || r.DiscountPercent > 100 {
		r.DiscountPercent = StatutoryDiscountPercent
	}
	return r
}

// IsStatutory reports whether these are still the figures this build ships
// with. The interface says so, because a sale worked out on overridden figures
// is the establishment's own figure, not the statute's.
func (r Rates) IsStatutory() bool {
	return r.VATPercent == VATRatePercent && r.DiscountPercent == StatutoryDiscountPercent
}

// Basis is the law a buyer claims the discount under. One buyer, one basis:
// see DiscountsPerQualifiedBuyer.
type Basis string

const (
	// BasisSenior is a senior citizen claiming under Republic Act 9994, proved
	// by an OSCA or senior citizen identification card.
	BasisSenior Basis = "senior"

	// BasisPWD is a person with disability claiming under Republic Act 10754,
	// proved by a person-with-disability identification card issued through
	// the National Council on Disability Affairs.
	BasisPWD Basis = "pwd"
)

// Valid reports whether a basis is one of the two laws this tool knows.
func (b Basis) Valid() bool { return b == BasisSenior || b == BasisPWD }

// Label is the basis in words, for a slip or a register column.
func (b Basis) Label() string {
	switch b {
	case BasisSenior:
		return "Senior citizen"
	case BasisPWD:
		return "Person with disability"
	default:
		return string(b)
	}
}

// Statute names the law the basis comes from.
func (b Basis) Statute() string {
	switch b {
	case BasisSenior:
		return "RA 9994"
	case BasisPWD:
		return "RA 10754"
	default:
		return ""
	}
}

// IDLabel names the proof of entitlement the establishment must take the
// number of and write into its sales record.
func (b Basis) IDLabel() string {
	switch b {
	case BasisSenior:
		return "OSCA / senior citizen ID no."
	case BasisPWD:
		return "PWD ID no."
	default:
		return "ID no."
	}
}
