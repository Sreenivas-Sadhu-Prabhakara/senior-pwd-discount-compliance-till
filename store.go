package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/senior-pwd-discount-compliance-till/internal/till"
)

// fileFormatVersion is written into the data file so that a later version of
// the tool can tell what it is reading.
const fileFormatVersion = 1

// settings are the establishment's own details and the figures its sales are
// worked out on.
type settings struct {
	Establishment till.Establishment `json:"establishment"`
	Rates         till.Rates         `json:"rates"`
}

// normalise fills in anything missing with the statutory value.
func (s settings) normalise() settings {
	s.Establishment = s.Establishment.Normalise()
	s.Rates = s.Rates.Normalise()
	return s
}

// fileData is the whole contents of the data file: the establishment's
// settings and its sales record.
type fileData struct {
	Version  int         `json:"version"`
	Settings settings    `json:"settings"`
	Sales    []till.Sale `json:"sales"`
}

// store keeps the sales record in memory and writes the whole file out after
// every change. A single till's statutory record is small — a few thousand
// entries a year — so rewriting it is both simpler and safer than updating it
// in place.
type store struct {
	mu   sync.RWMutex
	path string
	data fileData
}

// openStore reads the data file, or starts an empty record if there is no file
// yet. It does not create the file until something is saved.
func openStore(path string) (*store, error) {
	s := &store{
		path: path,
		data: fileData{
			Version:  fileFormatVersion,
			Settings: settings{Rates: till.DefaultRates()}.normalise(),
		},
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return s, nil
	}

	var loaded fileData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("reading %s: %w (move the file aside to start again)", path, err)
	}
	if loaded.Version > fileFormatVersion {
		return nil, fmt.Errorf("%s was written by a newer version of this tool (format %d, this build reads %d)",
			path, loaded.Version, fileFormatVersion)
	}
	loaded.Version = fileFormatVersion
	loaded.Settings = loaded.Settings.normalise()
	for i, sale := range loaded.Sales {
		loaded.Sales[i] = sale.Normalise()
	}
	till.SortSales(loaded.Sales)
	s.data = loaded
	return s, nil
}

// snapshot is a copy of the record that a reader can use without holding the
// lock.
func (s *store) snapshot() fileData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.data
	out.Sales = append([]till.Sale(nil), s.data.Sales...)
	return out
}

// change applies a change to the record and saves the file. If saving fails
// the change is rolled back, so what is in memory always matches what is on
// disk.
func (s *store) change(apply func(d *fileData) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	beforeSales := append([]till.Sale(nil), s.data.Sales...)
	beforeSettings := s.data.Settings

	if err := apply(&s.data); err != nil {
		s.data.Sales = beforeSales
		s.data.Settings = beforeSettings
		return err
	}
	till.SortSales(s.data.Sales)
	if err := s.save(); err != nil {
		s.data.Sales = beforeSales
		s.data.Settings = beforeSettings
		return err
	}
	return nil
}

// save writes the whole record to the data file atomically: a temporary file
// in the same directory, flushed to disk, then renamed over the old one. A
// crash therefore leaves either the old file or the new one, never half of
// either.
func (s *store) save() error {
	s.data.Version = fileFormatVersion

	body, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("preparing the record: %w", err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename has succeeded

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replacing %s: %w", s.path, err)
	}
	return nil
}

// newID makes a handle for an entry. It is this tool's own identifier and has
// no statutory meaning; the receipt number and the identification number are
// what tie an entry to a transaction.
func newID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// Only reached if the system's randomness is unavailable, in which
		// case a name that is merely unlikely to collide is better than
		// refusing to record the sale.
		return fmt.Sprintf("e%d", len(raw))
	}
	return hex.EncodeToString(raw[:])
}

// recordResult is what recording a bill did.
type recordResult struct {
	Saved   []till.Sale `json:"saved"`
	Skipped []till.Sale `json:"skipped"`
}

// record puts one entry per qualified buyer into the sales record.
//
// Every entry is validated before anything is saved, and a bill whose entries
// are already in the record is refused rather than doubled: a register with
// the same receipt twice overstates the deduction.
func (s *store) record(sales []till.Sale, today till.Date) (recordResult, error) {
	// Both lists start empty rather than absent, so the answer always carries
	// two lists and a reader never has to tell null from nothing.
	result := recordResult{Saved: []till.Sale{}, Skipped: []till.Sale{}}

	if len(sales) == 0 {
		return result, errors.New("there is no qualified buyer on this bill, so there is nothing to record")
	}

	prepared := make([]till.Sale, 0, len(sales))
	var problems []string
	for _, sale := range sales {
		sale = sale.Normalise()
		if err := sale.Validate(today); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		sale.ID = newID()
		prepared = append(prepared, sale)
	}
	if len(problems) > 0 {
		return result, errors.New(strings.Join(problems, "\n"))
	}

	err := s.change(func(d *fileData) error {
		have := make(map[string]bool, len(d.Sales))
		for _, existing := range d.Sales {
			have[existing.Key()] = true
		}
		for _, sale := range prepared {
			if have[sale.Key()] {
				result.Skipped = append(result.Skipped, sale)
				continue
			}
			have[sale.Key()] = true
			d.Sales = append(d.Sales, sale)
			result.Saved = append(result.Saved, sale)
		}
		if len(result.Saved) == 0 {
			return fmt.Errorf("this bill is already in the sales record under receipt %s", prepared[0].Reference)
		}
		return nil
	})
	if err != nil {
		return recordResult{Saved: []till.Sale{}, Skipped: []till.Sale{}}, err
	}
	return result, nil
}

// deleteSale removes one entry from the sales record.
func (s *store) deleteSale(id string) (till.Sale, error) {
	id = strings.TrimSpace(id)
	var removed till.Sale

	err := s.change(func(d *fileData) error {
		kept := make([]till.Sale, 0, len(d.Sales))
		found := false
		for _, sale := range d.Sales {
			if !found && sale.ID == id {
				removed = sale
				found = true
				continue
			}
			kept = append(kept, sale)
		}
		if !found {
			return fmt.Errorf("there is no entry %s in the sales record", id)
		}
		d.Sales = kept
		return nil
	})
	return removed, err
}

// putSettings replaces the establishment's details and the figures its sales
// are worked out on.
func (s *store) putSettings(next settings) (settings, error) {
	next = next.normalise()
	err := s.change(func(d *fileData) error {
		d.Settings = next
		return nil
	})
	return next, err
}
