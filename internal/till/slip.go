package till

import (
	"fmt"
	"strings"
)

// Establishment is who is granting the discount: the details that have to
// appear on the receipt and at the head of the sales record.
type Establishment struct {
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Address string `json:"address"`
	TIN     string `json:"tin"`
}

// Normalise tidies the establishment's details.
func (e Establishment) Normalise() Establishment {
	e.Name = strings.Join(strings.Fields(e.Name), " ")
	e.Branch = strings.Join(strings.Fields(e.Branch), " ")
	e.Address = strings.Join(strings.Fields(e.Address), " ")
	e.TIN = strings.ToUpper(strings.Join(strings.Fields(e.TIN), " "))
	return e
}

// Heading is the establishment in one line, for the top of a slip or a page.
func (e Establishment) Heading() string {
	parts := make([]string, 0, 4)
	if e.Name != "" {
		parts = append(parts, e.Name)
	} else {
		parts = append(parts, "(establishment name not set)")
	}
	if e.Branch != "" {
		parts = append(parts, e.Branch)
	}
	if e.Address != "" {
		parts = append(parts, e.Address)
	}
	if e.TIN != "" {
		parts = append(parts, "TIN "+e.TIN)
	}
	return strings.Join(parts, " · ")
}

// slipWidth is how wide the plain-text slip is set, which suits both a sheet
// of paper and a receipt printer.
const slipWidth = 72

// Slip writes the charge slip for one transaction as plain text: the order of
// operations line by line, then each qualified buyer with the identification
// number taken from them.
//
// It is the working behind the amount charged. An establishment that is asked
// how it arrived at a figure can hand this over.
func Slip(e Establishment, b Bill, c Computation) string {
	var out strings.Builder

	rule := strings.Repeat("-", slipWidth)
	writeCentred(&out, e.Heading())
	writeCentred(&out, "SENIOR CITIZEN / PERSON WITH DISABILITY CHARGE SLIP")
	out.WriteString(rule + "\n")

	writePair(&out, "Date", orDash(b.Date.String()))
	writePair(&out, "Receipt no.", orDash(b.Reference))
	writePair(&out, "People sharing the bill", fmt.Sprintf("%d", b.Persons))
	writePair(&out, "Qualified buyers", fmt.Sprintf("%d", c.Qualified))
	writePair(&out, "Share basis", allocationLabel(b.Allocation))
	out.WriteString(rule + "\n")

	out.WriteString("BILL AS PRICED\n")
	for i, line := range b.Lines {
		mark := "not covered"
		if line.Covered {
			mark = "covered"
			if line.VATExempt {
				mark = "covered, no VAT"
			}
		}
		writeAmount(&out, fmt.Sprintf("  %s (%s)", lineName(line, i), mark), line.Amount, false)
	}
	writeAmount(&out, "  Bill total as priced", c.BillGross, false)
	out.WriteString(rule + "\n")

	out.WriteString("HOW THE AMOUNT DUE WAS WORKED OUT\n")
	for _, step := range c.Steps {
		writeAmount(&out, "  "+step.Label, step.Amount, step.Deduction)
		if step.Authority != "" {
			out.WriteString(wrapIndented("      "+step.Authority, slipWidth))
		}
	}
	out.WriteString(rule + "\n")

	if len(c.Shares) > 0 {
		out.WriteString("QUALIFIED BUYERS\n")
		for _, share := range c.Shares {
			out.WriteString(fmt.Sprintf("  %s — %s\n", share.Buyer.Name, share.Buyer.Basis.Label()))
			out.WriteString(fmt.Sprintf("      %s %s", share.Buyer.Basis.IDLabel(), orDash(share.Buyer.IDNo)))
			if share.Buyer.Booklet != "" {
				out.WriteString("  ·  booklet " + share.Buyer.Booklet)
			}
			out.WriteString("\n")
			writeAmount(&out, "      Share as priced", share.Gross, false)
			writeAmount(&out, "      VAT not charged", share.VATRemoved, true)
			writeAmount(&out, "      VAT-exempt selling price", share.DiscountBase, false)
			writeAmount(&out, fmt.Sprintf("      Discount at %d%%", c.Rates.DiscountPercent), share.Discount, true)
			writeAmount(&out, "      Amount due", share.Due, false)
		}
		out.WriteString(rule + "\n")
	}

	writeAmount(&out, "TOTAL AMOUNT DUE", c.TotalDue, false)
	writeAmount(&out, "Saved by the qualified buyers", c.Saved, false)
	if c.OutputVAT > 0 {
		writeAmount(&out, "Output VAT still collected on this bill", c.OutputVAT, false)
	}
	out.WriteString(rule + "\n")

	if c.Promotion != nil {
		out.WriteString(wrapIndented("PROMOTIONAL DISCOUNT: "+c.Promotion.Advice, slipWidth))
		out.WriteString(wrapIndented("  "+c.Promotion.Authority, slipWidth))
		out.WriteString(rule + "\n")
	}

	out.WriteString("AUTHORITY\n")
	out.WriteString(wrapIndented(fmt.Sprintf(
		"  %d%% discount and VAT exemption: RA 9994 s. 4(a) for senior citizens, RA 10754 amending RA 7277 s. 32 for persons with disability. The discount is taken on the amount net of the %d%% value-added tax: BIR RR 7-2010 s. 6 and RR 5-2017.",
		c.Rates.DiscountPercent, c.Rates.VATPercent), slipWidth))
	for _, note := range c.Notes {
		writeBullet(&out, note)
	}
	return out.String()
}

