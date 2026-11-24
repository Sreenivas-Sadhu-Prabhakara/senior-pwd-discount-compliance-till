package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/senior-pwd-discount-compliance-till/internal/till"
)

// api serves the JSON interface the page talks to. Every call is same-origin
// and nothing leaves the machine.
type api struct {
	store *store
}

// routes registers every endpoint.
func (a *api) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", a.getState)
	mux.HandleFunc("POST /api/quote", a.postQuote)
	mux.HandleFunc("POST /api/sales", a.postSales)
	mux.HandleFunc("DELETE /api/sales", a.deleteSale)
	mux.HandleFunc("PUT /api/settings", a.putSettings)
	mux.HandleFunc("GET /api/summary", a.getSummary)
	mux.HandleFunc("GET /api/register.csv", a.getRegisterCSV)
	mux.HandleFunc("GET /api/register.txt", a.getRegisterText)
	mux.HandleFunc("GET /api/summary.txt", a.getSummaryText)
}

// statutoryFigures are the figures in force, sent to the page so that it can
// state them rather than carrying a second copy of them.
type statutoryFigures struct {
	VATRatePercent             int    `json:"vatRatePercent"`
	DiscountPercent            int    `json:"discountPercent"`
	SharesPerPersonInGroup     int    `json:"sharesPerPersonInGroup"`
	DiscountsPerQualifiedBuyer int    `json:"discountsPerQualifiedBuyer"`
	DiscountOnVATExclusive     bool   `json:"discountOnVatExclusive"`
	PromotionIsAlternative     bool   `json:"promotionIsAlternative"`
	DiscountIsADeduction       bool   `json:"discountIsADeduction"`
	SeniorStatute              string `json:"seniorStatute"`
	PWDStatute                 string `json:"pwdStatute"`
	VATStatute                 string `json:"vatStatute"`
	InForceAsAt                string `json:"inForceAsAt"`
}

// shippedFigures are the statutory figures this build carries, with the
// authority for each. The settings may override the two rates; this is what
// they are overriding.
func shippedFigures() statutoryFigures {
	return statutoryFigures{
		VATRatePercent:             till.VATRatePercent,
		DiscountPercent:            till.StatutoryDiscountPercent,
		SharesPerPersonInGroup:     till.SharesPerPersonInGroup,
		DiscountsPerQualifiedBuyer: till.DiscountsPerQualifiedBuyer,
		DiscountOnVATExclusive:     till.DiscountAppliesToVATExclusiveAmount,
		PromotionIsAlternative:     till.PromotionalDiscountIsAnAlternative,
		DiscountIsADeduction:       till.DiscountIsADeductionFromGrossIncome,
		SeniorStatute:              "RA 9994 s. 4(a), with BIR RR 7-2010",
		PWDStatute:                 "RA 10754 amending RA 7277 s. 32, with BIR RR 5-2017",
		VATStatute:                 "NIRC s. 106 and s. 108, as amended",
		InForceAsAt:                "the figures in force when this tool was written — confirm the current ones with the BIR",
	}
}

// periodOption is one choice in the period picker.
type periodOption struct {
	Kind  till.PeriodKind `json:"kind"`
	Year  int             `json:"year"`
	Index int             `json:"index"`
	Label string          `json:"label"`
	From  till.Date       `json:"from"`
	To    till.Date       `json:"to"`
}

// statePayload is everything the page needs to draw itself.
type statePayload struct {
	Today     till.Date        `json:"today"`
	Settings  settings         `json:"settings"`
	Statutory statutoryFigures `json:"statutory"`
	Entries   int              `json:"entries"`
	Periods   []periodOption   `json:"periods"`
	DataFile  string           `json:"dataFile"`
}

// getState answers with the settings, the statutory figures and how much is in
// the sales record.
func (a *api) getState(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()

	writeJSON(w, http.StatusOK, statePayload{
		Today:     till.Today(),
		Settings:  data.Settings,
		Statutory: shippedFigures(),
		Entries:   len(data.Sales),
		Periods:   periodOptions(till.Today()),
		DataFile:  a.store.path,
	})
}

// quotePayload is a worked-out bill, the entries it would put into the sales
// record, and the charge slip for it.
type quotePayload struct {
	Computation till.Computation `json:"computation"`
	Entries     []till.Sale      `json:"entries"`
	Slip        string           `json:"slip"`
	Rates       till.Rates       `json:"rates"`
}

