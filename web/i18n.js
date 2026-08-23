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
    return "en";
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
  }

  const languages = () => Object.keys(STRINGS).map((code) => ({ code, name: NAMES[code] || code }));
  const onChange = (cb) => listeners.push(cb);

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
    t, setLang, apply, onChange, languages, selfTest,
    get lang() { return lang; },
  };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
