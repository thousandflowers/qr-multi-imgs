// Turning a decoded payload back into a file: SVG, PNG, JPEG or PDF.
//
// The engine hands over a module matrix - which squares are black - and
// everything here is about writing that matrix into one of four containers.
// Three of the four are vector or vector-ish, and building them from modules
// rather than from a raster is why the wasm side returns a matrix instead of
// an image: a QR redrawn from pixels is a QR someone has to trace back.
//
// No dependency does any of this. The PDF in particular is written by hand,
// which sounds worse than it is: a page of black rectangles is about the
// smallest useful PDF there is, and pulling in a PDF library to draw squares
// would be a megabyte to avoid ninety lines.

(function (global) {
  "use strict";

  // Quiet zone, in modules. The spec says four. Codes get printed and
  // photographed, and a code with no margin is a code a scanner cannot find
  // the edge of.
  const QUIET = 4;

  function normalise(m) {
    if (!m || typeof m.size !== "number" || !Array.isArray(m.bits)) return null;
    if (m.size <= 0 || m.bits.length !== m.size * m.size) return null;
    return m;
  }

  // runs walks a row and returns [start, length] for each stretch of black.
  // Merging horizontal neighbours into one rectangle is what keeps an SVG or a
  // PDF page small: a version-10 code is 3249 modules and a few hundred runs.
  function runs(m, y) {
    const out = [];
    let x = 0;
    while (x < m.size) {
      if (!m.bits[y * m.size + x]) { x++; continue; }
      const start = x;
      while (x < m.size && m.bits[y * m.size + x]) x++;
      out.push([start, x - start]);
    }
    return out;
  }

  function toSVG(matrix, px) {
    const m = normalise(matrix);
    if (!m) return null;
    const side = m.size + QUIET * 2;
    const size = px || side * 8;
    const parts = [];
    for (let y = 0; y < m.size; y++) {
      for (const [x, len] of runs(m, y)) {
        parts.push(`M${x + QUIET} ${y + QUIET}h${len}v1h-${len}z`);
      }
    }
    return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" ` +
      `viewBox="0 0 ${side} ${side}" shape-rendering="crispEdges">` +
      `<rect width="${side}" height="${side}" fill="#fff"/>` +
      `<path d="${parts.join("")}" fill="#000"/></svg>`;
  }

  // toCanvas draws at whole-pixel module size, so a module is never 3.4 pixels
  // wide and rounded differently on one side of the code than the other. That
  // unevenness is exactly what stops a decoder reading a small render.
  function toCanvas(matrix, px, CanvasCtor) {
    const m = normalise(matrix);
    if (!m) return null;
    const side = m.size + QUIET * 2;
    const scale = Math.max(1, Math.floor((px || 512) / side));
    const dim = side * scale;
    // CanvasCtor is a factory, not a class: a DOM canvas is made with
    // createElement and sized afterwards, an OffscreenCanvas takes its size in
    // the constructor, and pretending they are the same shape breaks one of
    // the two.
    let canvas;
    if (typeof CanvasCtor === "function" && CanvasCtor.length === 0 && !CanvasCtor.prototype) {
      canvas = CanvasCtor();
      canvas.width = dim;
      canvas.height = dim;
    } else {
      canvas = new (CanvasCtor || global.OffscreenCanvas)(dim, dim);
    }
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, dim, dim);
    ctx.fillStyle = "#000";
    for (let y = 0; y < m.size; y++) {
      for (const [x, len] of runs(m, y)) {
        ctx.fillRect((x + QUIET) * scale, (y + QUIET) * scale, len * scale, scale);
      }
    }
    return canvas;
  }

  // toPDF writes one page per code, as vector rectangles.
  //
  // Hand-written, and deliberately the dullest possible PDF: no compression, no
  // fonts, no xref streams. Every offset is counted as the bytes are appended,
  // because an xref table with a wrong offset is a file that opens in one
  // reader and fails in another - and "it worked in Preview" is not a test.
  function toPDF(matrices, opts) {
    const list = (Array.isArray(matrices) ? matrices : [matrices]).map(normalise).filter(Boolean);
    if (!list.length) return null;
    const pt = (opts && opts.sizePt) || 288; // 4 inches
    const margin = (opts && opts.marginPt) || 36;
    const pageW = pt + margin * 2;

    const objs = [];        // 1-based, index 0 unused
    const push = (body) => { objs.push(body); return objs.length; };

    const pagesId = 1;
    objs.push(null);        // reserve object 1 for the page tree
    const kids = [];
    for (const m of list) {
      const side = m.size + QUIET * 2;
      const unit = pt / side;
      const rects = [];
      for (let y = 0; y < m.size; y++) {
        for (const [x, len] of runs(m, y)) {
          // PDF's origin is bottom-left, so the row index is flipped.
          const px = margin + (x + QUIET) * unit;
          const py = margin + (side - 1 - (y + QUIET)) * unit;
          rects.push(`${px.toFixed(3)} ${py.toFixed(3)} ${(len * unit).toFixed(3)} ${unit.toFixed(3)} re`);
        }
      }
      const stream = `1 1 1 rg 0 0 ${pageW} ${pageW} re f\n0 0 0 rg\n${rects.join("\n")}\nf\n`;
      const contentId = push(`<< /Length ${stream.length} >>\nstream\n${stream}endstream`);
      const pageId = push(`<< /Type /Page /Parent ${pagesId} 0 R /MediaBox [0 0 ${pageW} ${pageW}] ` +
        `/Contents ${contentId} 0 R /Resources << >> >>`);
      kids.push(pageId);
    }
    objs[0] = `<< /Type /Pages /Kids [${kids.map((k) => k + " 0 R").join(" ")}] /Count ${kids.length} >>`;
    const catalogId = push(`<< /Type /Catalog /Pages ${pagesId} 0 R >>`);

    let out = "%PDF-1.4\n";
    const offsets = [0];
    for (let i = 0; i < objs.length; i++) {
      offsets.push(out.length);
      out += `${i + 1} 0 obj\n${objs[i]}\nendobj\n`;
    }
    const xref = out.length;
    out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n`;
    for (let i = 1; i <= objs.length; i++) {
      out += String(offsets[i]).padStart(10, "0") + " 00000 n \n";
    }
    out += `trailer\n<< /Size ${objs.length + 1} /Root ${catalogId} 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
    return out;
  }

  function selfTest() {
    // A 3x3 matrix with a known shape: a full top row and one middle module.
    const m = { size: 3, bits: [1, 1, 1, 0, 1, 0, 0, 0, 0] };

    // Horizontal runs merge; three modules in a row are one rectangle.
    const r0 = runs(m, 0), r1 = runs(m, 1), r2 = runs(m, 2);
    if (r0.length !== 1 || r0[0][0] !== 0 || r0[0][1] !== 3) throw new Error("top row should be one run of 3: " + JSON.stringify(r0));
    if (r1.length !== 1 || r1[0][0] !== 1 || r1[0][1] !== 1) throw new Error("middle row wrong: " + JSON.stringify(r1));
    if (r2.length !== 0) throw new Error("empty row produced runs");

    const svg = toSVG(m, 100);
    if (!svg.startsWith("<svg") || !svg.includes(`viewBox="0 0 11 11"`)) {
      throw new Error("svg viewBox must include the quiet zone: " + svg.slice(0, 120));
    }
    if (!svg.includes("M4 4h3v1h-3z")) throw new Error("svg lost the top run: " + svg);
    if (!svg.includes('fill="#fff"')) throw new Error("svg has no white ground; a transparent QR is unreadable on dark paper");

    const pdf = toPDF(m);
    if (!pdf.startsWith("%PDF-1.4")) throw new Error("not a PDF");
    if (!pdf.includes("/Type /Catalog") || !pdf.includes("/Type /Pages") || !pdf.includes("/Type /Page ")) {
      throw new Error("PDF is missing the object tree");
    }
    // The xref offsets have to point at the objects. Check every one.
    const startxref = +pdf.slice(pdf.lastIndexOf("startxref") + 9).trim().split("\n")[0];
    if (pdf.slice(startxref, startxref + 4) !== "xref") throw new Error("startxref does not point at the table");
    const table = pdf.slice(startxref).split("\n");
    const count = +table[1].split(" ")[1];
    for (let i = 1; i < count; i++) {
      const off = +table[i + 2].slice(0, 10);
      const want = `${i} 0 obj`;
      if (pdf.slice(off, off + want.length) !== want) {
        throw new Error(`xref entry ${i} points at ${JSON.stringify(pdf.slice(off, off + 20))}, want ${want}`);
      }
    }
    // Several codes make several pages.
    const two = toPDF([m, m]);
    if (!/\/Count 2\b/.test(two)) throw new Error("two matrices did not make a two-page PDF");

    for (const bad of [null, undefined, {}, { size: 2, bits: [1] }, { size: 0, bits: [] }]) {
      if (toSVG(bad) !== null || toPDF(bad) !== null) throw new Error("a malformed matrix was accepted");
    }
    return "qrgen self-test passed (runs merge, svg quiet zone, pdf xref offsets)";
  }

  global.QRGen = { toSVG, toCanvas, toPDF, runs, QUIET, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
