// How the results list orders itself, and the two questions it is built on.
//
// This is the WEB's order, and it is deliberately not the terminal's, see
// resultRank in scanner/scanner.go, which leads with the images that decoded.
// Here the row is the unit of work: it carries the thumbnail a person looks at,
// the rename preview, and the filter tab that selects it, so the rows with an
// action behind them lead. In the terminal no action is per-row, and leading
// with failures would push every payload past the first screen.
//
// It lives in its own file so each surface can pin its order in its own test.
// A change to one must not quietly move the other, and when both orders were
// one function that is exactly what happened.

(function (global) {
  "use strict";

  // The classification the scanner sends for an image holding a QR code it
  // located and could not read. The string is the scanner's, not the page's,
  // see scanner.Classification.
  const UNREAD = "QR_DETECTED_DECODE_FAILED";

  // An image holding a code nobody could read. The one bucket a person can act
  // on, which is why it gets its own count, its own tab, and the top of the
  // list.
  const isUnread = (r) => r.reason === UNREAD;

  // Settled, opened fine, and no payload came out: the two states that used to
  // be merged into a single "without" number.
  const settledEmpty = (r) => !r.pending && !r.error && r.codes.length === 0;

  // Rows worth doing something about, then the ones with nothing in them, then
  // the codes that came out fine, then the files that would not open. Rows
  // still being scanned wait at the end rather than shuffling the list every
  // time one lands.
  function rowRank(r) {
    if (r.pending) return 9;
    if (isUnread(r)) return 0;
    if (r.error) return 3;
    return r.codes.length ? 2 : 1;
  }

  function selfTest() {
    const row = (name, over) =>
      Object.assign({ name, codes: [], error: "", reason: "", pending: false }, over);

    const set = [
      row("decoded.png", { codes: ["X"], reason: "DECODED" }),
      row("broken.heic", { error: "could not be read" }),
      row("unread.png", { reason: UNREAD }),
      row("scanning.png", { pending: true }),
      row("nothing.png", { reason: "NO_QR_FOUND" }),
    ];

    // The order this surface promises. If this line has to change, the change
    // is a product decision and not a refactor.
    const want = ["unread.png", "nothing.png", "decoded.png", "broken.heic", "scanning.png"];
    const got = set.slice().sort((a, b) => rowRank(a) - rowRank(b)).map((r) => r.name);
    if (String(got) !== String(want)) {
      throw new Error("web row order is " + got + ", want " + want);
    }

    // The distinction the whole change exists for: two rows that both have no
    // payload and both opened fine must not rank the same.
    const unread = set.find((r) => r.name === "unread.png");
    const nothing = set.find((r) => r.name === "nothing.png");
    if (rowRank(unread) === rowRank(nothing)) {
      throw new Error("located-but-unread and no-code-at-all rank the same");
    }
    if (!settledEmpty(unread) || !settledEmpty(nothing)) {
      throw new Error("settledEmpty missed a settled row with no payload");
    }
    if (settledEmpty(set.find((r) => r.name === "scanning.png"))) {
      throw new Error("settledEmpty accepted a row that is still scanning");
    }
    if (isUnread(nothing) || !isUnread(unread)) {
      throw new Error("isUnread does not follow the scanner's classification");
    }

    // The terminal orders the same data differently on purpose. Assert the
    // difference so nobody "fixes" one side into the other by accident: there,
    // decoded leads.
    if (rowRank(set[0]) === 0) {
      throw new Error("a decoded row leads the web order; that is the CLI's order, not this one");
    }

    return "order self-test passed (web order: " + want.join(" < ") + ")";
  }

  global.Order = { rowRank, isUnread, settledEmpty, UNREAD, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
