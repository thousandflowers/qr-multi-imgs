// What a link in a QR code actually points at, before anybody follows it.
//
// A QR code is a URL nobody can read. That is the whole point of the format
// and it is also the whole problem: the one place a scanner can mislead a
// person is by taking them somewhere the code did not visibly say. So the page
// never opens a link straight from a scan - it shows the host first, in plain
// text, with anything structurally suspicious named.
//
// The checks here are STRUCTURAL. There is no list of known-bad domains and
// there will not be one: a blocklist is stale the day it ships, and a list of
// famous brands to compare against invites false accusations against anyone
// whose domain happens to be similar. What is checkable without a list is
// whether the URL is built to be misread, and that is what this does.

(function (global) {
  "use strict";

  // Anything not http(s) is not something a page should navigate to from a
  // scanned code. javascript: and data: are the dangerous ones; the rest are
  // simply not links.
  const OPENABLE = new Set(["http:", "https:"]);

  function inspect(text) {
    const out = { input: String(text == null ? "" : text), openable: false, warnings: [] };
    let u;
    try {
      u = new URL(out.input);
    } catch {
      out.reason = "notALink";
      return out;
    }
    out.scheme = u.protocol;
    out.host = u.hostname;
    out.port = u.port;
    out.path = u.pathname + u.search + u.hash;

    if (!OPENABLE.has(u.protocol)) {
      out.reason = "notOpenable";
      return out;
    }
    out.openable = true;

    // Credentials in the authority. "https://a-bank.example@evil.test/" reads
    // as a-bank.example to a person and resolves to evil.test - the oldest
    // trick there is, and the one worth naming loudest.
    if (u.username || u.password) out.warnings.push("userinfo");

    // Punycode. A host that is not what it looks like is the entire homograph
    // attack; the browser shows the pretty form and the resolver uses this one.
    if (/(^|\.)xn--/i.test(u.hostname)) out.warnings.push("punycode");

    // A bare address instead of a name. Legitimate on a local network, close to
    // never on a printed code.
    if (/^\d{1,3}(\.\d{1,3}){3}$/.test(u.hostname) || u.hostname.startsWith("[")) out.warnings.push("ipHost");

    // Not encrypted: anything typed on the far side travels in the open.
    if (u.protocol === "http:") out.warnings.push("insecure");

    // A pile of labels is how a real domain gets buried in the middle of a
    // hostname where the eye stops reading.
    if (u.hostname.split(".").length > 4) out.warnings.push("manyLabels");

    // An address long enough that its beginning and its end are never on
    // screen together.
    if (out.input.length > 300) out.warnings.push("veryLong");

    // The full host, and only the full host.
    //
    // An earlier version shortened it to the last two labels, to show "the
    // domain". Without a public-suffix list that is a guess, and it guesses
    // wrong on every co.uk in the world - it would have shown "co.uk" for
    // shop.example.co.uk. In a prompt whose entire job is to stop someone
    // being deceived about where they are going, a confidently wrong domain is
    // worse than no domain. The host is shown whole, and the UI emphasises it.
    out.display = u.hostname;
    return out;
  }

  function selfTest() {
    const w = (u) => inspect(u).warnings;
    const has = (u, k) => w(u).includes(k);

    const plain = inspect("https://example.com/a?b=1");
    if (!plain.openable || plain.host !== "example.com" || plain.warnings.length) {
      throw new Error("an ordinary https link should pass clean: " + JSON.stringify(plain));
    }

    // The one that matters most.
    if (!has("https://bank.example@evil.test/x", "userinfo")) throw new Error("credentials in the authority went unflagged");
    if (inspect("https://bank.example@evil.test/x").host !== "evil.test") {
      throw new Error("the host reported must be where it resolves, not what it reads like");
    }

    if (!has("https://xn--80ak6aa92e.com/", "punycode")) throw new Error("punycode host went unflagged");
    if (!has("http://example.com/", "insecure")) throw new Error("plain http went unflagged");
    if (!has("https://192.168.1.9/x", "ipHost")) throw new Error("an IP host went unflagged");
    if (!has("https://a.b.c.d.example.com/", "manyLabels")) throw new Error("a deep hostname went unflagged");
    if (!has("https://example.com/" + "x".repeat(400), "veryLong")) throw new Error("a very long URL went unflagged");

    // Never openable, whatever else is true of them.
    for (const bad of ["javascript:alert(1)", "data:text/html,<script>", "file:///etc/passwd"]) {
      const r = inspect(bad);
      if (r.openable) throw new Error("scheme must not be openable: " + bad);
      if (r.reason !== "notOpenable") throw new Error("wrong reason for " + bad + ": " + r.reason);
    }
    if (inspect("just some text").reason !== "notALink") throw new Error("plain text was parsed as a link");
    if (inspect(null).openable) throw new Error("null was openable");

    // Whole host, never a guessed "domain": the public-suffix problem makes
    // any shortening wrong for a large part of the world.
    if (inspect("https://shop.example.co.uk/x").display !== "shop.example.co.uk") {
      throw new Error("display must be the whole host: " + inspect("https://shop.example.co.uk/x").display);
    }
    return "linkcheck self-test passed (userinfo, punycode, scheme, ip, depth, length)";
  }

  global.LinkCheck = { inspect, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
