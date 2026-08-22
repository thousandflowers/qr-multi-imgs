// Two languages, kept as data.
//
// Italian is not a translation layer bolted on: for the person photographing
// receipts in a shop it is the only language the page will ever be read in.
// So the strings live side by side and the page picks by the browser's own
// preference, with an explicit switch for when that guess is wrong.

(function (global) {
  "use strict";

  const STRINGS = {
    en: {
      "app.tagline": "QR scanner",
      "nav.advanced": "Advanced",
      "nav.source": "Source",

      "hero.title": "Read, filter and export every QR code in a folder of images",
      "hero.sub": "Most scanners take one picture at a time. Drop a whole folder — or one photo, your camera, or whatever is on the clipboard — and get every code in every image at once.",
      "hero.trust": "Nothing is uploaded — it all runs in this tab",

      "tab.images": "Images",
      "tab.camera": "Camera",

      "drop.title": "Drag & drop, or browse",
      "drop.sub": "One photo, a folder, or several folders",
      "drop.browse": "Browse files",
      "drop.folder": "Choose folder",
      "drop.paste": "Paste an image",
      "drop.shoot": "Take a photo",

      "cam.off": "The camera is off.<br>Point it at a code and it reads it live.",
      "cam.start": "Start camera",
      "cam.stop": "Stop camera",
      "cam.looking": "Looking for a code…",
      "cam.denied": "Camera permission was declined.",
      "cam.none": "No camera available here.",
      "cam.read": "Read {n} code(s) so far.",

      "res.title": "Results",
      "res.private": "100% private · runs in your browser",
      "res.empty": "Codes you scan will show up here.",
      "res.nothing": "Nothing there this page can read — pictures only.",
      "res.search": "Search inside the codes…",
      "res.noMatch": "No result matches that search.",
      "res.noCode": "No code in this image",
      "engine.loading": "starting the reader",
      "engine.ready": "reader ready",

      "stat.images": "images",
      "stat.with": "with a code",
      "stat.codes": "codes",
      "stat.unique": "unique",
      "stat.without": "without",
      "stat.unreadable": "unreadable",

      "filter.all": "Everything",
      "filter.with": "Only with a code",
      "filter.without": "Only without",
      "filter.error": "Only unreadable",
      "filter.dupes": "Only duplicates",

      "act.copyAll": "Copy all",
      "act.copy": "copy",
      "act.copied": "copied",
      "act.download": "Download",
      "act.clear": "Clear",
      "act.open": "Open",
      "act.saveContact": "Save contact",
      "act.reveal": "Reveal",
      "act.hide": "Hide",

      "rename.title": "Rename and download",
      "rename.sub": "Build new file names from what each code says, and take the images away in one archive.",
      "rename.pattern": "New name",
      "rename.code": "The code content",
      "rename.short": "The meaningful part of the code",
      "rename.nameCode": "Original name + code",
      "rename.keep": "Keep the original names",
      "rename.zip": "Download ZIP ({n})",
      "rename.none": "No image here has a code to be named after.",
      "rename.preview": "will be saved as",

      "dupe.badge": "in {n} images",
      "dupe.first": "first seen",

      "err.advice": "Retake the photo closer, with the code flat and evenly lit — or crop tight around it and try again.",
      "err.format": "This browser has no decoder for that file type.",

      "adv.title": "Advanced",
      "adv.sub": "For scripted use and for acting on files where they sit. Not needed to read codes.",
      "adv.cli": "Command line and batch use",
      "adv.localOn": "A local copy of the program is answering, so this page can also act on files on disk.",
      "adv.scan": "Scan on disk",
      "adv.paths": "Folders or files, one per line",
      "adv.recreate": "Recreate QR images",
      "adv.organize": "Sort into with_qr / without_qr",
      "adv.delete": "Delete images without a code",
      "adv.confirmOrganize": "Move every scanned image into a with_qr / without_qr folder beside it?",
      "adv.confirmDelete": "Permanently delete every scanned image that has no QR code. This cannot be undone. Continue?",
      "adv.scanFirst": "Scan on disk first.",

      "foot.privacy": "Your images never leave this device.",

      "kind.url": "Link",
      "kind.wifi": "Wi-Fi network",
      "kind.contact": "Contact",
      "kind.payment": "Payment",
      "kind.email": "Email",
      "kind.phone": "Phone number",
      "kind.sms": "Text message",
      "kind.location": "Location",
      "kind.event": "Event",
      "kind.crypto": "Crypto address",
      "kind.otp": "Two-factor setup",
      "kind.reference": "Reference",
      "kind.text": "Text",

      "f.site": "Site", "f.reference": "Reference", "f.address": "Address",
      "f.network": "Network", "f.security": "Security", "f.password": "Password",
      "f.name": "Name", "f.phone": "Phone", "f.email": "Email", "f.company": "Company",
      "f.payee": "Payee", "f.iban": "IBAN", "f.amount": "Amount",
      "f.subject": "Subject", "f.message": "Message", "f.number": "Number",
      "f.latitude": "Latitude", "f.longitude": "Longitude",
      "f.title": "Title", "f.starts": "Starts", "f.ends": "Ends", "f.where": "Where",
      "f.account": "Account", "f.issuer": "Issuer", "f.algorithm": "Algorithm",
      "f.secret": "Secret key", "f.content": "Content",
    },

    it: {
      "app.tagline": "lettore QR",
      "nav.advanced": "Avanzate",
      "nav.source": "Codice",

      "hero.title": "Leggi, filtra ed esporta ogni codice QR in una cartella di immagini",
      "hero.sub": "Quasi tutti i lettori prendono una foto alla volta. Qui trascini una cartella intera — o una sola foto, la fotocamera, o quello che hai negli appunti — e ottieni ogni codice di ogni immagine in un colpo solo.",
      "hero.trust": "Non viene caricato niente — funziona tutto in questa scheda",

      "tab.images": "Immagini",
      "tab.camera": "Fotocamera",

      "drop.title": "Trascina qui, oppure scegli",
      "drop.sub": "Una foto, una cartella, o più cartelle insieme",
      "drop.browse": "Scegli i file",
      "drop.folder": "Scegli una cartella",
      "drop.paste": "Incolla un'immagine",
      "drop.shoot": "Scatta una foto",

      "cam.off": "La fotocamera è spenta.<br>Inquadra un codice e lo legge dal vivo.",
      "cam.start": "Accendi la fotocamera",
      "cam.stop": "Spegni la fotocamera",
      "cam.looking": "Sto cercando un codice…",
      "cam.denied": "Permesso della fotocamera negato.",
      "cam.none": "Qui non c'è una fotocamera disponibile.",
      "cam.read": "Letti {n} codici finora.",

      "res.title": "Risultati",
      "res.private": "100% privato · funziona nel tuo browser",
      "res.empty": "I codici che leggi compaiono qui.",
      "res.nothing": "Lì dentro non c'è niente che questa pagina sappia leggere — solo immagini.",
      "res.search": "Cerca dentro i codici…",
      "res.noMatch": "Nessun risultato corrisponde alla ricerca.",
      "res.noCode": "Nessun codice in questa immagine",
      "engine.loading": "avvio del lettore",
      "engine.ready": "lettore pronto",

      "stat.images": "immagini",
      "stat.with": "con un codice",
      "stat.codes": "codici",
      "stat.unique": "univoci",
      "stat.without": "senza",
      "stat.unreadable": "illeggibili",

      "filter.all": "Tutto",
      "filter.with": "Solo con un codice",
      "filter.without": "Solo senza",
      "filter.error": "Solo illeggibili",
      "filter.dupes": "Solo doppioni",

      "act.copyAll": "Copia tutto",
      "act.copy": "copia",
      "act.copied": "copiato",
      "act.download": "Scarica",
      "act.clear": "Svuota",
      "act.open": "Apri",
      "act.saveContact": "Salva contatto",
      "act.reveal": "Mostra",
      "act.hide": "Nascondi",

      "rename.title": "Rinomina e scarica",
      "rename.sub": "Costruisci i nomi dei file da quello che dice ogni codice, e porta via le immagini in un archivio solo.",
      "rename.pattern": "Nuovo nome",
      "rename.code": "Il contenuto del codice",
      "rename.short": "La parte utile del codice",
      "rename.nameCode": "Nome originale + codice",
      "rename.keep": "Tieni i nomi originali",
      "rename.zip": "Scarica ZIP ({n})",
      "rename.none": "Nessuna immagine qui ha un codice da cui prendere il nome.",
      "rename.preview": "diventerà",

      "dupe.badge": "in {n} immagini",
      "dupe.first": "prima vista",

      "err.advice": "Rifai la foto più da vicino, con il codice dritto e illuminato in modo uniforme — oppure ritaglia stretto attorno al codice e riprova.",
      "err.format": "Questo browser non sa aprire questo tipo di file.",

      "adv.title": "Avanzate",
      "adv.sub": "Per l'uso da script e per agire sui file dove si trovano. Non serve per leggere i codici.",
      "adv.cli": "Riga di comando e uso in blocco",
      "adv.localOn": "Una copia locale del programma sta rispondendo, quindi questa pagina può anche agire sui file su disco.",
      "adv.scan": "Analizza su disco",
      "adv.paths": "Cartelle o file, uno per riga",
      "adv.recreate": "Rigenera le immagini QR",
      "adv.organize": "Ordina in with_qr / without_qr",
      "adv.delete": "Elimina le immagini senza codice",
      "adv.confirmOrganize": "Spostare ogni immagine analizzata in una cartella with_qr / without_qr accanto ad essa?",
      "adv.confirmDelete": "Eliminare definitivamente ogni immagine analizzata che non ha un codice QR. Non si può annullare. Procedere?",
      "adv.scanFirst": "Prima analizza su disco.",

      "foot.privacy": "Le tue immagini non lasciano mai questo dispositivo.",

      "kind.url": "Link",
      "kind.wifi": "Rete Wi-Fi",
      "kind.contact": "Contatto",
      "kind.payment": "Pagamento",
      "kind.email": "Email",
      "kind.phone": "Numero di telefono",
      "kind.sms": "Messaggio",
      "kind.location": "Posizione",
      "kind.event": "Evento",
      "kind.crypto": "Indirizzo crypto",
      "kind.otp": "Configurazione a due fattori",
      "kind.reference": "Riferimento",
      "kind.text": "Testo",

      "f.site": "Sito", "f.reference": "Riferimento", "f.address": "Indirizzo",
      "f.network": "Rete", "f.security": "Sicurezza", "f.password": "Password",
      "f.name": "Nome", "f.phone": "Telefono", "f.email": "Email", "f.company": "Azienda",
      "f.payee": "Beneficiario", "f.iban": "IBAN", "f.amount": "Importo",
      "f.subject": "Oggetto", "f.message": "Messaggio", "f.number": "Numero",
      "f.latitude": "Latitudine", "f.longitude": "Longitudine",
      "f.title": "Titolo", "f.starts": "Inizio", "f.ends": "Fine", "f.where": "Dove",
      "f.account": "Account", "f.issuer": "Servizio", "f.algorithm": "Algoritmo",
      "f.secret": "Chiave segreta", "f.content": "Contenuto",
    },
  };

  const STORE = "qmi.lang";
  const listeners = [];

  function pick() {
    try {
      const saved = localStorage.getItem(STORE);
      if (saved && STRINGS[saved]) return saved;
    } catch { /* private mode; the browser preference still decides */ }
    const want = navigator.languages || [navigator.language || "en"];
    for (const l of want) if (String(l).toLowerCase().startsWith("it")) return "it";
    return "en";
  }

  let lang = typeof navigator === "undefined" ? "en" : pick();

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

  function onChange(cb) { listeners.push(cb); }

  function selfTest() {
    const missing = [];
    for (const k in STRINGS.en) if (!(k in STRINGS.it)) missing.push("it:" + k);
    for (const k in STRINGS.it) if (!(k in STRINGS.en)) missing.push("en:" + k);
    if (missing.length) throw new Error("untranslated keys: " + missing.join(", "));
    if (t("dupe.badge", { n: 3 }).indexOf("3") < 0) throw new Error("interpolation is broken");
    if (t("no.such.key") !== "no.such.key") throw new Error("an unknown key should come back as itself");
    return "i18n self-test passed (" + Object.keys(STRINGS.en).length + " keys, both languages)";
  }

  global.I18N = {
    t, setLang, apply, onChange, selfTest,
    get lang() { return lang; },
  };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
