// Reading a ZIP, so a whole archive can be dropped on the page.
//
// No library: the browser already ships the hard part. DecompressionStream
// handles deflate, which is the only compression method anything writes in
// practice, and the rest is reading three fixed-layout records.
//
// The central directory is the authority, not the local headers: a streaming
// writer is allowed to leave the sizes in a local header as zero and put the
// real ones in a data descriptor after the payload. Trusting local headers
// works right up until it silently does not.

(function (global) {
  "use strict";

  const EOCD = 0x06054b50;
  const CENTRAL = 0x02014b50;
  const LOCAL = 0x04034b50;

  const looksLikeZip = (file) =>
    /\.zip$/i.test(file.name || "") ||
    file.type === "application/zip" ||
    file.type === "application/x-zip-compressed";

  // The end-of-central-directory record sits at the very end, unless a comment
  // follows it, so it is found by scanning backwards through the last 64 KB.
  function findEOCD(view) {
    const from = Math.max(0, view.byteLength - 65557);
    for (let i = view.byteLength - 22; i >= from; i--) {
      if (view.getUint32(i, true) === EOCD) return i;
    }
    return -1;
  }

  async function inflate(bytes, method) {
    if (method === 0) return bytes;
    if (method !== 8) throw new Error("unsupported compression method " + method);
    if (typeof DecompressionStream !== "function") {
      throw new Error("this browser cannot decompress a zip");
    }
    const stream = new Blob([bytes]).stream()
      .pipeThrough(new DecompressionStream("deflate-raw"));
    return new Uint8Array(await new Response(stream).arrayBuffer());
  }

  // entries returns every file in the archive as a blob, skipping directories,
  // and reporting rather than hiding what it could not open.
  async function entries(file) {
    const buf = await file.arrayBuffer();
    const view = new DataView(buf);
    const bytes = new Uint8Array(buf);

    const eocd = findEOCD(view);
    if (eocd < 0) throw new Error("not a zip file");

    const count = view.getUint16(eocd + 10, true);
    let offset = view.getUint32(eocd + 16, true);
    const out = [];
    const skipped = [];
    const decoder = new TextDecoder("utf-8");

    for (let i = 0; i < count; i++) {
      if (offset + 46 > view.byteLength || view.getUint32(offset, true) !== CENTRAL) break;

      const flags = view.getUint16(offset + 8, true);
      const method = view.getUint16(offset + 10, true);
      const compSize = view.getUint32(offset + 20, true);
      const nameLen = view.getUint16(offset + 28, true);
      const extraLen = view.getUint16(offset + 30, true);
      const commentLen = view.getUint16(offset + 32, true);
      const localAt = view.getUint32(offset + 42, true);
      const name = decoder.decode(bytes.subarray(offset + 46, offset + 46 + nameLen));
      offset += 46 + nameLen + extraLen + commentLen;

      if (name.endsWith("/")) continue;                   // a directory carries no data
      if (flags & 0x1) { skipped.push(name); continue; }  // encrypted
      if (view.getUint32(localAt, true) !== LOCAL) { skipped.push(name); continue; }

      const lNameLen = view.getUint16(localAt + 26, true);
      const lExtraLen = view.getUint16(localAt + 28, true);
      const dataAt = localAt + 30 + lNameLen + lExtraLen;

      try {
        const raw = bytes.subarray(dataAt, dataAt + compSize);
        out.push({ name, blob: new Blob([await inflate(raw, method)]) });
      } catch {
        skipped.push(name);
      }
    }
    return { files: out, skipped };
  }

  async function selfTest() {
    // Run from node the writer is not loaded yet; in the browser the page's
    // script tags have already put it in place.
    if (!global.Zip && typeof require === "function") require("./zip.js");

    // Round-trip through the writer next door: whatever store-only shape it
    // produces, the reader has to accept.
    const enc = new TextEncoder();
    const archive = global.Zip.build([
      { name: "uno.txt", data: enc.encode("primo") },
      { name: "cartella/due.txt", data: enc.encode("secondo") },
    ]);
    const { files, skipped } = await entries(archive);
    const names = files.map((f) => f.name).join(" ");
    if (names !== "uno.txt cartella/due.txt") throw new Error("names: " + names);
    if (skipped.length) throw new Error("nothing should have been skipped");
    const first = await files[0].blob.text();
    if (first !== "primo") throw new Error("content: " + first);

    let refused = false;
    try { await entries(new Blob([enc.encode("not an archive at all")])); }
    catch { refused = true; }
    if (!refused) throw new Error("a non-zip should be refused, not parsed");

    return "unzip self-test passed";
  }

  global.Unzip = { entries, looksLikeZip, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    selfTest().then(console.log, (e) => {
      console.error(String((e && e.message) || e));
      process.exit(1);
    });
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
