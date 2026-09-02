// Reading a CSV of payloads, to turn a list into a sheet of QR codes.
//
// The inverse of what the rest of the page does: instead of finding codes in
// pictures, this makes pictures from codes. It lands in the same place -
// "recreate the codes" already knows how to write PNG, JPEG, SVG and PDF - so
// a CSV adds a source, not a second feature with its own buttons.
//
// The parser is RFC 4180 as far as it goes: quoted fields, doubled quotes
// inside them, commas and newlines inside quotes, CRLF or LF. Semicolons are
// accepted as a separator too, because a spreadsheet saved anywhere with a
// comma decimal separator produces them and the user did not choose that.

(function (global) {
  "use strict";

  // Header names that mean "this is the thing to encode", lowercase. Not a
  // guess about the user's spreadsheet: if none of these appear, the first
  // column is used and that is stated in the UI.
  const VALUE_HEADERS = ["text", "content", "value", "payload", "code", "data", "url", "link"];
  const NAME_HEADERS = ["name", "file", "filename", "label", "title", "id"];

  function sniffSeparator(s) {
    const line = s.split(/\r?\n/, 1)[0] || "";
    // Count outside quotes, so a comma inside "Smith, John" does not vote.
    let inQ = false, comma = 0, semi = 0, tab = 0;
    for (let i = 0; i < line.length; i++) {
      const c = line[i];
      if (c === '"') inQ = !inQ;
      else if (!inQ && c === ",") comma++;
      else if (!inQ && c === ";") semi++;
      else if (!inQ && c === "\t") tab++;
    }
    if (tab > comma && tab > semi) return "\t";
    return semi > comma ? ";" : ",";
  }

  function parse(text, sep) {
    const s = String(text == null ? "" : text).replace(/^﻿/, "");
    const d = sep || sniffSeparator(s);
    const rows = [];
    let row = [], field = "", inQ = false;
    for (let i = 0; i < s.length; i++) {
      const c = s[i];
      if (inQ) {
        if (c === '"') {
          if (s[i + 1] === '"') { field += '"'; i++; }
          else inQ = false;
        } else field += c;
        continue;
      }
      if (c === '"') { inQ = true; continue; }
      if (c === d) { row.push(field); field = ""; continue; }
      if (c === "\n" || c === "\r") {
        if (c === "\r" && s[i + 1] === "\n") i++;
        row.push(field); field = "";
        if (row.length > 1 || row[0] !== "") rows.push(row);
        row = [];
        continue;
      }
      field += c;
    }
    row.push(field);
    if (row.length > 1 || row[0] !== "") rows.push(row);
    return rows;
  }

  // read turns a CSV into the payloads to encode, and a name for each.
  //
  // A file with a recognisable header uses the named columns. Anything else
  // uses the first column, and the second as a name when there is one. The
  // result says which it did, because a silent guess about somebody's
  // spreadsheet is how the wrong column ends up on a hundred stickers.
  function read(text) {
    const rows = parse(text);
    if (!rows.length) return { items: [], usedHeader: false, valueColumn: null, skipped: 0 };

    const head = rows[0].map((h) => h.trim().toLowerCase());
    let vi = head.findIndex((h) => VALUE_HEADERS.includes(h));
    let ni = head.findIndex((h) => NAME_HEADERS.includes(h));
    const usedHeader = vi >= 0;
    let body = rows;
    if (usedHeader) {
      body = rows.slice(1);
    } else {
      vi = 0;
      ni = rows[0].length > 1 ? 1 : -1;
      // A first row that is plainly labels rather than data: every cell is a
      // known header word. Dropping it beats encoding the word "url".
      if (head.every((h) => VALUE_HEADERS.includes(h) || NAME_HEADERS.includes(h)) && head.length) {
        body = rows.slice(1);
      }
    }

    const items = [];
    let skipped = 0;
    for (const r of body) {
      const value = (r[vi] || "").trim();
      if (!value) { skipped++; continue; }
      const name = ni >= 0 ? (r[ni] || "").trim() : "";
      items.push({ value, name });
    }
    return {
      items,
      usedHeader,
      valueColumn: usedHeader ? head[vi] : String(vi + 1),
      skipped,
    };
  }

  const looksLikeCSV = (file) =>
    !!file && /\.(csv|tsv|txt)$/i.test(file.name || "");

  function selfTest() {
    const eq = (a, b, what) => { if (JSON.stringify(a) !== JSON.stringify(b)) throw new Error(what + ": " + JSON.stringify(a) + " vs " + JSON.stringify(b)); };

    // Quotes, embedded separators and newlines survive.
    eq(parse('a,b\n"x,y",z'), [["a", "b"], ["x,y", "z"]], "quoted comma");
    eq(parse('"he said ""hi""",2'), [['he said "hi"', "2"]], "doubled quote");
    eq(parse('a,b\r\nc,d\r\n'), [["a", "b"], ["c", "d"]], "crlf");
    eq(parse('"line\nbreak",2'), [["line\nbreak", "2"]], "newline inside quotes");

    // A semicolon file is a spreadsheet's doing, not the user's.
    if (sniffSeparator("a;b;c") !== ";") throw new Error("semicolon not sniffed");
    if (sniffSeparator('"a,b";c') !== ";") throw new Error("a comma inside quotes voted for comma");
    if (sniffSeparator("a\tb\tc") !== "\t") throw new Error("tab not sniffed");
    eq(parse("a;b\nc;d"), [["a", "b"], ["c", "d"]], "semicolon rows");

    // Header recognised: the named column wins whatever position it is in.
    const h = read("name,url\nPoster,https://example.com/a\nFlyer,https://example.com/b");
    if (!h.usedHeader || h.valueColumn !== "url") throw new Error("header column not used: " + JSON.stringify(h));
    eq(h.items, [{ value: "https://example.com/a", name: "Poster" },
                 { value: "https://example.com/b", name: "Flyer" }], "header rows");

    // No header: first column, second as a name.
    const n = read("https://example.com/a,Poster\nhttps://example.com/b,Flyer");
    if (n.usedHeader) throw new Error("invented a header");
    eq(n.items, [{ value: "https://example.com/a", name: "Poster" },
                 { value: "https://example.com/b", name: "Flyer" }], "headerless rows");

    // A single column is the whole file.
    eq(read("one\ntwo\nthree").items.map((i) => i.value), ["one", "two", "three"], "single column");

    // Empty rows are counted, not encoded: a blank QR is not a QR.
    const sk = read("a\n\n \nb");
    eq(sk.items.map((i) => i.value), ["a", "b"], "blank rows dropped");
    if (sk.skipped !== 1) throw new Error("blank rows not counted: " + sk.skipped);

    if (read("").items.length) throw new Error("an empty file produced items");
    if (!looksLikeCSV({ name: "list.CSV" }) || looksLikeCSV({ name: "photo.png" })) {
      throw new Error("looksLikeCSV is wrong");
    }
    return "csvin self-test passed (quotes, separators, header columns, blanks)";
  }

  global.CSVIn = { parse, read, sniffSeparator, looksLikeCSV, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
