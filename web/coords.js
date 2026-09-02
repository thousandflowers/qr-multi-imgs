// The coordinate spaces a bounding box crosses, and the one function that
// crosses them.
//
// There are four, and drawing anything before they are reconciled ships a box
// that lies:
//
//   1. STORED     the pixels as the file encodes them, before any EXIF
//                 orientation is honoured.
//   2. NATURAL    what createImageBitmap(blob) hands back. On every engine
//                 measured this is the display-oriented image: see the note on
//                 orientation below.
//   3. FRAME      what the decoder actually saw. The worker caps the long side
//                 at 1600 for the first pass and retries at 3000, so a box can
//                 be expressed in a shrunk image's pixels.
//   4. DISPLAY    the size the image is drawn at on screen.
//
// Detections arrive in FRAME. A user's crop rectangle arrives in DISPLAY and
// has to reach NATURAL to be re-scanned. Both directions are here, because an
// error in the second one silently rescans the wrong region and nothing
// reports it: the user draws a box, the tool reads somewhere else, and the
// answer comes back "still nothing".
//
// EXIF ORIENTATION, MEASURED RATHER THAN ASSUMED
//
// Chrome 151 (macOS arm64, headless), main thread and worker alike, applies
// EXIF orientation in createImageBitmap and ignores the imageOrientation
// option entirely: for a real orientation-6 photo stored 4032x3024, all of
// `default`, `{imageOrientation:"none"}` and `{imageOrientation:"from-image"}`
// returned 3024x4032 with byte-identical corner fingerprints.
//
// So on that engine the decoder never sees an unrotated image, the boxes come
// back already display-oriented, and there is no rotation term to apply. But
// the safe rule does not depend on the engine, and it is this:
//
//   DERIVE THE CANVAS FROM THE SAME ImageBitmap THE DECODE USED.
//
// Then orientation cancels whatever the browser does with it, because both
// sides of the mapping inherit the same decision. The failure this avoids is
// mixing sources — decoding from an ImageBitmap and drawing from an <img>,
// which always applies orientation — and that failure is invisible on a
// desktop screenshot and wrong on every phone photo.
//
// There is deliberately no rotation parameter here. Adding one would invite a
// caller to correct for an orientation that has already been applied, which is
// the exact bug this comment exists to prevent.

