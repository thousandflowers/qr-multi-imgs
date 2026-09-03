// The selection rectangle: where its handles are, what a drag does to it, and
// where it is allowed to end up.
//
// All of it is pure geometry over plain numbers, deliberately. The pointer
// plumbing in the page is untestable without a browser; this is the part that
// decides where the user's rectangle actually lands, and it is the part that
// silently rescans the wrong region when it is wrong. So it lives here with a
// test that runs under node in CI.
//
// Every rectangle here is in DISPLAY pixels, the canvas the user is dragging
// on. Turning one into source pixels is coords.js's job, and the two are kept
// apart so that neither has to know how the other scales.

(function (global) {
  "use strict";

  // A selection smaller than this is almost certainly a stray click rather
  // than an intention, and a zero-area crop decodes nothing while looking like
  // a decode failure.
  const MIN = 8;

  // How close a pointer has to be to a handle to grab it. Generous, because
  // this has to work with a fingertip on a phone, where the "handle" the user
  // aims at is roughly a centimetre wide.
  const GRAB = 12;

  const num = (n) => typeof n === "number" && Number.isFinite(n);
  const rectOf = (r) => (r && num(r.x) && num(r.y) && num(r.w) && num(r.h) ? { x: r.x, y: r.y, w: r.w, h: r.h } : null);

  // normalise turns a rectangle dragged up or to the left, which has negative
  // width or height while the pointer is down, into one with positive sides.
  function normalise(r) {
    const b = rectOf(r);
    if (!b) return null;
    return {
      x: b.w < 0 ? b.x + b.w : b.x,
      y: b.h < 0 ? b.y + b.h : b.y,
      w: Math.abs(b.w),
      h: Math.abs(b.h),
    };
  }

  // clamp keeps a selection inside the image and no smaller than MIN. Outside
  // the image is not a region that exists: cropping there reads pixels the
  // source does not have, and the decoder would be handed a band of nothing.
  function clamp(r, bounds) {
    const b = normalise(r), s = rectOf(bounds);
    if (!b || !s) return null;
    const w = Math.min(Math.max(b.w, MIN), s.w);
    const h = Math.min(Math.max(b.h, MIN), s.h);
    return {
      x: Math.min(Math.max(b.x, s.x), s.x + s.w - w),
      y: Math.min(Math.max(b.y, s.y), s.y + s.h - h),
      w, h,
    };
  }

  // The eight corners and edges, as unit positions within the rectangle. Data
  // rather than eight branches, so hit testing and cursor choice read the same
  // table and cannot disagree about where a handle is.
  const HANDLES = [
    { id: "nw", fx: 0, fy: 0, cursor: "nwse-resize" },
    { id: "n", fx: 0.5, fy: 0, cursor: "ns-resize" },
    { id: "ne", fx: 1, fy: 0, cursor: "nesw-resize" },
    { id: "e", fx: 1, fy: 0.5, cursor: "ew-resize" },
    { id: "se", fx: 1, fy: 1, cursor: "nwse-resize" },
    { id: "s", fx: 0.5, fy: 1, cursor: "ns-resize" },
    { id: "sw", fx: 0, fy: 1, cursor: "nesw-resize" },
    { id: "w", fx: 0, fy: 0.5, cursor: "ew-resize" },
  ];

  function handlePoints(r) {
    const b = normalise(r);
    if (!b) return [];
    return HANDLES.map((h) => ({ ...h, x: b.x + b.w * h.fx, y: b.y + b.h * h.fy }));
  }

  // hit says what the pointer grabbed: a handle id, "move" when it is inside
  // the rectangle, or null when it is outside and a fresh rectangle should be
  // started instead.
  //
  // Handles win over the interior, and corners win over edges, because a
  // corner sits on top of two edges and the user aiming at it means the corner.
  function hit(r, px, py, grab) {
    const b = normalise(r);
    if (!b || !num(px) || !num(py)) return null;
    const tol = num(grab) ? grab : GRAB;
    let best = null, bestD = Infinity;
    for (const h of handlePoints(b)) {
      const d = Math.hypot(px - h.x, py - h.y);
      const corner = h.id.length === 2;
      // A corner within tolerance beats an edge at the same distance.
      const score = corner ? d - 0.001 : d;
      if (d <= tol && score < bestD) { best = h.id; bestD = score; }
    }
    if (best) return best;
    if (px >= b.x && px <= b.x + b.w && py >= b.y && py <= b.y + b.h) return "move";
    return null;
  }

  // resize applies a drag of one handle. Dragging a side past its opposite
  // flips the rectangle rather than inverting it, which is what every drawing
  // tool does and what a user expects when they overshoot.
  function resize(r, handle, dx, dy, bounds) {
    const b = normalise(r);
    if (!b || !num(dx) || !num(dy)) return null;
    let { x, y, w, h } = b;
    if (handle.includes("w")) { x += dx; w -= dx; }
    if (handle.includes("e")) { w += dx; }
    if (handle.includes("n")) { y += dy; h -= dy; }
    if (handle.includes("s")) { h += dy; }
    return clamp({ x, y, w, h }, bounds);
  }

  function move(r, dx, dy, bounds) {
    const b = normalise(r);
    if (!b || !num(dx) || !num(dy)) return null;
    return clamp({ x: b.x + dx, y: b.y + dy, w: b.w, h: b.h }, bounds);
  }

  // initial is the rectangle the panel opens with.
  //
  // When the decoder located a code it could not read, that box is the initial
  // selection and the user adjusts it. That is the single highest-leverage
  // detail in this whole feature: it turns the interaction from "find the code
  // and box it" into "nudge this box", which is roughly the difference between
  // something people use and something they try once.
  //
  // With no box, an image where nothing was found at all, which is exactly the
  // case where a person can see a code the tool cannot, it opens with a
  // centred rectangle over the middle 60%, as a thing to drag rather than a
  // guess about where the code is.
  function initial(box, bounds) {
    const s = rectOf(bounds);
    if (!s) return null;
    const b = rectOf(box);
    if (b) return clamp(b, s);
    return clamp({ x: s.x + s.w * 0.2, y: s.y + s.h * 0.2, w: s.w * 0.6, h: s.h * 0.6 }, s);
  }

  function selfTest() {
    const eq = (a, b, what) => {
      for (const k of ["x", "y", "w", "h"]) {
        if (Math.abs(a[k] - b[k]) > 1e-9) throw new Error(`${what}: ${JSON.stringify(a)} vs ${JSON.stringify(b)}`);
      }
    };
    const img = { x: 0, y: 0, w: 400, h: 300 };

    // A rectangle dragged up and to the left is still a rectangle.
    eq(normalise({ x: 100, y: 100, w: -40, h: -30 }), { x: 60, y: 70, w: 40, h: 30 }, "normalise");

    // Nothing may leave the image: a crop outside it reads pixels the source
    // does not have.
    const out = clamp({ x: -50, y: -50, w: 100, h: 100 }, img);
    if (out.x < 0 || out.y < 0) throw new Error("clamp let the selection out the top left");
    const big = clamp({ x: 350, y: 250, w: 500, h: 500 }, img);
    if (big.x + big.w > 400.000001 || big.y + big.h > 300.000001) throw new Error("clamp let the selection out the bottom right");
    if (clamp({ x: 10, y: 10, w: 1, h: 1 }, img).w < 8) throw new Error("clamp allowed a selection too small to be an intention");

    // Handles: corners beat edges, the interior is a move, outside is nothing.
    const sel = { x: 100, y: 100, w: 200, h: 100 };
    if (hit(sel, 100, 100) !== "nw") throw new Error("top-left corner not grabbed");
    if (hit(sel, 300, 200) !== "se") throw new Error("bottom-right corner not grabbed");
    if (hit(sel, 200, 100) !== "n") throw new Error("top edge not grabbed");
    if (hit(sel, 200, 150) !== "move") throw new Error("interior is not a move");
    if (hit(sel, 20, 20) !== null) throw new Error("a point outside grabbed something");
    // At a corner the corner wins, even though two edges are also in range.
    if (hit(sel, 102, 102) !== "nw") throw new Error("an edge stole a corner grab");

    // Resizing from each side moves that side and leaves the opposite alone.
    eq(resize(sel, "e", 50, 0, img), { x: 100, y: 100, w: 250, h: 100 }, "resize east");
    eq(resize(sel, "w", 50, 0, img), { x: 150, y: 100, w: 150, h: 100 }, "resize west");
    eq(resize(sel, "n", 20, 20, img), { x: 100, y: 120, w: 200, h: 80 }, "resize north");
    eq(resize(sel, "se", 50, 50, img), { x: 100, y: 100, w: 250, h: 150 }, "resize southeast");

    // Overshooting a side past its opposite flips rather than inverting.
    const flipped = resize(sel, "e", -300, 0, img);
    if (flipped.w < 8) throw new Error("overshoot produced a degenerate rectangle");
    if (flipped.x + flipped.w > 300.000001) throw new Error("a flipped rectangle escaped its own bounds");

    // Moving stops at the edge instead of sliding out.
    eq(move(sel, 1000, 0, img), { x: 200, y: 100, w: 200, h: 100 }, "move clamped east");
    eq(move(sel, -1000, -1000, img), { x: 0, y: 0, w: 200, h: 100 }, "move clamped to origin");

    // The pre-drawn box is the initial selection. This is the whole point of
    // carrying the geometry through four layers to get here.
    eq(initial({ x: 50, y: 40, w: 120, h: 90 }, img), { x: 50, y: 40, w: 120, h: 90 }, "initial from a detection");
    // With no box it opens centred rather than guessing.
    const mid = initial(null, img);
    eq(mid, { x: 80, y: 60, w: 240, h: 180 }, "initial with no detection");
    // A box bigger than the image is brought inside it.
    const huge = initial({ x: -10, y: -10, w: 900, h: 900 }, img);
    if (huge.w > 400.000001 || huge.h > 300.000001) throw new Error("initial let an oversized box through");

    // Totality: nothing returns NaN or throws on rubbish.
    for (const bad of [null, undefined, {}, { x: 1 }, { x: NaN, y: 1, w: 1, h: 1 }]) {
      if (normalise(bad) !== null) throw new Error("normalise accepted rubbish");
      if (clamp(bad, img) !== null) throw new Error("clamp accepted rubbish");
      if (hit(bad, 1, 1) !== null) throw new Error("hit accepted rubbish");
    }
    if (clamp(sel, null) !== null) throw new Error("clamp accepted a bounds that is not a space");

    return "select self-test passed (handles, clamping, flip, pre-drawn initial)";
  }

  global.Select = { normalise, clamp, hit, resize, move, initial, handlePoints, HANDLES, MIN, GRAB, selfTest };

  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
