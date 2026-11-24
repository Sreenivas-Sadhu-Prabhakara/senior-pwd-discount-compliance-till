# Senior citizen and PWD discount till and sales register

A free, offline, single-binary tool for Philippine establishments that have to grant the
statutory senior citizen and person-with-disability discount: it works the discount out in
the order the regulations require, keeps the sales record the establishment must maintain
for those transactions, and totals the discounts granted over a period.

**Explainer site:** https://sreenivas-sadhu-prabhakara.github.io/senior-pwd-discount-compliance-till/

## What it does

A qualified senior citizen or person with disability buying a covered good or service is
exempt from the 12% value-added tax, and the 20% discount is taken on the amount **net of
that tax** — not on the posted price. On a VAT-inclusive bill of ₱1,120.00:

```
Bill as priced (VAT-inclusive)                 1,120.00
Less 12% VAT — the sale is VAT-exempt           (120.00)
VAT-exempt selling price                       1,000.00   <- the discount base
Less 20% statutory discount                     (200.00)
Amount due                                       800.00
```

Taking 20% off the ₱1,120.00 gives ₱896.00, and so does discounting the ₱1,000.00 and
adding the tax back afterwards. Both are wrong. The order is: exempt the sale from VAT
first, then discount the net amount.

On a shared bill the discount reaches only what the qualified buyer consumed. Where the
bill itemises that, the tool uses it; where it does not, the bill is divided by the number
of people sharing it and only the qualified buyers' shares are exempt and discounted. A
₱2,240.00 bill between four diners, one a senior citizen, comes to ₱2,080.00 — ₱400.00
from the senior citizen and ₱560.00 from each of the others.

The tool then:

- prints a plain-text charge slip showing every step of the working;
- puts one entry per qualified buyer into the sales record, with the card number;
- refuses an entry whose arithmetic does not hold together;
- prints the register for any month, quarter or year, on screen, as CSV or as plain text;
- totals the discounts granted for a period — the figure claimed as a deduction from
  gross income — split by law claimed under and broken down by month.

## Who it's for

Small Philippine establishments serving walk-in guests: restaurants and carinderias,
caterers, homestays and small hotels, tour operators, and anyone else whose goods or
services the two statutes cover. It suits a till with no point-of-sale system, or one
whose point-of-sale system gets the order of operations wrong.

## The rules it implements

- **RA 9994** (Expanded Senior Citizens Act of 2010), section 4(a) — 20% discount and VAT
  exemption on covered goods and services for a senior citizen. Implemented by its IRR and
  by **BIR Revenue Regulations No. 7-2010**.
- **RA 10754** (2016), amending section 32 of **RA 7277** (Magna Carta for Persons with
  Disability) — the same 20% and VAT exemption for a person with disability. Implemented
  by **BIR Revenue Regulations No. 5-2017**.
- **NIRC of 1997**, sections 106 and 108 as amended — the 12% value-added tax the
  exemption is from.
- **IRR of RA 9994, Rule IV Article 7** — group meals and group purchases are divided per
  head; a purchase already on promotional discount gives the buyer whichever discount is
  higher, not both.
- **RA 9994 section 4** with RR 7-2010 — the establishment claims the discounts granted as
  a **deduction from gross income** for the taxable year granted. It is not a tax credit.

Every rate and rule is a named constant in one file, `internal/till/statute.go`, with its
authority in the comment beside it. The VAT rate and the discount rate are overridable in
settings if the law changes, and each recorded entry keeps the rates it was worked out on,
so a change never restates a past month.

## Install and run

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/senior-pwd-discount-compliance-till.git
cd senior-pwd-discount-compliance-till

go run .                              # or: go build -o senior-pwd-till . && ./senior-pwd-till
```

Then open <http://127.0.0.1:8080>.

```sh
./senior-pwd-till -addr 127.0.0.1:9000 -data /path/to/sales-record.json
```

Go 1.25 to build; nothing to run. No dependencies — `go.mod` has no `require` block, and
the web interface is embedded in the binary.

## Where the data is stored

One JSON file beside the binary, `senior-pwd-discount-compliance-till-data.json` unless
you pass `-data`, holding the establishment's details, the rates in force and the sales
record. It is written atomically — a temporary file, flushed, then renamed — so an
interrupted save leaves the old file rather than half a new one.

The tool binds to `127.0.0.1`, so only your own machine can reach it, and it makes no
outbound network calls of any kind: no telemetry, no analytics, no external fonts or
scripts. Back the data file up the way you back up anything else that matters.

## Scope and accuracy

- **It does not decide what qualifies.** Which goods and services the discount and the
  exemption reach is set by RA 9994, RA 10754 and their implementing rules, each with its
  own conditions. You mark each line on the bill; the tool does the arithmetic that
  follows. It cannot tell you whether a card is genuine or current.
- **Not implemented:** the 5% discount on basic necessities and prime commodities under
  RA 9994 section 4(g), which has its own weekly purchase cap set by joint DTI and DA
  administrative order and no VAT exemption; the "most expensive meal combination" basis
  the implementing rules use for pre-contracted party packages and bundled meals, which
  the establishment sets itself; and the treatment of a service charge, which you mark
  covered or not yourself, on advice.
- **Not a point-of-sale system and not a return.** It does not issue or number official
  receipts, touch a cash drawer, file anything, or compute your VAT return, percentage tax
  or income tax. It produces the figures and the register.
- **Input tax** attributable to a VAT-exempt sale is not creditable; how it is closed to
  cost or expense is a matter for your accountant.
- **The figures are indicative.** The 12% rate, the 20% discount, the per-head split and
  the deduction treatment are those in force when this tool was written, and are stated in
  the settings screen with their authority. Confirm the current ones — and above all which
  goods and services qualify — with the Bureau of Internal Revenue, your city or municipal
  Office for Senior Citizens Affairs, and the National Council on Disability Affairs. The
  statutes and their implementing rules are the authority; this tool is not, and nothing
  here is legal or tax advice.

Rounding: amounts are held in whole centavos and a half centavo rounds away from zero. The
regulations do not prescribe a rounding rule, so this is ordinary commercial rounding,
applied once per step.

## Layout

```
main.go                     flags, the embedded interface, the local server
store.go                    the data file: atomic save, load, settings
api.go                      the JSON interface the page talks to
internal/till/statute.go    every statutory figure and rule, with its authority
internal/till/money.go      centavo arithmetic: VAT out, percentages, splitting a bill
internal/till/compute.go    the order of operations
internal/till/sale.go       one entry in the statutory sales record, and its validation
internal/till/register.go   periods, totals, the register's columns and rows
internal/till/slip.go       the charge slip, the printable register, the summary
web/                        the interface, embedded in the binary
docs/                       the explainer site
```

The rules live in `internal/till` as pure functions with table-driven tests, including the
worked examples above, a shared bill where only some diners qualify, the rounding, and the
zero and one-centavo cases.

```sh
go test ./... -count=1
```

MIT licensed.
