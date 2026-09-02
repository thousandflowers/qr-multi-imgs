// Choosing a language and applying it. The words themselves live in
// strings.js, so this file does not grow when a language is added.

(function (global) {
  "use strict";

  // Under node the strings are not loaded by a script tag.
  if (!global.QR_STRINGS && typeof require === "function") require("./strings.js");

  const STRINGS = global.QR_STRINGS || {};
  const NAMES = global.QR_LANG_NAMES || {};
  const STORE = "qmi.lang";
  const listeners = [];

  function pick() {
    try {
      const saved = localStorage.getItem(STORE);
      if (saved && STRINGS[saved]) return saved;
    } catch { /* private mode; the browser preference still decides */ }
    if (typeof navigator === "undefined") return "en";
    // navigator.languages is in the user's own order of preference, so the
    // first one the page speaks is the right answer.
    for (const tag of navigator.languages || [navigator.language || "en"]) {
      const base = String(tag).toLowerCase().split("-")[0];
      if (STRINGS[base]) return base;
    }
    // Nothing the browser is set to is a language this page speaks. Before
    // falling back to English, ask where the machine thinks it is.
    //
    // WHY THE TIME ZONE AND NOT THE ACTUAL LOCATION. Real geolocation costs
    // either a permission prompt or a request to somebody else's server to
    // turn an IP into a place. The second is out of the question: this page's
    // one promise is that nothing leaves the device, and it is enforced by a
    // CSP that allows no third party at all. The first asks the user to grant
    // a sensitive permission to pick a menu language.
    //
    // The time zone is already on the machine, needs no permission, sends
    // nothing, and is set from where the computer was configured - which is
    // the same question, answered offline.
    return fromTimeZone() || "en";
  }

  // Zones for the languages this page speaks. A map rather than a chain of
  // conditions, and deliberately not a world atlas: a zone that is not here
  // simply falls through to English, which is what it did before.
  const ZONE_LANG = {
    "Europe/Rome": "it", "Europe/Vatican": "it", "Europe/San_Marino": "it",
    "Europe/Madrid": "es", "Atlantic/Canary": "es", "Africa/Ceuta": "es",
    "America/Mexico_City": "es", "America/Bogota": "es", "America/Lima": "es",
    "America/Argentina/Buenos_Aires": "es", "America/Santiago": "es",
    "Europe/Paris": "fr", "Europe/Brussels": "fr", "Europe/Monaco": "fr",
    "Europe/Berlin": "de", "Europe/Vienna": "de", "Europe/Zurich": "de",
    "Europe/Lisbon": "pt", "Atlantic/Azores": "pt", "Atlantic/Madeira": "pt",
    "America/Sao_Paulo": "pt", "America/Bahia": "pt", "America/Fortaleza": "pt",
  };

  function fromTimeZone() {
    try {
      const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
      const lang = ZONE_LANG[zone];
      return lang && STRINGS[lang] ? lang : null;
    } catch {
      return null;
    }
  }

  let lang = pick();

  // A missing key falls back to English rather than rendering the key itself:
  // an untranslated sentence is a small blemish, a raw key is a broken page.
  function t(key, vars) {
    let s = (STRINGS[lang] && STRINGS[lang][key]) || STRINGS.en[key] || key;
    if (vars) for (const k in vars) s = s.split("{" + k + "}").join(vars[k]);
    return s;
  }

  function setLang(next) {
    if (!STRINGS[next] || next === lang) return;
    lang = next;
    try { localStorage.setItem(STORE, next); } catch { /* nothing to do */ }
    apply();
    for (const cb of listeners) cb(next);
  }

  // Static text is marked in the markup, so translating it is a walk rather
  // than a list of element ids kept in step by hand.
  function apply() {
    document.documentElement.lang = lang;
    for (const el of document.querySelectorAll("[data-t]")) el.textContent = t(el.dataset.t);
    for (const el of document.querySelectorAll("[data-t-html]")) el.innerHTML = t(el.dataset.tHtml);
    for (const el of document.querySelectorAll("[data-t-ph]")) el.placeholder = t(el.dataset.tPh);
    for (const el of document.querySelectorAll("[data-t-title]")) el.title = t(el.dataset.tTitle);
    // An icon-only control has no text to read, so its name has to come from
    // somewhere. This is that somewhere, and it translates like everything else.
    for (const el of document.querySelectorAll("[data-t-aria]")) el.setAttribute("aria-label", t(el.dataset.tAria));
  }

  const languages = () => Object.keys(STRINGS).map((code) => ({ code, name: NAMES[code] || code }));
  const onChange = (cb) => listeners.push(cb);

  // keysUsedIn pulls every translation key a page actually asks for out of its
  // markup. Parity between languages is not the only way a string goes missing:
  // a key can be absent from ALL of them at once, and then every language
  // agrees perfectly while the page renders "insp.keys" at the user. That
  // happened. This is the check that catches it.
  function keysUsedIn(html) {
    const out = new Set();
    const re = /data-t(?:-html|-ph|-title|-aria)?\s*=\s*"([^"]+)"/g;
    let m;
    while ((m = re.exec(html))) out.add(m[1]);
    return [...out];
  }

  function selfTest() {
    const codes = Object.keys(STRINGS);
    if (codes.length < 2) throw new Error("only one language is defined");

    // Every language must carry every key, in both directions: a key added to
    // one language and forgotten in the rest is the failure this catches.
    const gaps = [];
    for (const code of codes) {
      if (!NAMES[code]) gaps.push(code + " has no display name");
      for (const k in STRINGS.en) if (!(k in STRINGS[code])) gaps.push(code + " is missing " + k);
      for (const k in STRINGS[code]) if (!(k in STRINGS.en)) gaps.push(k + " exists only in " + code);
    }
    if (gaps.length) throw new Error(gaps.slice(0, 8).join("; ") + (gaps.length > 8 ? " …" : ""));

    // A placeholder dropped in translation leaves a sentence with a hole in it.
    for (const code of codes) {
      for (const k in STRINGS.en) {
        const holes = (STRINGS.en[k].match(/\{\w+\}/g) || []).sort().join();
        const theirs = (STRINGS[code][k].match(/\{\w+\}/g) || []).sort().join();
        if (holes !== theirs) throw new Error(code + "/" + k + ": placeholders differ");
      }
    }

    if (t("dupe.badge", { n: 3 }).indexOf("3") < 0) throw new Error("interpolation is broken");
    if (t("no.such.key") !== "no.such.key") throw new Error("an unknown key should come back as itself");

    return "i18n self-test passed (" + Object.keys(STRINGS.en).length + " keys x " +
           codes.length + " languages: " + codes.join(" ") + ")";
  }

  global.I18N = {
    t, setLang, apply, onChange, languages, selfTest, keysUsedIn, fromTimeZone, ZONE_LANG,
    get lang() { return lang; },
  };

  // Every zone in the table must name a language the page actually speaks,
  // or the fallback silently does nothing for the people it was written for.
  function checkZones() {
    const bad = Object.entries(ZONE_LANG).filter(([, l]) => !STRINGS[l]);
    if (bad.length) throw new Error("zones point at languages that do not exist: " + JSON.stringify(bad));
    return Object.keys(ZONE_LANG).length + " time zones mapped to " +
      new Set(Object.values(ZONE_LANG)).size + " languages";
  }

  function checkMarkup() {
    let html;
    try {
      html = require("fs").readFileSync(require("path").join(__dirname, "index.html"), "utf8");
    } catch {
      return "index.html not readable from here, markup keys unchecked";
    }
    const used = keysUsedIn(html);
    const missing = used.filter((k) => !STRINGS.en[k]);
    if (missing.length) {
      throw new Error("index.html asks for keys no language defines: " + missing.join(", "));
    }
    return used.length + " markup keys, all defined";
  }

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
    console.log(checkMarkup());
    console.log(checkZones());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
