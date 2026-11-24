/* Senior citizen and PWD discount till — the page's own behaviour.

   Every figure comes from the tool running on this machine; nothing else is
   contacted. The page builds its tables as text nodes, never as markup, so a
   buyer's name can only ever be read as a name. */
(function () {
  "use strict";

  var pesoFmt = new Intl.NumberFormat("en-PH", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

  var state = null;
  var lastQuote = null;

  /* ---------- small helpers ---------- */

  function $(id) { return document.getElementById(id); }

  function setText(node, s) { if (node) { node.textContent = s; } }

  function show(node, on) {
    if (!node) { return; }
    if (on) { node.removeAttribute("hidden"); } else { node.setAttribute("hidden", ""); }
  }

  function clear(node) {
    while (node && node.firstChild) { node.removeChild(node.firstChild); }
  }

  function peso(value) {
    var n = Number(value);
    if (!isFinite(n)) { n = 0; }
    return "₱" + pesoFmt.format(n);
  }

  function el(tag, cls, text) {
    var node = document.createElement(tag);
    if (cls) { node.className = cls; }
    if (text !== undefined && text !== null) { node.appendChild(document.createTextNode(String(text))); }
    return node;
  }

  function td(text, cls) { return el("td", cls, text); }

  function row(cells) {
    var tr = document.createElement("tr");
    cells.forEach(function (cell) { tr.appendChild(cell); });
    return tr;
  }

  function banner(message, kind) {
    var node = $("banner");
    clear(node);
    if (!message) { show(node, false); return; }
    node.appendChild(document.createTextNode(message));
    node.className = kind === "ok" ? "note ok" : "note";
    show(node, true);
  }

  function problems(payload, fallback) {
    if (payload && payload.problems && payload.problems.length) { return payload.problems.join(" · "); }
    if (payload && payload.error) { return payload.error; }
    return fallback;
  }

  function send(method, path, body) {
    var options = { method: method, headers: { "Accept": "application/json" } };
    if (body !== undefined) {
      options.headers["Content-Type"] = "application/json";
      options.body = JSON.stringify(body);
    }
    return fetch(path, options).then(function (response) {
      return response.text().then(function (text) {
        var payload = null;
        if (text) { try { payload = JSON.parse(text); } catch (e) { payload = null; } }
        if (!response.ok) {
          throw new Error(problems(payload, "the tool answered " + response.status));
        }
        return payload;
      });
    });
  }

  /* ---------- the bill form ---------- */

  function lineRows() { return Array.prototype.slice.call($("lines").querySelectorAll(".line")); }
  function buyerRows() { return Array.prototype.slice.call($("buyers").querySelectorAll(".buyer")); }

  function addLine() {
    var node = $("tpl-line").content.firstElementChild.cloneNode(true);
    node.querySelector(".drop").addEventListener("click", function () {
      node.parentNode.removeChild(node);
      if (lineRows().length === 0) { addLine(); }
    });
    $("lines").appendChild(node);
    return node;
  }

  function addBuyer() {
    var node = $("tpl-buyer").content.firstElementChild.cloneNode(true);
    node.querySelector(".drop").addEventListener("click", function () {
      node.parentNode.removeChild(node);
    });
    $("buyers").appendChild(node);
    paintAllocation();
    return node;
  }

  function paintAllocation() {
    var perHead = $("b-alloc").value === "per_head";
    buyerRows().forEach(function (node) {
      if (perHead) {
        node.classList.add("perhead");
        node.querySelector(".y-share").value = "";
      } else {
        node.classList.remove("perhead");
      }
    });
  }

  function amount(value) {
    var text = String(value === undefined || value === null ? "" : value).trim();
    return text === "" ? "0" : text;
  }

  function readBill() {
    var lines = lineRows().map(function (node) {
      var kind = node.querySelector(".l-kind").value;
      return {
        description: node.querySelector(".l-desc").value.trim(),
        amount: amount(node.querySelector(".l-amount").value),
        covered: kind === "covered" || kind === "covered_novat",
        vatExempt: kind === "covered_novat"
      };
    });

    var buyers = buyerRows().map(function (node) {
      return {
        name: node.querySelector(".y-name").value.trim(),
        basis: node.querySelector(".y-basis").value,
        idNo: node.querySelector(".y-id").value.trim(),
        booklet: node.querySelector(".y-booklet").value.trim(),
        share: amount(node.querySelector(".y-share").value)
      };
    });

    return {
      date: $("b-date").value,
      reference: $("b-ref").value.trim(),
      lines: lines,
      persons: Number($("b-persons").value) || 0,
      buyers: buyers,
      allocation: $("b-alloc").value,
      promotion: amount($("b-promo").value),
      note: $("b-note").value.trim()
    };
  }

  function clearBill() {
    $("b-ref").value = "";
    $("b-promo").value = "";
    $("b-note").value = "";
    $("b-persons").value = "1";
    $("b-alloc").value = "per_head";
    clear($("lines"));
    clear($("buyers"));
    addLine();
    addBuyer();
    lastQuote = null;
    renderQuote(null);
    banner("");
  }

  /* ---------- the worked-out bill ---------- */

  function renderQuote(payload) {
    lastQuote = payload;
    var steps = $("r-steps");
    var buyers = $("r-buyers");
    var notes = $("r-notes");
    clear(steps);
    clear(buyers);
    clear(notes);
    show($("r-sliptext"), false);

    if (!payload) {
      setText($("r-total"), "—");
      setText($("r-sub"), "Enter the bill and work it out.");
      return;
    }

    var computed = payload.computation;
    setText($("r-total"), peso(computed.totalDue));

    var sub = computed.qualified === 0
      ? "No qualified buyer on this bill."
      : computed.qualified + (computed.qualified === 1 ? " qualified buyer saved " : " qualified buyers saved ") + peso(computed.saved)
        + " (" + peso(computed.vatRemoved) + " VAT not charged, " + peso(computed.discount) + " discount)";
    setText($("r-sub"), sub);

    computed.steps.forEach(function (step) {
      var label = td("");
      label.appendChild(document.createTextNode(step.label));
      if (step.authority) { label.appendChild(el("span", "why", step.authority)); }

      var value = step.deduction && step.amount !== "0.00"
        ? td("(" + peso(step.amount) + ")", "num neg")
        : td(peso(step.amount), "num");

      var tr = row([label, value]);
      if (step.emphasis) { tr.className = "key"; }
      steps.appendChild(tr);
    });

    (computed.shares || []).forEach(function (share) {
      var card = el("div", "buyercard");
      card.appendChild(el("h3", null, share.buyer.name + " — " + basisLabel(share.buyer.basis)));
      var meta = share.buyer.idNo;
      if (share.buyer.booklet) { meta += " · booklet " + share.buyer.booklet; }
      card.appendChild(el("div", "meta", meta));

      var table = el("table", "breakdown");
      var body = document.createElement("tbody");
      [
        ["Share as priced", share.gross, false],
        ["VAT not charged", share.vatRemoved, true],
        ["VAT-exempt selling price", share.discountBase, false],
        ["Discount granted", share.discount, true],
        ["Amount due", share.due, false]
      ].forEach(function (line) {
        var value = line[2] && line[1] !== "0.00"
          ? td("(" + peso(line[1]) + ")", "num neg")
          : td(peso(line[1]), "num");
        body.appendChild(row([td(line[0]), value]));
      });
      table.appendChild(body);
      card.appendChild(table);
      buyers.appendChild(card);
    });

    var all = (computed.notes || []).slice();
    if (computed.promotion) { all.unshift(computed.promotion.advice); }
    if (all.length) {
      var box = el("div", "note");
      var list = document.createElement("ul");
      all.forEach(function (text) { list.appendChild(el("li", null, text)); });
      box.appendChild(list);
      notes.appendChild(box);
    }
  }

  function basisLabel(basis) {
    if (basis === "senior") { return "Senior citizen (RA 9994)"; }
    if (basis === "pwd") { return "Person with disability (RA 10754)"; }
    return basis;
  }

  function quote() {
    banner("");
    return send("POST", "/api/quote", readBill()).then(function (payload) {
      renderQuote(payload);
    }).catch(function (err) {
      renderQuote(null);
      banner(err.message);
    });
  }

  function record() {
    banner("");
    return send("POST", "/api/sales", readBill()).then(function (payload) {
      renderQuote({ computation: payload.computation, slip: payload.slip });
      var saved = (payload.saved || []).length;
      var skipped = (payload.skipped || []).length;
      var message = saved + (saved === 1 ? " entry" : " entries") + " added to the sales record.";
      if (skipped) { message += " " + skipped + " was already there and was left alone."; }
      banner(message, "ok");
      return Promise.all([loadState(), loadRegister(), loadSummary()]);
    }).catch(function (err) {
      banner(err.message);
    });
  }

  function showSlip() {
    var box = $("r-sliptext");
    if (!lastQuote || !lastQuote.slip) {
      banner("Work the bill out first, then the charge slip has something to show.");
      return;
    }
    if (box.hasAttribute("hidden")) {
      setText(box, lastQuote.slip);
      show(box, true);
    } else {
      show(box, false);
    }
  }

  /* ---------- the register and the period summary ---------- */

  function periodQuery(select) {
    var chosen = select.options[select.selectedIndex];
    if (!chosen) { return ""; }
    return "kind=" + encodeURIComponent(chosen.getAttribute("data-kind"))
      + "&year=" + encodeURIComponent(chosen.getAttribute("data-year"))
      + "&index=" + encodeURIComponent(chosen.getAttribute("data-index"));
  }

  function fillPeriods(select, periods, preferredKind) {
    clear(select);
    periods.forEach(function (period) {
      var option = document.createElement("option");
      option.value = period.kind + ":" + period.year + ":" + period.index;
      option.setAttribute("data-kind", period.kind);
      option.setAttribute("data-year", period.year);
      option.setAttribute("data-index", period.index);
      option.appendChild(document.createTextNode(period.label));
      select.appendChild(option);
    });
    for (var i = 0; i < select.options.length; i++) {
      if (select.options[i].getAttribute("data-kind") === preferredKind) {
        select.selectedIndex = i;
        break;
      }
    }
  }

  function loadRegister() {
    var query = periodQuery($("p-period"));
    $("p-csv").setAttribute("href", "/api/register.csv?" + query);
    $("p-txt").setAttribute("href", "/api/register.txt?" + query);

    return send("GET", "/api/summary?" + query).then(function (payload) {
      var head = $("reg-head");
      clear(head);
      var where = state && state.settings ? state.settings.establishment : null;
      var heading = where && where.name ? where.name : "Senior citizen and PWD sales record";
      head.appendChild(document.createTextNode(heading + " — " + payload.summary.period.label));

      var cols = $("reg-cols");
      clear(cols);
      payload.columns.forEach(function (name, i) {
        cols.appendChild(el("th", i >= 6 && i <= 10 ? "num" : null, name));
      });

      var body = $("reg-rows");
      clear(body);
      payload.rows.forEach(function (cells) {
        body.appendChild(row(cells.map(function (cell, i) {
          return td(i >= 6 && i <= 10 && cell !== "" ? peso(cell) : cell, i >= 6 && i <= 10 ? "num" : null);
        })));
      });

      var totals = $("reg-totals");
      clear(totals);
      payload.totalsRow.forEach(function (cell, i) {
        totals.appendChild(el("td", i >= 6 && i <= 10 ? "num" : null, i >= 6 && i <= 10 && cell !== "" ? peso(cell) : cell));
      });
      totals.className = "total";

      show($("reg-empty"), payload.rows.length === 0);
    }).catch(function (err) {
      banner(err.message);
    });
  }

  function loadSummary() {
    var query = periodQuery($("s-period"));
    $("s-txt").setAttribute("href", "/api/summary.txt?" + query);

    return send("GET", "/api/summary?" + query).then(function (payload) {
      var summary = payload.summary;
      var total = summary.total;

      setText($("s-deduction"), peso(summary.total.discount));
      setText($("s-sub"), summary.period.label + " · " + total.entries
        + (total.entries === 1 ? " entry" : " entries") + " for " + total.buyers
        + (total.buyers === 1 ? " buyer" : " buyers"));

      var totals = $("s-totals");
      clear(totals);
      [
        ["Gross sales, VAT-inclusive", total.gross],
        ["VAT not charged (exempt sales)", total.vatForgone],
        ["VAT-exempt sales — the amount the discounts were taken on", total.vatExempt],
        ["Discounts granted", total.discount],
        ["Net sales collected", total.net]
      ].forEach(function (line) {
        totals.appendChild(row([td(line[0]), td(peso(line[1]), "num")]));
      });

      var split = $("s-split");
      clear(split);
      [
        ["Entries", String(summary.senior.entries), String(summary.pwd.entries)],
        ["VAT-exempt sales", peso(summary.senior.vatExempt), peso(summary.pwd.vatExempt)],
        ["Discounts granted", peso(summary.senior.discount), peso(summary.pwd.discount)],
        ["Net sales", peso(summary.senior.net), peso(summary.pwd.net)]
      ].forEach(function (line) {
        split.appendChild(row([td(line[0]), td(line[1], "num"), td(line[2], "num")]));
      });

      var months = $("s-months");
      clear(months);
      (summary.months || []).forEach(function (month) {
        months.appendChild(row([
          td(month.label),
          td(String(month.tally.entries), "num"),
          td(peso(month.tally.vatExempt), "num"),
          td(peso(month.tally.discount), "num")
        ]));
      });

      var notes = $("s-notes");
      clear(notes);
      if ((summary.notes || []).length) {
        var box = el("div", "note");
        var list = document.createElement("ul");
        summary.notes.forEach(function (text) { list.appendChild(el("li", null, text)); });
        box.appendChild(list);
        notes.appendChild(box);
      }
    }).catch(function (err) {
      banner(err.message);
    });
  }

  /* ---------- settings ---------- */

  function paintSettings() {
    var where = state.settings.establishment;
    var rates = state.settings.rates;

    $("e-name").value = where.name || "";
    $("e-branch").value = where.branch || "";
    $("e-address").value = where.address || "";
    $("e-tin").value = where.tin || "";
    $("e-vatreg").value = rates.vatRegistered ? "yes" : "no";
    $("e-vat").value = rates.vatPercent;
    $("e-disc").value = rates.discountPercent;
    setText($("e-datafile"), state.dataFile);
    setText($("e-entries"), state.entries + (state.entries === 1 ? " entry" : " entries"));

    var table = $("e-statutory");
    clear(table);
    var figures = state.statutory;
    [
      ["Value-added tax rate", figures.vatRatePercent + "%", figures.vatStatute],
      ["Statutory discount", figures.discountPercent + "%", figures.seniorStatute + "; " + figures.pwdStatute],
      ["Order of operations", figures.discountOnVatExclusive ? "VAT out first, discount on the net amount" : "—", "BIR RR 7-2010 s. 6; RR 5-2017"],
      ["Shared bill", figures.sharesPerPersonInGroup + " share per person", "IRR of RA 9994, Rule IV Art. 7"],
      ["Discounts per buyer", String(figures.discountsPerQualifiedBuyer), "a buyer qualifying under both laws claims one discount"],
      ["Promotional discount", figures.promotionIsAlternative ? "An alternative, not an addition" : "—", "IRR of RA 9994, Rule IV Art. 7"],
      ["Discounts granted", figures.discountIsADeduction ? "A deduction from gross income" : "—", "RA 9994 s. 4; not a tax credit"]
    ].forEach(function (line) {
      var label = td("");
      label.appendChild(document.createTextNode(line[0]));
      label.appendChild(el("span", "why", line[2]));
      table.appendChild(row([label, td(line[1], "num")]));
    });
  }

  function saveSettings(event) {
    event.preventDefault();
    banner("");

    var next = {
      establishment: {
        name: $("e-name").value.trim(),
        branch: $("e-branch").value.trim(),
        address: $("e-address").value.trim(),
        tin: $("e-tin").value.trim()
      },
      rates: {
        vatPercent: Number($("e-vat").value),
        discountPercent: Number($("e-disc").value),
        vatRegistered: $("e-vatreg").value === "yes"
      }
    };

    send("PUT", "/api/settings", next).then(function () {
      banner("Settings saved.", "ok");
      return loadState();
    }).catch(function (err) {
      banner(err.message);
    });
  }

  /* ---------- loading and wiring ---------- */

  function loadState() {
    return send("GET", "/api/state").then(function (payload) {
      var first = state === null;
      state = payload;
      paintSettings();

      if (first) {
        if (!$("b-date").value) { $("b-date").value = state.today; }
        fillPeriods($("p-period"), state.periods, "month");
        fillPeriods($("s-period"), state.periods, "month");
      }
    });
  }

  function openPanel(name) {
    var buttons = $("tabs").querySelectorAll("button[data-panel]");
    for (var i = 0; i < buttons.length; i++) {
      var panel = buttons[i].getAttribute("data-panel");
      buttons[i].setAttribute("aria-pressed", panel === name ? "true" : "false");
      show($("panel-" + panel), panel === name);
    }
    if (name === "record") { loadRegister(); }
    if (name === "period") { loadSummary(); }
  }

  function wireTheme() {
    var root = document.documentElement;
    var button = document.querySelector("[data-themer]");
    if (!button) { return; }

    function stored() {
      try { return localStorage.getItem("omsme-theme"); } catch (e) { return null; }
    }
    function current() {
      var set = root.getAttribute("data-theme");
      if (set) { return set; }
      return (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
    }
    function paint() {
      var dark = current() === "dark";
      button.textContent = dark ? "☀ Light" : "☾ Dark";
      button.setAttribute("aria-label", dark ? "Switch to light theme" : "Switch to dark theme");
    }
    paint();
    button.addEventListener("click", function () {
      var next = current() === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("omsme-theme", next); } catch (e) { /* private browsing */ }
      paint();
    });
    if (window.matchMedia) {
      window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
        if (!stored()) { paint(); }
      });
    }
  }

  function wire() {
    wireTheme();

    $("tabs").addEventListener("click", function (event) {
      var panel = event.target.getAttribute && event.target.getAttribute("data-panel");
      if (panel) { openPanel(panel); }
    });

    $("add-line").addEventListener("click", function () { addLine(); });
    $("add-buyer").addEventListener("click", function () { addBuyer(); });
    $("b-alloc").addEventListener("change", paintAllocation);
    $("b-quote").addEventListener("click", quote);
    $("b-record").addEventListener("click", record);
    $("b-clear").addEventListener("click", clearBill);
    $("r-slip").addEventListener("click", showSlip);

    $("p-period").addEventListener("change", loadRegister);
    $("p-print").addEventListener("click", function () { window.print(); });
    $("s-period").addEventListener("change", loadSummary);

    $("form-settings").addEventListener("submit", saveSettings);
  }

  wire();
  addLine();
  addBuyer();
  loadState().then(function () {
    return loadRegister();
  }).catch(function (err) {
    banner(err.message);
  });
})();
