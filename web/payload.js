// What a decoded string actually *is*.
//
// A raw payload is close to useless to the person holding the receipt. They do
// not think "code", they think "is this a link, an order number, a payment
// reference?". So every payload is run past a small table of recognisers and
// comes back as a kind, a label, and named fields worth reading.
//
// The table is data, not a chain of ifs: adding a format means adding a row.

(function (global) {
  "use strict";

  // Each entry: id, match(text) -> bool, parse(text) -> [{k, v}] of fields.
  // Order matters only in that the first match wins, so put the specific
  // formats before the general ones and leave `text` last.
  const KINDS = [
    {
      id: "wifi",
      // WIFI:T:WPA;S:MyNetwork;P:secret;H:false;;
      match: (t) => /^WIFI:/i.test(t),
      parse: (t) => {
        const f = wifiFields(t);
        return [
          { k: "network", v: f.S || f.s },
          { k: "security", v: (f.T || f.t || "").toUpperCase() || "none" },
          { k: "password", v: f.P || f.p, secret: true },
        ].filter((x) => x.v);
      },
      summary: (t) => wifiFields(t).S || wifiFields(t).s || "",
    },
    {
      id: "contact",
      match: (t) => /^BEGIN:VCARD/i.test(t) || /^MECARD:/i.test(t),
      parse: (t) => {
        if (/^MECARD:/i.test(t)) {
          const f = semiFields(t.replace(/^MECARD:/i, ""));
          return [
            { k: "name", v: flipName(f.N) },
            { k: "phone", v: f.TEL },
            { k: "email", v: f.EMAIL },
            { k: "address", v: f.ADR },
          ].filter((x) => x.v);
        }
        const line = (re) => (t.match(re) || [])[1];
        return [
          { k: "name", v: flipName(line(/^FN:(.+)$/mi) || line(/^N:(.+)$/mi)) },
          { k: "phone", v: line(/^TEL[^:]*:(.+)$/mi) },
          { k: "email", v: line(/^EMAIL[^:]*:(.+)$/mi) },
          { k: "company", v: line(/^ORG:(.+)$/mi) },
        ].filter((x) => x.v);
      },
      summary: (t) => {
        const m = t.match(/^FN:(.+)$/mi) || t.match(/^N:(.+)$/mi) || t.match(/N:([^;]+)/i);
        return m ? flipName(m[1]) : "";
      },
    },
    {
      id: "payment",
      // EPC069-12, the SEPA credit transfer QR used across the euro area.
      match: (t) => /^BCD\r?\n/.test(t),
      parse: (t) => {
        const L = t.split(/\r?\n/);
        return [
          { k: "payee", v: L[5] },
          { k: "iban", v: L[6] },
          { k: "amount", v: (L[7] || "").replace(/^EUR/i, "EUR ") },
          { k: "reference", v: L[9] || L[10] },
        ].filter((x) => x.v);
      },
      summary: (t) => (t.split(/\r?\n/)[5] || ""),
    },
    {
      id: "payment",
      match: (t) => /^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(t.replace(/\s+/g, "")),
      parse: (t) => [{ k: "iban", v: t.replace(/\s+/g, "") }],
      summary: (t) => t.replace(/\s+/g, ""),
    },
    {
      id: "otp",
      // otpauth://totp/Issuer:account?secret=BASE32&issuer=Issuer
      //
      // This one carries the two-factor SEED. Anyone who reads it can generate
      // that account's codes forever, so the field is marked secret and the
      // page keeps it covered until someone deliberately uncovers it. Showing
      // it by default would turn a scan in a shared room into a compromise.
      match: (t) => /^otpauth:\/\//i.test(t),
      parse: (t) => {
        let account = "", issuer = "", secret = "", algorithm = "";
        try {
          const u = new URL(t);
          const label = decodeURIComponent(u.pathname.replace(/^\//, ""));
          const cut = label.indexOf(":");
          account = cut >= 0 ? label.slice(cut + 1) : label;
          issuer = u.searchParams.get("issuer") || (cut >= 0 ? label.slice(0, cut) : "");
          secret = u.searchParams.get("secret") || "";
          algorithm = (u.searchParams.get("algorithm") || "").toUpperCase();
        } catch { /* leave the fields empty rather than guess */ }
        return [
          { k: "account", v: account },
          { k: "issuer", v: issuer },
          { k: "algorithm", v: algorithm },
          { k: "secret", v: secret, secret: true },
        ].filter((x) => x.v);
      },
      summary: (t) => {
        try {
          const u = new URL(t);
          return u.searchParams.get("issuer") ||
                 decodeURIComponent(u.pathname.replace(/^\//, "")).split(":")[0];
        } catch { return "otp"; }
      },
    },
    {
      id: "email",
      match: (t) => /^mailto:/i.test(t) || /^MATMSG:/i.test(t) ||
                    /^[^\s@:\/]+@[^\s@:\/]+\.[A-Za-z]{2,}$/.test(t),
      parse: (t) => {
        if (/^MATMSG:/i.test(t)) {
          const f = semiFields(t.replace(/^MATMSG:/i, ""));
          return [
            { k: "address", v: f.TO },
            { k: "subject", v: f.SUB },
            { k: "message", v: f.BODY },
          ].filter((x) => x.v);
        }
        const u = t.replace(/^mailto:/i, "").split("?");
        const q = new URLSearchParams(u[1] || "");
        return [
          { k: "address", v: decodeURIComponent(u[0]) },
          { k: "subject", v: q.get("subject") },
          { k: "message", v: q.get("body") },
        ].filter((x) => x.v);
      },
      summary: (t) => t.replace(/^mailto:/i, "").split("?")[0],
    },
    {
      id: "phone",
      // Deliberately narrow. A loose "digits and punctuation" test swallows
      // order numbers like 2026-0042, and calling a receipt reference a phone
      // number is worse than leaving it as a reference.
      match: (t) => /^tel:/i.test(t) ||
                    /^\+[\d\s.()-]{7,18}$/.test(t) ||
                    /^\d{9,15}$/.test(t),
      parse: (t) => [{ k: "number", v: t.replace(/^tel:/i, "").trim() }],
      summary: (t) => t.replace(/^tel:/i, "").trim(),
    },
    {
      id: "sms",
      match: (t) => /^(smsto|sms):/i.test(t),
      parse: (t) => {
        const rest = t.replace(/^(smsto|sms):/i, "").split(":");
        return [
          { k: "number", v: rest[0] },
          { k: "message", v: rest.slice(1).join(":") },
        ].filter((x) => x.v);
      },
      summary: (t) => t.replace(/^(smsto|sms):/i, "").split(":")[0],
    },
    {
      id: "location",
      match: (t) => /^geo:/i.test(t),
      parse: (t) => {
        const [lat, lon] = t.replace(/^geo:/i, "").split(",");
        return [{ k: "latitude", v: lat }, { k: "longitude", v: (lon || "").split("?")[0] }]
          .filter((x) => x.v);
      },
      summary: (t) => t.replace(/^geo:/i, ""),
    },
    {
      id: "event",
      match: (t) => /BEGIN:VEVENT/i.test(t),
      parse: (t) => {
        const line = (re) => (t.match(re) || [])[1];
        return [
          { k: "title", v: line(/^SUMMARY:(.+)$/mi) },
          { k: "starts", v: prettyStamp(line(/^DTSTART[^:]*:(.+)$/mi)) },
          { k: "ends", v: prettyStamp(line(/^DTEND[^:]*:(.+)$/mi)) },
          { k: "where", v: line(/^LOCATION:(.+)$/mi) },
        ].filter((x) => x.v);
      },
      summary: (t) => (t.match(/^SUMMARY:(.+)$/mi) || [])[1] || "",
    },
    {
      id: "crypto",
      match: (t) => /^(bitcoin|ethereum|litecoin|bitcoincash):/i.test(t),
      parse: (t) => {
        const [scheme, rest] = t.split(":");
        const [addr, query] = rest.split("?");
        const q = new URLSearchParams(query || "");
        return [
          { k: "network", v: scheme.toLowerCase() },
          { k: "address", v: addr },
          { k: "amount", v: q.get("amount") },
        ].filter((x) => x.v);
      },
      summary: (t) => t.split(":")[1].split("?")[0],
    },
    {
      id: "url",
      match: (t) => /^https?:\/\//i.test(t),
      parse: (t) => {
        try {
          const u = new URL(t);
          const ref = codeLikeParam(u);
          return [
            { k: "site", v: u.hostname.replace(/^www\./, "") },
            { k: "reference", v: ref },
            { k: "address", v: t, full: true },
          ].filter((x) => x.v);
        } catch { return [{ k: "address", v: t, full: true }]; }
      },
      summary: (t) => { try { return new URL(t).hostname.replace(/^www\./, ""); } catch { return t; } },
      href: (t) => t,
    },
    {
      id: "reference",
      // A bare number or a short alphanumeric token is an order or receipt
      // reference far more often than it is anything else.
      match: (t) => /^[A-Z0-9][A-Z0-9._\/-]{2,39}$/i.test(t) && /\d/.test(t),
      parse: (t) => [{ k: "reference", v: t }],
      summary: (t) => t,
    },
    {
      id: "text",
      match: () => true,
      parse: (t) => [{ k: "content", v: t, full: true }],
      summary: (t) => t,
    },
  ];

  function wifiFields(t) {
    return semiFields(t.replace(/^WIFI:/i, ""));
  }

  // `A:one;B:two;;`, the separator is escapable with a backslash, which a
  // naive split on ";" would trip over whenever a password contains one.
  function semiFields(body) {
    const out = {};
    let key = "", val = "", inKey = true, esc = false;
    const flush = () => { if (key) out[key.toUpperCase()] = val; key = ""; val = ""; inKey = true; };
    for (const ch of body) {
      if (esc) { val += ch; esc = false; continue; }
      if (ch === "\\") { esc = true; continue; }
      if (inKey && ch === ":") { inKey = false; continue; }
      if (ch === ";") { flush(); continue; }
      if (inKey) key += ch; else val += ch;
    }
    flush();
    return out;
  }

  function flipName(n) {
    if (!n) return "";
    // vCard and MECARD both carry "Surname;Given"; people read it the other way.
    const parts = n.split(";").map((s) => s.trim()).filter(Boolean);
    return parts.length > 1 ? parts[1] + " " + parts[0] : parts[0] || "";
  }

  function prettyStamp(s) {
    if (!s) return "";
    const m = s.match(/^(\d{4})(\d{2})(\d{2})(?:T(\d{2})(\d{2}))?/);
    return m ? `${m[3]}/${m[2]}/${m[1]}` + (m[4] ? ` ${m[4]}:${m[5]}` : "") : s;
  }

  // The part of a URL a human would quote over the phone. A query value that
  // looks like a reference beats the path, which is usually campaign wording.
  function codeLikeParam(u) {
    let best = "";
    for (const [, v] of u.searchParams) {
      if (v.length >= 5 && v.length <= 64 && /\d/.test(v) && /^[A-Z0-9._\/-]+$/i.test(v)) {
        if (v.length > best.length) best = v;
      }
    }
    return best;
  }

  // shortValue is what a filename should be built from: the meaningful part,
  // not the whole payload.
  function shortValue(text) {
    const k = classify(text);
    if (k.id === "url") {
      try {
        const u = new URL(text);
        const param = codeLikeParam(u);
        if (param) return param;
        const seg = u.pathname.split("/").filter(Boolean).pop();
        if (seg) return decodeURIComponent(seg);
        return u.hostname.replace(/^www\./, "");
      } catch { /* fall through */ }
    }
    const s = k.summary ? k.summary(text) : text;
    // Never let a secret become a filename: an otpauth seed in a file name
    // would leak through every folder listing and backup it ever touches.
    if (k.id === "otp") return (s || "otp").trim();
    return (s || text).trim();
  }

  function classify(text) {
    const t = String(text || "").trim();
    for (const k of KINDS) if (k.match(t)) return k;
    return KINDS[KINDS.length - 1];
  }

  function describe(text) {
    const t = String(text || "").trim();
    const kind = classify(t);
    return {
      kind: kind.id,
      raw: t,
      fields: kind.parse(t),
      short: shortValue(t),
      href: kind.href ? kind.href(t) : null,
    };
  }

  // The kinds this recogniser knows, for a page that wants to filter by them.
  const kinds = () => KINDS.map((k) => k.id);

  global.Payload = {
    kinds, describe, classify, shortValue, selfTest };

  // One runnable check for the part that is all edge cases. Run it with
  //   node web/payload.js --test
  function selfTest() {
    const eq = (got, want, what) => {
      if (got !== want) throw new Error(`${what}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
    };
    const kindOf = (t) => describe(t).kind;

    eq(kindOf("https://shop.it/x?code=AB12345"), "url", "a link is a link");
    eq(kindOf("WIFI:T:WPA;S:Bar;P:pw;;"), "wifi", "wifi");
    eq(kindOf("BEGIN:VCARD\nN:Rossi;Mario\nEND:VCARD"), "contact", "vcard");
    eq(kindOf("MECARD:N:Rossi,Mario;TEL:0551;;"), "contact", "mecard");
    eq(kindOf("BCD\n002\n1\nSCT\n\nBar Srl\nIT60X0542811101000000123456\nEUR12.50"), "payment", "sepa");
    eq(kindOf("IT60X0542811101000000123456"), "payment", "bare iban");
    eq(kindOf("mailto:a@b.it"), "email", "mailto");
    eq(kindOf("a@b.it"), "email", "bare address");
    eq(kindOf("tel:+390551234"), "phone", "tel scheme");
    eq(kindOf("+39 055 1234567"), "phone", "international number");
    eq(kindOf("bitcoin:1A1zP1?amount=0.1"), "crypto", "crypto");
    eq(kindOf("geo:43.77,11.25"), "location", "geo");
    eq(kindOf("BEGIN:VEVENT\nSUMMARY:Consegna\nDTSTART:20260901T0900\nEND:VEVENT"), "event", "event");

    // The one that used to be wrong: an order number is not a phone number.
    eq(kindOf("2026-0042"), "reference", "an order number stays a reference");
    eq(kindOf("ORD/2026/0042"), "reference", "a slashed reference");
    eq(kindOf("Buono sconto 10%"), "text", "prose falls through to text");

    // A two-factor seed must be recognised, marked secret, and kept out of any
    // filename built from the payload.
    const otp = describe("otpauth://totp/ACME:mario@acme.it?secret=JBSWY3DPEHPK3PXP&issuer=ACME");
    eq(otp.kind, "otp", "otpauth");
    eq(otp.fields.find((f) => f.k === "account").v, "mario@acme.it", "otp account");
    const seed = otp.fields.find((f) => f.k === "secret");
    eq(seed.v, "JBSWY3DPEHPK3PXP", "otp secret is parsed");
    eq(seed.secret, true, "otp secret is flagged as one");
    eq(otp.short, "ACME", "a filename never carries the seed");

    // A separator inside a value must survive; a naive split on ";" loses it.
    const wifi = describe("WIFI:T:WPA;S:Bar Centrale;P:ciao\\;123;;");
    eq(wifi.fields.find((f) => f.k === "password").v, "ciao;123", "escaped separator in a password");
    eq(wifi.fields.find((f) => f.k === "network").v, "Bar Centrale", "network name");

    // Names are stored surname-first and must be shown the way people say them.
    eq(describe("BEGIN:VCARD\nN:Rossi;Mario\nEND:VCARD").short, "Mario Rossi", "name order");

    // The filename-worthy part of a link is the reference, not the campaign path.
    eq(describe("https://www.mcdonalds.it/tua-opinione-conta?code=MJHFK-Y47NX").short,
       "MJHFK-Y47NX", "a link shortens to its reference");
    eq(describe("https://shop.it/ordini/12345").short, "12345", "no query, so the last path segment");

    return "payload self-test passed";
  }

  // Running the file directly is the test.
  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
