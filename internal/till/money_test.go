package till

import (
	"encoding/json"
	"testing"
)

func TestParsePeso(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  Centavos
		wantE bool
	}{
		{name: "whole pesos", in: "1120", want: 112000},
		{name: "two decimals", in: "1120.00", want: 112000},
		{name: "one decimal is tenths of a peso", in: "90.5", want: 9050},
		{name: "grouped with commas", in: "1,234,567.89", want: 123456789},
		{name: "peso sign and a space", in: "₱1 120.50", want: 112050},
		{name: "P prefix", in: "P90.50", want: 9050},
		{name: "PHP prefix", in: "PHP250", want: 25000},
		{name: "leading plus", in: "+12.34", want: 1234},
		{name: "negative", in: "-12.50", want: -1250},
		{name: "centavos only", in: ".07", want: 7},
		{name: "zero", in: "0", want: 0},
		{name: "zero with decimals", in: "0.00", want: 0},
		{name: "surrounding space", in: "  42.00  ", want: 4200},

		{name: "empty", in: "", wantE: true},
		{name: "only spaces", in: "   ", wantE: true},
		{name: "three decimal places", in: "10.005", wantE: true},
		{name: "not a number", in: "free", wantE: true},
		{name: "sign only", in: "-", wantE: true},
		{name: "dot only", in: ".", wantE: true},
		{name: "letters in the centavos", in: "10.a", wantE: true},
		{name: "over the working limit", in: "100000001.00", wantE: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParsePeso(c.in)
			if c.wantE {
				if err == nil {
					t.Fatalf("ParsePeso(%q) = %v, wanted an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePeso(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParsePeso(%q) = %d centavos, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestCentavosStringAndFormat(t *testing.T) {
	cases := []struct {
		in         Centavos
		wantString string
		wantFormat string
	}{
		{in: 0, wantString: "0.00", wantFormat: "0.00"},
		{in: 7, wantString: "0.07", wantFormat: "0.07"},
		{in: 70, wantString: "0.70", wantFormat: "0.70"},
		{in: 100, wantString: "1.00", wantFormat: "1.00"},
		{in: 100000, wantString: "1000.00", wantFormat: "1,000.00"},
		{in: 112000, wantString: "1120.00", wantFormat: "1,120.00"},
		{in: 123456789, wantString: "1234567.89", wantFormat: "1,234,567.89"},
		{in: -1250, wantString: "-12.50", wantFormat: "-12.50"},
		{in: -123456789, wantString: "-1234567.89", wantFormat: "-1,234,567.89"},
	}

	for _, c := range cases {
		t.Run(c.wantString, func(t *testing.T) {
			if got := c.in.String(); got != c.wantString {
				t.Errorf("String() = %q, want %q", got, c.wantString)
			}
			if got := c.in.Format(); got != c.wantFormat {
				t.Errorf("Format() = %q, want %q", got, c.wantFormat)
			}
		})
	}
}

func TestParseFormatRoundTrip(t *testing.T) {
	for _, in := range []Centavos{0, 1, 99, 100, 112000, -5, 123456789, MaxAmount} {
		back, err := ParsePeso(in.String())
		if err != nil {
			t.Fatalf("ParsePeso(%q): %v", in.String(), err)
		}
		if back != in {
			t.Fatalf("round trip of %d centavos came back as %d", in, back)
		}
	}
}

func TestNetOfVATAndVATWithin(t *testing.T) {
	cases := []struct {
		name       string
		gross      Centavos
		vat        int
		wantNet    Centavos
		wantWithin Centavos
	}{
		{name: "the worked example from the regulations", gross: 112000, vat: 12, wantNet: 100000, wantWithin: 12000},
		{name: "one head of a shared bill", gross: 56000, vat: 12, wantNet: 50000, wantWithin: 6000},
		{name: "not an even division, rounds up at a half", gross: 33334, vat: 12, wantNet: 29763, wantWithin: 3571},
		{name: "one peso", gross: 100, vat: 12, wantNet: 89, wantWithin: 11},
		{name: "one centavo", gross: 1, vat: 12, wantNet: 1, wantWithin: 0},
		{name: "zero", gross: 0, vat: 12, wantNet: 0, wantWithin: 0},
		{name: "a rate of zero leaves the amount alone", gross: 112000, vat: 0, wantNet: 112000, wantWithin: 0},
		{name: "a different rate, should one be legislated", gross: 110000, vat: 10, wantNet: 100000, wantWithin: 10000},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			net := c.gross.NetOfVAT(c.vat)
			if net != c.wantNet {
				t.Errorf("NetOfVAT(%d) of %d = %d, want %d", c.vat, c.gross, net, c.wantNet)
			}
			within := c.gross.VATWithin(c.vat)
			if within != c.wantWithin {
				t.Errorf("VATWithin(%d) of %d = %d, want %d", c.vat, c.gross, within, c.wantWithin)
			}
			if net+within != c.gross {
				t.Errorf("the net %d and the tax %d do not add back to the gross %d", net, within, c.gross)
			}
		})
	}
}

func TestPercentRounding(t *testing.T) {
	cases := []struct {
		name string
		base Centavos
		pct  int
		want Centavos
	}{
		{name: "the worked example", base: 100000, pct: 20, want: 20000},
		{name: "exact fifth", base: 50000, pct: 20, want: 10000},
		{name: "half a centavo rounds away from zero", base: 29763, pct: 20, want: 5953},
		{name: "another half centavo", base: 5, pct: 10, want: 1},
		{name: "rounds down below a half", base: 29762, pct: 20, want: 5952},
		{name: "one centavo", base: 1, pct: 20, want: 0},
		{name: "three centavos", base: 3, pct: 20, want: 1},
		{name: "zero", base: 0, pct: 20, want: 0},
		{name: "nothing off", base: 100000, pct: 0, want: 0},
		{name: "everything off", base: 100000, pct: 100, want: 100000},
		{name: "negative rounds away from zero too", base: -29763, pct: 20, want: -5953},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.base.Percent(c.pct); got != c.want {
				t.Fatalf("%d centavos at %d%% = %d, want %d", c.base, c.pct, got, c.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		name  string
		total Centavos
		n     int
		want  []Centavos
	}{
		{name: "even", total: 224000, n: 4, want: []Centavos{56000, 56000, 56000, 56000}},
		{name: "one person", total: 112000, n: 1, want: []Centavos{112000}},
		{name: "odd centavo to the first share", total: 100000, n: 3, want: []Centavos{33334, 33333, 33333}},
		{name: "two odd centavos", total: 1000, n: 3, want: []Centavos{334, 333, 333}},
		{name: "less than a centavo each", total: 2, n: 5, want: []Centavos{1, 1, 0, 0, 0}},
		{name: "zero", total: 0, n: 3, want: []Centavos{0, 0, 0}},
		{name: "negative keeps its sign", total: -1000, n: 3, want: []Centavos{-334, -333, -333}},
		{name: "nobody", total: 1000, n: 0, want: nil},
		{name: "a negative headcount", total: 1000, n: -2, want: nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Split(c.total, c.n)
			if len(got) != len(c.want) {
				t.Fatalf("Split(%d, %d) returned %d shares, want %d", c.total, c.n, len(got), len(c.want))
			}
			var sum Centavos
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("share %d = %d, want %d", i, got[i], c.want[i])
				}
				sum += got[i]
			}
			if len(got) > 0 && sum != c.total {
				t.Errorf("the shares add to %d, not to the %d split", sum, c.total)
			}
		})
	}
}

func TestSplitAlwaysAddsBack(t *testing.T) {
	for total := Centavos(0); total <= 400; total += 7 {
		for n := 1; n <= 13; n++ {
			var sum Centavos
			for _, share := range Split(total, n) {
				sum += share
			}
			if sum != total {
				t.Fatalf("splitting %d between %d lost or gained money: %d", total, n, sum)
			}
		}
	}
}

func TestCentavosJSON(t *testing.T) {
	type holder struct {
		Amount Centavos `json:"amount"`
	}

	t.Run("writes a quoted decimal", func(t *testing.T) {
		body, err := json.Marshal(holder{Amount: 112000})
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"amount":"1120.00"}` {
			t.Fatalf("marshalled to %s", body)
		}
	})

	cases := []struct {
		name  string
		in    string
		want  Centavos
		wantE bool
	}{
		{name: "quoted decimal", in: `{"amount":"1120.00"}`, want: 112000},
		{name: "bare number is pesos", in: `{"amount":1120}`, want: 112000},
		{name: "bare number with decimals", in: `{"amount":1120.5}`, want: 112050},
		{name: "null is nothing", in: `{"amount":null}`, want: 0},
		{name: "empty string is nothing", in: `{"amount":""}`, want: 0},
		{name: "missing is nothing", in: `{}`, want: 0},
		{name: "nonsense is refused", in: `{"amount":"free"}`, wantE: true},
		{name: "too many decimals is refused", in: `{"amount":"10.005"}`, wantE: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var h holder
			err := json.Unmarshal([]byte(c.in), &h)
			if c.wantE {
				if err == nil {
					t.Fatalf("unmarshalling %s gave %v, wanted an error", c.in, h.Amount)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshalling %s: %v", c.in, err)
			}
			if h.Amount != c.want {
				t.Fatalf("unmarshalling %s gave %d centavos, want %d", c.in, h.Amount, c.want)
			}
		})
	}
}

func TestPesos(t *testing.T) {
	if got := Pesos(1120); got != 112000 {
		t.Fatalf("Pesos(1120) = %d, want 112000", got)
	}
	if got := Pesos(0); got != 0 {
		t.Fatalf("Pesos(0) = %d, want 0", got)
	}
}