// SummaryText writes a period summary as plain text, for filing with the
// month's or the year's papers.
func SummaryText(e Establishment, s Summary) string {
	var out strings.Builder
	rule := strings.Repeat("-", slipWidth)

	writeCentred(&out, e.Heading())
	writeCentred(&out, "SENIOR CITIZEN AND PWD DISCOUNTS GRANTED")
	writeCentred(&out, s.Period.Label)
	out.WriteString(rule + "\n")

	writeTally(&out, "All qualified sales", s.Total)
	out.WriteString("\n")
	writeTally(&out, "Senior citizens (RA 9994)", s.Senior)
	out.WriteString("\n")
	writeTally(&out, "Persons with disability (RA 10754)", s.PWD)
	out.WriteString(rule + "\n")

	if len(s.Months) > 1 {
		out.WriteString("BY MONTH\n")
		for _, month := range s.Months {
			writeAmount(&out, fmt.Sprintf("  %s (%s)", month.Label, plural(month.Tally.Entries, "entry", "entries")), month.Tally.Discount, false)
		}
		out.WriteString(rule + "\n")
	}

	writeAmount(&out, "DISCOUNTS GRANTED — claimed as a deduction from gross income", s.Deduction(), false)
	out.WriteString(rule + "\n")
	for _, note := range s.Notes {
		writeBullet(&out, note)
	}
	return out.String()
}

// RegisterText writes the register itself as a plain-text table, every column
// sized to its contents, for printing or pasting into a book.
func RegisterText(e Establishment, sales []Sale, p Period) string {
	var out strings.Builder

	writeCentred(&out, e.Heading())
	writeCentred(&out, "SENIOR CITIZEN AND PWD SALES RECORD")
	writeCentred(&out, p.Label)
	out.WriteString("\n")

	rows := [][]string{RegisterColumns()}
	rows = append(rows, RegisterRows(sales)...)
	rows = append(rows, RegisterTotalsRow(Summarise(sales, p).Total))
	out.WriteString(Table(rows))

	out.WriteString("\nKept under BIR RR 7-2010 (senior citizens) and RR 5-2017 (persons with disability).\n")
	out.WriteString("Amounts in Philippine pesos. Figures are indicative; confirm the required columns\nand retention period with the Bureau of Internal Revenue.\n")
	return out.String()
}