// postQuote works a bill out without recording anything, so the till can show
// the customer the figures before the sale is made.
func (a *api) postQuote(w http.ResponseWriter, r *http.Request) {
	var bill till.Bill
	if !readJSON(w, r, &bill) {
		return
	}
	data := a.store.snapshot()

	computed, err := till.Compute(bill, data.Settings.Rates)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, quotePayload{
		Computation: computed,
		Entries:     till.SalesFrom(bill, computed),
		Slip:        till.Slip(data.Settings.Establishment, bill, computed),
		Rates:       data.Settings.Rates,
	})
}

// recordPayload is the result of recording a bill.
type recordPayload struct {
	Computation till.Computation `json:"computation"`
	Saved       []till.Sale      `json:"saved"`
	Skipped     []till.Sale      `json:"skipped"`
	Slip        string           `json:"slip"`
	Entries     int              `json:"entries"`
}

// postSales works a bill out and puts one entry per qualified buyer into the
// sales record.
func (a *api) postSales(w http.ResponseWriter, r *http.Request) {
	var bill till.Bill
	if !readJSON(w, r, &bill) {
		return
	}
	data := a.store.snapshot()

	computed, err := till.Compute(bill, data.Settings.Rates)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	result, err := a.store.record(till.SalesFrom(bill, computed), till.Today())
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, recordPayload{
		Computation: computed,
		Saved:       result.Saved,
		Skipped:     result.Skipped,
		Slip:        till.Slip(data.Settings.Establishment, bill, computed),
		Entries:     len(a.store.snapshot().Sales),
	})
}

// deleteSale removes one entry from the sales record.
func (a *api) deleteSale(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeProblem(w, http.StatusBadRequest, errors.New("which entry? pass ?id=<entry>"))
		return
	}
	removed, err := a.store.deleteSale(id)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

// putSettings saves the establishment's details and the figures its sales are
// worked out on.
func (a *api) putSettings(w http.ResponseWriter, r *http.Request) {
	var in settings
	if !readJSON(w, r, &in) {
		return
	}
	saved, err := a.store.putSettings(in)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": saved})
}

// summaryPayload is a period of the sales record: the totals, the printable
// register and the column headings to print it under.
type summaryPayload struct {
	Summary   till.Summary `json:"summary"`
	Columns   []string     `json:"columns"`
	Rows      [][]string   `json:"rows"`
	TotalsRow []string     `json:"totalsRow"`
	Sales     []till.Sale  `json:"sales"`
	Deduction string       `json:"deduction"`
}

