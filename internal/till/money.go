package till

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Centavos is an amount of money held as a whole number of centavos.
//
// Money on a till is never held as a fraction: a bill of 1,120.00 divided by
// 1.12 is 1,000.00 exactly, and the only way to be sure of that — and of the
// hundreds of amounts that do not come out even — is to count in centavos and
// round once, deliberately, where the rule says to.
type Centavos int64

// centavosPerPeso is the unit this type counts in.
const centavosPerPeso = 100

// Pesos builds an amount from whole pesos. It is for tests and worked
// examples, where the numbers are written as pesos.
func Pesos(p int64) Centavos { return Centavos(p * centavosPerPeso) }

// String writes the amount as plain digits with two decimal places and no
// grouping and no currency sign: 1000.00, -12.50, 0.00. This is the form the
// data file and the JSON interface use, so a stored amount is exact and
// readable at once.
func (c Centavos) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/centavosPerPeso, v%centavosPerPeso)
}

// Format writes the amount the way a Philippine receipt does: grouped in
// thousands, two decimal places, no currency sign. The caller adds the peso
// sign where it wants one.
func (c Centavos) Format() string {
	plain := c.String()
	neg := strings.HasPrefix(plain, "-")
	plain = strings.TrimPrefix(plain, "-")

	whole, frac, _ := strings.Cut(plain, ".")
	var b strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(digit)
	}
	out := b.String() + "." + frac
	if neg {
		return "-" + out
	}
	return out
}

// ParsePeso reads an amount as a person types it at a counter: 1120, 1,120.00,
// ₱1 120.50, P90.5, a leading minus. It refuses anything with more than two
// decimal places, because a price in thirds of a centavo is a keying mistake
// rather than a price.
func ParsePeso(s string) (Centavos, error) {
	raw := strings.TrimSpace(s)
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ',', ' ', ' ', '_', '₱': // comma, spaces, underscore, peso sign
			return -1
		}
		return r
	}, raw)
	cleaned = strings.TrimPrefix(cleaned, "PHP")
	cleaned = strings.TrimPrefix(cleaned, "Php")
	cleaned = strings.TrimPrefix(cleaned, "php")
	if len(cleaned) > 1 && (cleaned[0] == 'P' || cleaned[0] == 'p') {
		cleaned = cleaned[1:]
	}

	if cleaned == "" {
		return 0, errors.New("no amount given")
	}

	neg := false
	switch cleaned[0] {
	case '-':
		neg = true
		cleaned = cleaned[1:]
	case '+':
		cleaned = cleaned[1:]
	}
	if cleaned == "" || cleaned == "." {
		return 0, fmt.Errorf("%q is not an amount", raw)
	}

	whole, frac, hasFrac := strings.Cut(cleaned, ".")
	if whole == "" {
		whole = "0"
	}
	if hasFrac {
		if len(frac) > 2 {
			return 0, fmt.Errorf("%q has more than two decimal places — amounts are in pesos and centavos", raw)
		}
		for len(frac) < 2 {
			frac += "0"
		}
	} else {
		frac = "00"
	}

	wholeN, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", raw)
	}
	fracN, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", raw)
	}
	if wholeN > int64(MaxAmount)/centavosPerPeso {
		return 0, fmt.Errorf("%q is larger than this tool will work with", raw)
	}

	total := Centavos(wholeN*centavosPerPeso + fracN)
	if neg {
		total = -total
	}
	return total, nil
}

// MarshalJSON writes the amount as a quoted decimal string, so the data file
// and the interface both carry it exactly and legibly.
func (c Centavos) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.String() + `"`), nil
}

// UnmarshalJSON reads an amount written either as a quoted decimal string
// ("1120.00") or as a bare JSON number (1120 or 1120.5), in both cases as
// pesos. The digits are read from the literal text rather than through a
// floating-point number, so nothing is lost on the way in.
func (c *Centavos) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "null" || text == `""` {
		*c = 0
		return nil
	}
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	parsed, err := ParsePeso(text)
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// mulDivRound works out c * num / den on whole centavos, rounding a half
// centavo away from zero — the ordinary commercial rounding a till does, and
// the one that leaves the discount on the buyer's side of a tie.
//
// It is the only place in this package where an amount is rounded.
func mulDivRound(c Centavos, num, den int64) Centavos {
	if den == 0 {
		return 0
	}
	v := int64(c) * num
	if den < 0 {
		v, den = -v, -den
	}
	if v < 0 {
		return Centavos(-((-v*2 + den) / (den * 2)))
	}
	return Centavos((v*2 + den) / (den * 2))
}

// Percent returns pct per cent of the amount, rounded to the centavo.
func (c Centavos) Percent(pct int) Centavos { return mulDivRound(c, int64(pct), 100) }

// NetOfVAT takes a VAT-inclusive amount back to the selling price before tax:
// amount / (1 + rate). At a rate of 12 that is amount x 100 / 112.
//
// This is the step that has to come first for a qualified buyer. The discount
// is then taken on what this returns.
func (c Centavos) NetOfVAT(vatPercent int) Centavos {
	return mulDivRound(c, 100, int64(100+vatPercent))
}

// VATWithin returns the tax already inside a VAT-inclusive amount:
// amount x rate / (100 + rate). It is exactly the amount less NetOfVAT, which
// is how it is worked out here, so the two can never disagree by a centavo.
func (c Centavos) VATWithin(vatPercent int) Centavos {
	return c - c.NetOfVAT(vatPercent)
}

// Split divides an amount into n equal shares. Where it does not divide
// evenly, the odd centavos go to the earliest shares, so the shares always add
// back to exactly the amount split — a bill divided between diners can never
// gain or lose a centavo on the way.
func Split(total Centavos, n int) []Centavos {
	if n <= 0 {
		return nil
	}
	neg := total < 0
	if neg {
		total = -total
	}

	each := total / Centavos(n)
	odd := int(total % Centavos(n))

	shares := make([]Centavos, n)
	for i := range shares {
		shares[i] = each
		if i < odd {
			shares[i]++
		}
		if neg {
			shares[i] = -shares[i]
		}
	}
	return shares
}