(function (global) {
  "use strict";

  const finite = (n) => typeof n === "number" && Number.isFinite(n);

  // space validates a {w,h} pair, with an optional drawn origin. Zero is not a
  // coordinate space, and a scale factor derived from it is an Infinity that
  // propagates into a box nobody notices is wrong.
  function space(s) {
    if (!s || !finite(s.w) || !finite(s.h) || s.w <= 0 || s.h <= 0) return null;
    return { w: s.w, h: s.h, x: finite(s.x) ? s.x : 0, y: finite(s.y) ? s.y : 0 };
  }

  function box(b) {
    if (!b || !finite(b.x) || !finite(b.y) || !finite(b.w) || !finite(b.h)) return null;
    return { x: b.x, y: b.y, w: b.w, h: b.h };
  }

  // scaleBox maps a box between two spaces by per-axis factors.
  //
  // Per-axis rather than one uniform factor on purpose. The worker's resize
  // rounds each side independently, so a 4032x3024 photo capped at 1600 lands
  // at 1600x1200 and the two factors differ in the last decimal. One shared
  // factor would put the box a fraction out on the long axis of every photo,
  // which is not visible in a test with square inputs and is visible on a
  // phone.
  function scaleBox(b, from, to) {
    const fx = to.w / from.w, fy = to.h / from.h;
    return {
      x: to.x + (b.x - from.x) * fx,
      y: to.y + (b.y - from.y) * fy,
      w: b.w * fx,
      h: b.h * fy,
    };
  }

  // clampTo keeps a mapped box inside its target. Rounding at either end can
  // put a corner a fraction outside; a box that pokes past the image is drawn
  // over the page furniture and, on the way back, crops coordinates the source
  // does not have.
  function clampTo(b, s) {
    const x0 = Math.min(Math.max(b.x, s.x), s.x + s.w);
    const y0 = Math.min(Math.max(b.y, s.y), s.y + s.h);
    const x1 = Math.min(Math.max(b.x + b.w, s.x), s.x + s.w);
    const y1 = Math.min(Math.max(b.y + b.h, s.y), s.y + s.h);
    return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
  }

  // frameOf and naturalOf read the two spaces out of the frame record the
  // worker attaches to every row. Keeping them here means a caller never has to
  // remember which pair of fields is which.
  const frameOf = (f) => (f ? space({ w: f.w, h: f.h }) : null);
  const naturalOf = (f) => (f ? space({ w: f.naturalW, h: f.naturalH }) : null);

  // frameToNatural maps a decoder box into the image's own pixels, undoing the
  // 1600 cap or the 3000 retry — whichever the decode actually ran at, which is
  // why the frame travels with the boxes instead of being recomputed.
  function frameToNatural(b, frame) {
    const from = frameOf(frame), to = naturalOf(frame), bb = box(b);
    if (!from || !to || !bb) return null;
    return clampTo(scaleBox(bb, from, to), to);
  }

  function naturalToDisplay(b, frame, display) {
    const from = naturalOf(frame), to = space(display), bb = box(b);
    if (!from || !to || !bb) return null;
    return clampTo(scaleBox(bb, from, to), to);
  }

  // displayToNatural is the direction the user's crop travels. It is the one
  // that fails silently: a wrong box here re-scans a region the user did not
  // choose and reports that nothing was found there.
  function displayToNatural(b, frame, display) {
    const from = space(display), to = naturalOf(frame), bb = box(b);
    if (!from || !to || !bb) return null;
    return clampTo(scaleBox(bb, from, to), to);
  }

  // detectionToDisplay is the headline: one detection, the frame it was found
  // in, the size it is being shown at, and the box to draw. Every space it
  // crosses is an argument; none is implicit, global, or read off the DOM.
  //
  // Returns floats. Rounding is a rendering decision and belongs where the
  // drawing happens; rounding here would make the round trip lossy for the
  // crop that has to travel back.
  function detectionToDisplay(det, frame, display) {
    if (!det) return null;
    const nat = frameToNatural(det.box || det, frame);
    if (!nat) return null;
    return naturalToDisplay(nat, frame, display);
  }

  function selfTest() {
    const near = (a, b, tol, what) => {
      if (Math.abs(a - b) > tol) throw new Error(`${what}: ${a} vs ${b} (tol ${tol})`);
    };
    const roundTrip = (label, frame, display, b) => {
      const nat = frameToNatural(b, frame);
      const disp = naturalToDisplay(nat, frame, display);
      const back = displayToNatural(disp, frame, display);
      for (const k of ["x", "y", "w", "h"]) near(back[k], nat[k], 1e-6, `${label} float round trip ${k}`);
      // And the same trip with the rounding a renderer would do, which is the
      // form the requirement is stated in.
      const rounded = { x: Math.round(disp.x), y: Math.round(disp.y), w: Math.round(disp.w), h: Math.round(disp.h) };
      const back2 = displayToNatural(rounded, frame, display);
      // "Within a pixel" means within one DISPLAY pixel, and a display pixel is
      // worth this many natural pixels. Stating the tolerance in the wrong
      // space is how a mapping test passes while the mapping drifts.
      const perPx = Math.max(frame.naturalW / display.w, frame.naturalH / display.h);
      for (const k of ["x", "y", "w", "h"]) near(back2[k], nat[k], perPx, `${label} rounded round trip ${k}`);
      return { nat, disp };
    };

    // A 12-megapixel photo decoded under the 1600 cap and shown at 800 wide.
    // This is the case the previous verification could not exercise: the
    // 256x256 corpus image had a scale factor of exactly 1 on every axis and
    // would have passed with the scaling code deleted.
    const shrunk = { w: 1600, h: 1200, naturalW: 4032, naturalH: 3024 };
    const disp800 = { w: 800, h: 600 };
    const r1 = roundTrip("shrunk", shrunk, disp800, { x: 100, y: 80, w: 400, h: 300 });
    near(r1.nat.x, 252, 0.5, "shrunk natural x");
    near(r1.nat.w, 1008, 0.5, "shrunk natural w");
    near(r1.disp.x, 50, 0.5, "shrunk display x");
    near(r1.disp.w, 200, 0.5, "shrunk display w");
    if (r1.nat.w <= 400) throw new Error("the 1600 cap was not undone; the box is still in frame pixels");

    // The 3000 retry, which is a different factor from the first pass.
    roundTrip("retry", { w: 3000, h: 2250, naturalW: 4032, naturalH: 3024 }, { w: 600, h: 450 },
      { x: 10, y: 20, w: 900, h: 700 });

    // Non-square aspect, where a single shared scale factor would be wrong on
    // one axis and right on the other.
    const wide = { w: 1600, h: 400, naturalW: 4000, naturalH: 1000 };
    const r2 = roundTrip("wide", wide, { w: 533, h: 133 }, { x: 200, y: 50, w: 800, h: 200 });
    near(r2.nat.x, 500, 0.5, "wide natural x");
    near(r2.nat.y, 125, 0.5, "wide natural y");
    const fx = r2.nat.w / 800, fy = r2.nat.h / 200;
    near(fx, 2.5, 1e-9, "wide x factor");
    near(fy, 2.5, 1e-9, "wide y factor");

    // A frame the worker did not shrink at all, shown smaller. The identity
    // step must still be an identity.
    const same = { w: 900, h: 1600, naturalW: 900, naturalH: 1600 };
    const nat = frameToNatural({ x: 1, y: 2, w: 3, h: 4 }, same);
    for (const [k, v] of [["x", 1], ["y", 2], ["w", 3], ["h", 4]]) near(nat[k], v, 1e-9, "identity " + k);
    roundTrip("unshrunk", same, { w: 300, h: 533 }, { x: 100, y: 200, w: 300, h: 400 });

    // A canvas that letterboxes the image: the drawn rect has an origin, and a
    // box that ignores it lands in the letterbox.
    const off = naturalToDisplay({ x: 0, y: 0, w: 4032, h: 3024 }, shrunk, { w: 800, h: 600, x: 40, y: 10 });
    near(off.x, 40, 1e-9, "letterbox origin x");
    near(off.y, 10, 1e-9, "letterbox origin y");
    near(off.w, 800, 1e-9, "letterbox w");

    // The real detection from the corpus image, which is the shape the page
    // actually receives.
    const det = { box: { x: 54, y: 25, w: 200, h: 185 }, moduleSize: 8.93, version: 1 };
    const d = detectionToDisplay(det, { w: 256, h: 256, naturalW: 256, naturalH: 256 }, { w: 512, h: 512 });
    near(d.x, 108, 1e-9, "detection display x");
    near(d.w, 400, 1e-9, "detection display w");

    // Totality: no input produces NaN or a box in a space that does not exist.
    const bad = [
      [null, shrunk, disp800],
      [det, null, disp800],
      [det, shrunk, null],
      [det, { w: 0, h: 0, naturalW: 10, naturalH: 10 }, disp800],
      [det, { w: 10, h: 10, naturalW: 0, naturalH: 0 }, disp800],
      [det, shrunk, { w: 0, h: 10 }],
      [{ box: { x: 0, y: 0, w: NaN, h: 5 } }, shrunk, disp800],
    ];
    for (const [a, b, c] of bad) {
      if (detectionToDisplay(a, b, c) !== null) {
        throw new Error("a malformed mapping returned a box: " + JSON.stringify([a, b, c]));
      }
    }

    // Clamping: a box outside the frame cannot be mapped to somewhere outside
    // the image, because a crop there reads pixels the source does not have.
    const out = frameToNatural({ x: 1500, y: 1150, w: 500, h: 500 }, shrunk);
    if (out.x + out.w > 4032.0001 || out.y + out.h > 3024.0001) {
      throw new Error("a box past the frame edge escaped the image: " + JSON.stringify(out));
    }

    return "coords self-test passed (frame -> natural -> display -> natural, shrunk and non-square)";
  }

  global.Coords = {
    detectionToDisplay, frameToNatural, naturalToDisplay, displayToNatural, selfTest,
  };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
