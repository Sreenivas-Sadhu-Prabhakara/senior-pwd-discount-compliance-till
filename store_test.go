package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/senior-pwd-discount-compliance-till/internal/till"
)

// sampleBill is the worked example: one senior citizen, dining alone, on a
// VAT-inclusive bill of 1,120.00.
func sampleBill() till.Bill {
	return till.Bill{
		Date:      till.MustParseDate("2026-03-15"),
		Reference: "OR-1001",
		Lines: []till.Line{
			{Description: "Lunch", Amount: till.Pesos(1120), Covered: true},
		},
		Persons: 1,
		Buyers: []till.Buyer{
			{Name: "Corazon Reyes", Basis: till.BasisSenior, IDNo: "OSCA-12345"},
		},
		Allocation: till.AllocationPerHead,
	}
}

// recordBill works a bill out and records it, the way the interface does.
func recordBill(t *testing.T, st *store, bill till.Bill) recordResult {
	t.Helper()
	computed, err := till.Compute(bill, st.snapshot().Settings.Rates)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	result, err := st.record(till.SalesFrom(bill, computed), till.MustParseDate("2026-03-31"))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	return result
}

func newStore(t *testing.T) (*store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "record.json")
	st, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	return st, path
}

func TestOpenStoreStartsEmptyAndWritesNothing(t *testing.T) {
	st, path := newStore(t)

	if got := len(st.snapshot().Sales); got != 0 {
		t.Fatalf("a new record has %d entries", got)
	}
	if !st.snapshot().Settings.Rates.IsStatutory() {
		t.Fatalf("a new record does not start on the statutory figures: %+v", st.snapshot().Settings.Rates)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("opening the record created the file before anything was saved")
	}
}

func TestRecordSavesAndReloads(t *testing.T) {
	st, path := newStore(t)

	result := recordBill(t, st, sampleBill())
	if len(result.Saved) != 1 || len(result.Skipped) != 0 {
		t.Fatalf("recording one bill saved %d and skipped %d", len(result.Saved), len(result.Skipped))
	}
	if result.Saved[0].ID == "" {
		t.Fatal("the saved entry has no handle")
	}
	if result.Saved[0].Discount != till.Pesos(200) {
		t.Fatalf("the saved discount is %s, want 200.00", result.Saved[0].Discount.Format())
	}

	// The file is on disk, readable, and only the owner may read it.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the record was not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("the record is mode %04o, want 0600", mode)
	}

	// Reopening gives back the same entries.
	again, err := openStore(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	reloaded := again.snapshot().Sales
	if len(reloaded) != 1 {
		t.Fatalf("reopening gave %d entries", len(reloaded))
	}
	if reloaded[0].Gross != till.Pesos(1120) || reloaded[0].VATExempt != till.Pesos(1000) ||
		reloaded[0].Discount != till.Pesos(200) || reloaded[0].Net != till.Pesos(800) {
		t.Fatalf("the amounts did not survive the round trip: %+v", reloaded[0])
	}
	if err := reloaded[0].Validate(till.MustParseDate("2026-03-31")); err != nil {
		t.Fatalf("a reloaded entry no longer validates: %v", err)
	}
}

func TestRecordRefusesTheSameBillTwice(t *testing.T) {
	st, _ := newStore(t)

	recordBill(t, st, sampleBill())

	computed, err := till.Compute(sampleBill(), st.snapshot().Settings.Rates)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.record(till.SalesFrom(sampleBill(), computed), till.MustParseDate("2026-03-31"))
	if err == nil {
		t.Fatal("the same receipt was recorded twice — the deduction would be overstated")
	}
	if !strings.Contains(err.Error(), "already in the sales record") {
		t.Fatalf("error %q does not explain the duplicate", err)
	}
	if got := len(st.snapshot().Sales); got != 1 {
		t.Fatalf("the record holds %d entries after a refused duplicate", got)
	}
}

func TestRecordSkipsOnlyTheBuyerAlreadyThere(t *testing.T) {
	st, _ := newStore(t)

	// One bill, one qualified buyer, recorded.
	first := sampleBill()
	first.Reference = "OR-2000"
	recordBill(t, st, first)

	// The same receipt, now with a second card shown. The buyer already in
	// the record is left alone and the new one is added.
	second := first
	second.Persons = 2
	second.Buyers = []till.Buyer{
		{Name: "Corazon Reyes", Basis: till.BasisSenior, IDNo: "OSCA-12345"},
		{Name: "Noel Garcia", Basis: till.BasisPWD, IDNo: "PWD-77-0042"},
	}
	computed, err := till.Compute(second, st.snapshot().Settings.Rates)
	if err != nil {
		t.Fatal(err)
	}
	result, err := st.record(till.SalesFrom(second, computed), till.MustParseDate("2026-03-31"))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(result.Saved) != 1 || len(result.Skipped) != 1 {
		t.Fatalf("saved %d and skipped %d, want one of each", len(result.Saved), len(result.Skipped))
	}
	if result.Saved[0].IDNo != "PWD-77-0042" {
		t.Errorf("the wrong buyer was added: %s", result.Saved[0].IDNo)
	}
	if result.Skipped[0].IDNo != "OSCA-12345" {
		t.Errorf("the wrong buyer was skipped: %s", result.Skipped[0].IDNo)
	}
}

