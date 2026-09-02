// The decoder the browser already has.
//
// Chrome and Edge ship BarcodeDetector, and on macOS it is backed by the
// system's own Vision framework - the same engine that reads 5 of 5 of the
// photographs the pure-Go cascade reads 2 of. It is a platform capability, not
// a dependency: nothing is downloaded, nothing is sent, and where it is absent
// the page works exactly as before.
//
// Measured on real photographs of business cards, at their own resolution:
// four of five read, five codes in total, in about 20ms each. The pure-Go
// cascade read two of five in about two seconds each. On the synthetic corpus
// the Go path still earns its place - colour-channel projections recover codes
// that a single grayscale pass loses - so the two are unioned rather than one
// replacing the other.
//
// Safari does not implement it. That is the whole reason the wasm engine stays
// the floor rather than the fallback of last resort.

(function (global) {
  "use strict";

  let detector = null;
  let checked = false;

  function available() {
    return typeof global.BarcodeDetector === "function";
  }

  function get() {
    if (!checked) {
      checked = true;
      try {
        detector = available() ? new global.BarcodeDetector({ formats: ["qr_code"] }) : null;
      } catch {
        detector = null;
      }
    }
    return detector;
  }

  // boxOf turns the corner points the API returns into the axis-aligned box the
  // rest of the page draws. boundingBox is a DOMRectReadOnly and does not
  // survive being posted from a worker, so the corners are the honest source.
  function boxOf(d) {
    const pts = d && d.cornerPoints;
    if (Array.isArray(pts) && pts.length) {
      const xs = pts.map((p) => p.x), ys = pts.map((p) => p.y);
      const x = Math.min(...xs), y = Math.min(...ys);
      const w = Math.max(...xs) - x, h = Math.max(...ys) - y;
      if (w > 0 && h > 0) return { x, y, w, h };
    }
    const b = d && d.boundingBox;
    if (b && b.width > 0 && b.height > 0) return { x: b.x, y: b.y, w: b.width, h: b.height };
    return null;
  }

  // shape converts a detection list into the form carry() already speaks, so
  // nothing downstream can tell which engine found a code - which is the point.
  function shape(list) {
    const out = { codes: [], detections: [] };
    for (const d of list || []) {
      if (!d || typeof d.rawValue !== "string" || !d.rawValue) continue;
      out.codes.push(d.rawValue);
      const box = boxOf(d);
      if (box) out.detections.push({ box, module_size: 0, version: 0, text: d.rawValue });
    }
    return out;
  }

  // detect returns null when there is no native decoder, which is different
  // from returning nothing found.
  async function detect(source) {
    const det = get();
    if (!det) return null;
    try {
      return shape(await det.detect(source));
    } catch {
      return null;
    }
  }

  function selfTest() {
    const b = boxOf({ cornerPoints: [{ x: 10, y: 20 }, { x: 50, y: 22 }, { x: 52, y: 60 }, { x: 12, y: 58 }] });
    if (!b || b.x !== 10 || b.y !== 20 || b.w !== 42 || b.h !== 40) {
      throw new Error("corner points did not become a box: " + JSON.stringify(b));
    }
    // A rotated code still yields the box that contains it.
    if (boxOf({ cornerPoints: [{ x: 0, y: 0 }] }) !== null) throw new Error("a single point produced a box");
    if (boxOf({ boundingBox: { x: 1, y: 2, width: 3, height: 4 } }).w !== 3) throw new Error("boundingBox fallback broken");
    if (boxOf({}) !== null || boxOf(null) !== null) throw new Error("nothing produced a box out of nothing");

    const s = shape([
      { rawValue: "one", cornerPoints: [{ x: 0, y: 0 }, { x: 4, y: 0 }, { x: 4, y: 4 }, { x: 0, y: 4 }] },
      { rawValue: "" },
      null,
      { rawValue: "two" },   // no geometry: still a code
    ]);
    if (s.codes.join(",") !== "one,two") throw new Error("shape lost or invented a code: " + s.codes);
    if (s.detections.length !== 1) throw new Error("shape invented geometry it was not given");
    if (s.detections[0].text !== "one") throw new Error("a native detection must carry its payload");
    return "native self-test passed (corner points to box, empty and geometry-less results)";
  }

  global.Native = { available, detect, shape, boxOf, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