// Table lays rows of cells out as a plain-text table, every column as wide as
// its widest cell. The first row is taken as the headings and underlined.
func Table(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	widths := make([]int, 0, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			for i >= len(widths) {
				widths = append(widths, 0)
			}
			if n := len([]rune(cell)); n > widths[i] {
				widths[i] = n
			}
		}
	}

	var out strings.Builder
	for r, row := range rows {
		cells := make([]string, 0, len(widths))
		for i := range widths {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			cells = append(cells, pad(cell, widths[i]))
		}
		out.WriteString(strings.TrimRight(strings.Join(cells, "  "), " ") + "\n")
		if r == 0 {
			dashes := make([]string, 0, len(widths))
			for _, w := range widths {
				dashes = append(dashes, strings.Repeat("-", w))
			}
			out.WriteString(strings.Join(dashes, "  ") + "\n")
		}
	}
	return out.String()
}

// writeTally writes one tally as a labelled block.
func writeTally(out *strings.Builder, label string, t Tally) {
	out.WriteString(label + "\n")
	writePair(out, "  Entries", fmt.Sprintf("%d over %s", t.Entries, plural(t.Buyers, "buyer", "buyers")))
	writeAmount(out, "  Gross sales, VAT-inclusive", t.Gross, false)
	writeAmount(out, "  VAT not charged", t.VATForgone, false)
	writeAmount(out, "  VAT-exempt sales", t.VATExempt, false)
	writeAmount(out, "  Discounts granted", t.Discount, false)
	writeAmount(out, "  Net sales collected", t.Net, false)
}

// writeAmount writes a label on the left and a right-aligned amount, with a
// deduction shown in brackets the way a ledger does.
func writeAmount(out *strings.Builder, label string, amount Centavos, deduction bool) {
	value := amount.Format()
	if deduction && amount != 0 {
		value = "(" + value + ")"
	}
	label = strings.TrimRight(label, " ")
	if len([]rune(label))+len([]rune(value))+2 > slipWidth {
		// Too long to sit beside its amount: wrap the label and hang the
		// amount under it, right-aligned, so the column still reads down.
		out.WriteString(wrapIndented(label, slipWidth))
		out.WriteString(strings.Repeat(" ", max(0, slipWidth-len([]rune(value)))) + value + "\n")
		return
	}
	gap := slipWidth - len([]rune(label)) - len([]rune(value))
	out.WriteString(label + strings.Repeat(" ", gap) + value + "\n")
}

// writeBullet writes a note as a dash item, hanging the lines after the first
// under the text rather than under the dash.
func writeBullet(out *strings.Builder, text string) {
	const lead = "  - "
	const hang = "    "
	for i, line := range wrap(text, slipWidth-len(lead)) {
		if i == 0 {
			out.WriteString(lead + line + "\n")
			continue
		}
		out.WriteString(hang + line + "\n")
	}
}

// writePair writes a label and a value with a dotted lead between them.
func writePair(out *strings.Builder, label, value string) {
	gap := slipWidth - len([]rune(label)) - len([]rune(value))
	if gap < 1 {
		gap = 1
	}
	out.WriteString(label + strings.Repeat(" ", gap) + value + "\n")
}

// writeCentred writes a line centred in the slip width, wrapping it first if
// it is too long to centre.
func writeCentred(out *strings.Builder, text string) {
	if text == "" {
		return
	}
	for _, line := range wrap(text, slipWidth) {
		pad := (slipWidth - len([]rune(line))) / 2
		if pad < 0 {
			pad = 0
		}
		out.WriteString(strings.Repeat(" ", pad) + line + "\n")
	}
}

// wrapIndented wraps text to a width, keeping the leading spaces of the first
// line on every line after it, and returns it with a trailing newline.
func wrapIndented(text string, width int) string {
	indent := text[:len(text)-len(strings.TrimLeft(text, " "))]
	lines := wrap(strings.TrimLeft(text, " "), width-len(indent))
	var out strings.Builder
	for _, line := range lines {
		out.WriteString(indent + line + "\n")
	}
	return out.String()
}

// wrap breaks text into lines no longer than width, at spaces. A single word
// longer than the width is left long rather than cut.
func wrap(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if width < 8 {
		width = 8
	}
	lines := []string{}
	current := words[0]
	for _, word := range words[1:] {
		if len([]rune(current))+1+len([]rune(word)) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	return append(lines, current)
}

// plural writes a count with the right form of its noun.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// pad widens a cell to a column width.
func pad(s string, width int) string {
	if n := len([]rune(s)); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// orDash is a value, or a dash where there is none.
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