func TestRecordRefusesNonsense(t *testing.T) {
	st, _ := newStore(t)

	cases := []struct {
		name    string
		sales   []till.Sale
		wantErr string
	}{
		{name: "nothing to record", sales: nil, wantErr: "no qualified buyer"},
		{
			name: "an entry whose arithmetic does not hold",
			sales: []till.Sale{{
				Date:      till.MustParseDate("2026-03-15"),
				Reference: "OR-9",
				BuyerName: "Corazon Reyes",
				Basis:     till.BasisSenior,
				IDNo:      "OSCA-1",
				Gross:     till.Pesos(1120),
				VATExempt: till.Pesos(1000),
				Discount:  till.Pesos(224), // 20% of the VAT-inclusive price: wrong
				Net:       till.Pesos(776),
			}},
			wantErr: "at 20% is 200.00, not 224.00",
		},
		{
			name: "an entry with no card number",
			sales: []till.Sale{{
				Date:      till.MustParseDate("2026-03-15"),
				Reference: "OR-9",
				BuyerName: "Corazon Reyes",
				Basis:     till.BasisSenior,
				Gross:     till.Pesos(1120),
				VATExempt: till.Pesos(1000),
				Discount:  till.Pesos(200),
				Net:       till.Pesos(800),
			}},
			wantErr: "needs the osca",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := st.record(c.sales, till.MustParseDate("2026-03-31"))
			if err == nil {
				t.Fatalf("the record accepted it, wanted %q", c.wantErr)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.wantErr)) {
				t.Fatalf("error %q does not mention %q", err, c.wantErr)
			}
			if got := len(st.snapshot().Sales); got != 0 {
				t.Fatalf("a refused entry left %d entries behind", got)
			}
		})
	}
}

func TestDeleteSale(t *testing.T) {
	st, _ := newStore(t)
	saved := recordBill(t, st, sampleBill()).Saved[0]

	removed, err := st.deleteSale(saved.ID)
	if err != nil {
		t.Fatalf("deleteSale: %v", err)
	}
	if removed.Reference != "OR-1001" {
		t.Errorf("removed %+v", removed)
	}
	if got := len(st.snapshot().Sales); got != 0 {
		t.Fatalf("the record still holds %d entries", got)
	}
	if _, err := st.deleteSale(saved.ID); err == nil {
		t.Fatal("deleting the same entry twice succeeded")
	}
	if _, err := st.deleteSale("  "); err == nil {
		t.Fatal("deleting nothing succeeded")
	}
}

func TestPutSettingsNormalisesAndPersists(t *testing.T) {
	st, path := newStore(t)

	saved, err := st.putSettings(settings{
		Establishment: till.Establishment{Name: "  Dalisay   Kitchen ", TIN: " 123-456-789-0000 "},
		Rates:         till.Rates{VATPercent: -5, DiscountPercent: 0, VATRegistered: false},
	})
	if err != nil {
		t.Fatalf("putSettings: %v", err)
	}
	if saved.Establishment.Name != "Dalisay Kitchen" {
		t.Errorf("the name was not tidied: %q", saved.Establishment.Name)
	}
	if saved.Rates.VATPercent != till.VATRatePercent || saved.Rates.DiscountPercent != till.StatutoryDiscountPercent {
		t.Errorf("impossible rates were not brought back to the statute: %+v", saved.Rates)
	}
	if saved.Rates.VATRegistered {
		t.Error("the VAT registration flag was overwritten")
	}

	again, err := openStore(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	if again.snapshot().Settings.Establishment.TIN != "123-456-789-0000" {
		t.Errorf("the settings did not survive the round trip: %+v", again.snapshot().Settings)
	}
}

func TestOpenStoreHandlesABadFile(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "an empty file is an empty record", body: ""},
		{name: "whitespace is an empty record", body: "   \n"},
		{name: "not JSON at all", body: "{nope", wantErr: "move the file aside"},
		{name: "a newer format", body: `{"version":99,"sales":[]}`, wantErr: "newer version"},
		{name: "an amount that is not an amount", body: `{"version":1,"sales":[{"gross":"free"}]}`, wantErr: "not an amount"},
		{name: "a date that is not a date", body: `{"version":1,"sales":[{"date":"15/03/2026"}]}`, wantErr: "not a date"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "record.json")
			if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
				t.Fatal(err)
			}

			st, err := openStore(path)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("openStore: %v", err)
				}
				if got := len(st.snapshot().Sales); got != 0 {
					t.Fatalf("%d entries came out of an empty file", got)
				}
				return
			}
			if err == nil {
				t.Fatalf("openStore accepted the file, wanted %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error %q does not mention %q", err, c.wantErr)
			}
		})
	}
}

func TestSaveIsAtomicAndLeavesNoTemporaryFile(t *testing.T) {
	st, path := newStore(t)
	recordBill(t, st, sampleBill())

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != filepath.Base(path) {
			t.Errorf("saving left %q behind", entry.Name())
		}
	}

	// The file is valid JSON in its own right, with the version written in.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var loaded fileData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("the saved file is not JSON: %v", err)
	}
	if loaded.Version != fileFormatVersion {
		t.Errorf("the file says format %d, this build writes %d", loaded.Version, fileFormatVersion)
	}
}

func TestNewIDIsDistinct(t *testing.T) {
	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		id := newID()
		if id == "" {
			t.Fatal("an empty handle was made")
		}
		if seen[id] {
			t.Fatalf("the handle %q was made twice", id)
		}
		seen[id] = true
	}
}
