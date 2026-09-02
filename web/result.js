// What a decode result carries from the engine to the row, and the coordinate
// space its geometry lives in.
//
// This file exists because of a bug it now makes impossible. The Go side
// located QR codes it could not read, computed a bounding box for each, and
// marshalled them across the wasm boundary — and the worker copied
// `res.classification` into `out.reason` and dropped `res.detections` on the
// floor. Nothing broke, nothing errored, and the page simply never had the
// geometry. The web UI's whole reason for wanting it is to pre-draw a box the
// user adjusts instead of tracing one from scratch, and that feature was
// unbuildable from data that never arrived.
//
// So the hop is a named function with a test rather than three assignments in
// a worker. A worker cannot be loaded by node — importScripts and self do not
// exist there — which is why the mapping lives here and the worker calls it.
//
// It draws nothing. Mapping a box onto a canvas is the next step and belongs
// with the canvas.

(function (global) {
  "use strict";

  // A detection is one located-but-unread QR code, in the shape
  // scanner.Detection marshals to on every surface: a nested box, a module
  // size in pixels, and an estimated version that is 0 when the geometry did
  // not yield a plausible one.
  //
  // Anything malformed is dropped rather than repaired. A half-read box would
  // be drawn somewhere wrong on a photograph, and a box in the wrong place is
  // worse than no box: it tells the user the tool found something where it did
  // not.
  function detection(d) {
    if (!d || typeof d !== "object") return null;
    const b = d.box;
    if (!b || typeof b !== "object") return null;
    const nums = [b.x, b.y, b.w, b.h];
    if (!nums.every((n) => typeof n === "number" && Number.isFinite(n))) return null;
    // A zero-area box is not a location. The Go side already refuses to emit
    // one; this is the assertion that it stays that way across the boundary.
    if (b.w <= 0 || b.h <= 0) return null;
    return {
      box: { x: b.x, y: b.y, w: b.w, h: b.h },
      moduleSize: typeof d.module_size === "number" ? d.module_size : 0,
      version: typeof d.version === "number" ? d.version : 0,
      // The payload when this code was read, empty when it was only located.
      // It is what decides the colour a viewfinder draws it in.
      text: typeof d.text === "string" ? d.text : "",
    };
  }

  function detections(list) {
    if (!Array.isArray(list)) return [];
    return list.map(detection).filter(Boolean);
  }

  // carry maps one qrDecode() return value into the fields a worker message
  // hands to the page.
  //
  // frame is the pixel buffer the decode actually ran on, which is NOT
  // necessarily the file's own size: the page decodes at a cap first and
  // retries at a larger one, so a box may be expressed in a shrunk image's
  // coordinates. It travels with the boxes because only the caller knows the
  // factor, and a consumer that has the boxes without it cannot draw them
  // correctly on anything.
  function carry(res, frame) {
    res = res || {};
    return {
      codes: Array.isArray(res.codes) ? res.codes : [],
      // Why this image ended up where it did. The page turns it into a
      // sentence; this only carries it.
      reason: res.classification || "",
      detections: detections(res.detections),
      frame: frameOf(frame),
    };
  }

  function frameOf(f) {
    if (!f) return null;
    const n = (v) => (typeof v === "number" && Number.isFinite(v) && v > 0 ? v : 0);
    const out = { w: n(f.w), h: n(f.h), naturalW: n(f.naturalW), naturalH: n(f.naturalH) };
    return out.w && out.h ? out : null;
  }

  // adopt copies the carried fields onto a row. It is the second half of the
  // same hop: the worker can carry a field correctly and the page can still
  // fail to keep it.
  function adopt(row, msg) {
    msg = msg || {};
    row.codes = msg.codes || [];
    row.reason = msg.reason || "";
    row.detections = Array.isArray(msg.detections) ? msg.detections : [];
    row.frame = msg.frame || null;
    return row;
  }

  function selfTest() {
    const box = { x: 10, y: 20, w: 30, h: 40 };
    const res = {
      codes: [],
      classification: "QR_DETECTED_DECODE_FAILED",
      detections: [{ box, module_size: 4.5, version: 7 }],
    };
    const frame = { w: 1600, h: 1200, naturalW: 3200, naturalH: 2400 };

    // The regression this file exists for.
    const carried = carry(res, frame);
    if (carried.detections.length !== 1) {
      throw new Error("carry dropped the detections; that is the bug this file exists to stop");
    }
    const d = carried.detections[0];
    if (d.box.x !== 10 || d.box.y !== 20 || d.box.w !== 30 || d.box.h !== 40) {
      throw new Error("carry mangled the box: " + JSON.stringify(d.box));
    }
    if (d.moduleSize !== 4.5 || d.version !== 7) {
      throw new Error("carry lost the module size or version: " + JSON.stringify(d));
    }
    if (d.text !== "") throw new Error("an unread detection must carry no text");
    const read = carry({ detections: [{ box, text: "hello" }] }, frame);
    if (read.detections[0].text !== "hello") throw new Error("carry dropped a decoded detection's payload");
    if (carried.reason !== "QR_DETECTED_DECODE_FAILED") {
      throw new Error("carry lost the classification");
    }

    // And the second half of the hop.
    const row = adopt({}, carried);
    if (row.detections.length !== 1 || row.detections[0].box.w !== 30) {
      throw new Error("adopt dropped the detections on the way to the row");
    }
    if (!row.frame || row.frame.w !== 1600 || row.frame.naturalW !== 3200) {
      throw new Error("adopt lost the frame the boxes are measured in");
    }

    // Geometry without its coordinate space is unusable, so the frame must
    // survive alongside it or be honestly absent.
    if (carry(res, null).frame !== null) {
      throw new Error("a missing frame must be null, not invented");
    }
    if (carry(res, { w: 0, h: 0 }).frame !== null) {
      throw new Error("a zero-sized frame is not a coordinate space");
    }

    // A result with nothing in it is the common case and must not throw.
    const empty = carry({ codes: ["x"], classification: "DECODED" }, frame);
    if (empty.detections.length !== 0 || empty.codes[0] !== "x") {
      throw new Error("a decoded result with no detections did not come through clean");
    }
    if (carry(undefined, undefined).codes.length !== 0) {
      throw new Error("carry(undefined) must not throw");
    }

    // Malformed geometry is dropped, never repaired: a box in the wrong place
    // is worse than no box.
    const bad = carry({
      detections: [
        null,
        {},
        { box: null },
        { box: { x: 1, y: 2, w: 0, h: 5 } },
        { box: { x: 1, y: 2, w: 5, h: -1 } },
        { box: { x: "1", y: 2, w: 5, h: 5 } },
        { box: { x: 1, y: 2, w: NaN, h: 5 } },
        { box },
      ],
    }, frame);
    if (bad.detections.length !== 1) {
      throw new Error("malformed detections were not dropped: " + JSON.stringify(bad.detections));
    }
    if (bad.detections[0].moduleSize !== 0 || bad.detections[0].version !== 0) {
      throw new Error("a detection with no module size or version must read 0, not undefined");
    }

    return "result self-test passed (geometry survives engine -> worker -> row)";
  }

  global.Result = { carry, adopt, detections, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