// getSummary answers with the register and the totals for a period.
func (a *api) getSummary(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	period, err := resolvePeriod(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	sales := till.InPeriod(data.Sales, period)
	summary := till.Summarise(data.Sales, period)

	writeJSON(w, http.StatusOK, summaryPayload{
		Summary:   summary,
		Columns:   till.RegisterColumns(),
		Rows:      till.RegisterRows(sales),
		TotalsRow: till.RegisterTotalsRow(summary.Total),
		Sales:     sales,
		Deduction: summary.Deduction().String(),
	})
}

// getRegisterCSV answers with the register for a period as a CSV file.
func (a *api) getRegisterCSV(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	period, err := resolvePeriod(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}

	sales := till.InPeriod(data.Sales, period)
	summary := till.Summarise(data.Sales, period)

	rows := till.RegisterRows(sales)
	rows = append(rows, till.RegisterTotalsRow(summary.Total))

	writeCSV(w, "senior-pwd-sales-record-"+safeFileName(period.Label)+".csv", till.RegisterColumns(), rows)
}

// getRegisterText answers with the printable register as plain text.
func (a *api) getRegisterText(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	period, err := resolvePeriod(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	body := till.RegisterText(data.Settings.Establishment, till.InPeriod(data.Sales, period), period)
	writeText(w, "senior-pwd-sales-record-"+safeFileName(period.Label)+".txt", body)
}

// getSummaryText answers with the period summary as plain text, to file with
// the month's or the year's papers.
func (a *api) getSummaryText(w http.ResponseWriter, r *http.Request) {
	data := a.store.snapshot()
	period, err := resolvePeriod(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, err)
		return
	}
	body := till.SummaryText(data.Settings.Establishment, till.Summarise(data.Sales, period))
	writeText(w, "senior-pwd-discounts-"+safeFileName(period.Label)+".txt", body)
}

// resolvePeriod reads a period from the query: a calendar month, quarter or
// year, or a span of dates. With nothing given it is the month we are in.
func resolvePeriod(r *http.Request) (till.Period, error) {
	q := r.URL.Query()

	if from := strings.TrimSpace(q.Get("from")); from != "" {
		fromDate, err := till.ParseDate(from)
		if err != nil {
			return till.Period{}, fmt.Errorf("from: %w", err)
		}
		toDate, err := till.ParseDate(q.Get("to"))
		if err != nil {
			return till.Period{}, fmt.Errorf("to: %w", err)
		}
		return till.CustomPeriod(fromDate, toDate)
	}

	today := till.Today()
	year, err := intParam(q.Get("year"), today.Year())
	if err != nil {
		return till.Period{}, fmt.Errorf("year: %w", err)
	}

	switch kind := till.PeriodKind(strings.TrimSpace(q.Get("kind"))); kind {
	case "", till.PeriodMonth:
		month, err := intParam(q.Get("index"), int(today.Month()))
		if err != nil {
			return till.Period{}, fmt.Errorf("month: %w", err)
		}
		return till.MonthPeriod(year, time.Month(month))
	case till.PeriodQuarter:
		quarter, err := intParam(q.Get("index"), (int(today.Month())-1)/3+1)
		if err != nil {
			return till.Period{}, fmt.Errorf("quarter: %w", err)
		}
		return till.QuarterPeriod(year, quarter)
	case till.PeriodYear:
		return till.YearPeriod(year)
	default:
		return till.Period{}, fmt.Errorf("%q is not a kind of period — use month, quarter, year, or a from and to date", kind)
	}
}

// periodOptions lists the periods a register can be read over: every month and
// quarter of this year and the two before it, and those years.
func periodOptions(today till.Date) []periodOption {
	var out []periodOption
	for year := today.Year(); year > today.Year()-3; year-- {
		if p, err := till.YearPeriod(year); err == nil {
			out = append(out, periodOption{Kind: till.PeriodYear, Year: year, Index: 0, Label: p.Label, From: p.From, To: p.To})
		}
		for quarter := 4; quarter >= 1; quarter-- {
			if p, err := till.QuarterPeriod(year, quarter); err == nil {
				out = append(out, periodOption{Kind: till.PeriodQuarter, Year: year, Index: quarter, Label: p.Label, From: p.From, To: p.To})
			}
		}
		for month := 12; month >= 1; month-- {
			if p, err := till.MonthPeriod(year, time.Month(month)); err == nil {
				out = append(out, periodOption{Kind: till.PeriodMonth, Year: year, Index: month, Label: p.Label, From: p.From, To: p.To})
			}
		}
	}
	return out
}

// intParam reads a whole number from the query, falling back when it is not
// given.
func intParam(raw string, fallback int) (int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback, fmt.Errorf("%q is not a number", v)
	}
	return n, nil
}

// readJSON reads a JSON request body, answering with a problem and reporting
// false when it cannot.
func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeProblem(w, http.StatusBadRequest, fmt.Errorf("this request did not read as JSON: %w", err))
		return false
	}
	return true
}

// writeJSON answers with a JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "could not prepare the answer", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeProblem answers with an error, splitting the several problems a form can
// have into a list the page can show against the fields.
func writeProblem(w http.ResponseWriter, status int, err error) {
	problems := strings.Split(err.Error(), "\n")
	writeJSON(w, status, map[string]any{
		"error":    strings.Join(problems, "; "),
		"problems": problems,
	})
}

// writeCSV answers with a CSV file for the browser to save.
func writeCSV(w http.ResponseWriter, filename string, header []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	out := csv.NewWriter(w)
	if err := out.Write(header); err != nil {
		return
	}
	for _, row := range rows {
		if err := out.Write(row); err != nil {
			return
		}
	}
	out.Flush()
}

// writeText answers with a plain-text file for the browser to save or print.
func writeText(w http.ResponseWriter, filename, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write([]byte(body))
}

// safeFileName reduces a period label to something safe to offer as a file
// name on any system.
func safeFileName(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "period"
	}
	return strings.ToLower(out)
}
