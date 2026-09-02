// One decoding engine, off the main thread.
//
// The split of labour matters: the browser turns a file into pixels, because it
// already ships native decoders for everything it can display (JPEG, PNG, WebP,
// AVIF, GIF — and HEIC on Safari/macOS), and Go's wasm build does the part the
// browser has no answer for, which is finding and reading the codes.
//
// Nothing here uploads anything. The file never leaves the tab.

importScripts("wasm_exec.js", "result.js");

// A photo straight off a phone can be 12 megapixels, and the cost is almost
// entirely in the pixels: measured on a 3024x4032 photo, decoding at a 3000 px
// cap took 3826 ms and at 1600 px took 1287 ms — three times faster, same code
// found. So the first pass is deliberately small.
//
// It is a first pass rather than the only one because failure is the expensive
// case: when nothing is found the decoder has no candidate count to stop at and
// runs every strategy. A photo whose code really does need the detail would be
// lost at 1600, so one retry at full size follows a fruitless first pass. That
// costs a code-less large photo about half a second more and makes every photo
// that does carry a code three times quicker, which is the trade a folder of
// receipts wants.
const MAX_FAST = 1600;
const MAX_SIDE = 3000;
const THUMB = 72;

let ready = false;
const pending = [];

self.qrWasmReady = () => {
  ready = true;
  for (const job of pending.splice(0)) ((job && HANDLERS[job.type]) || run)(job);
  self.postMessage({ type: "ready" });
};

// The module arrives already compiled, from the page. Fetching it here would
// mean one download per worker: six workers pulling the same 4.6 MB is the
// difference between a page that is ready at once and one that stalls on
// first load.
async function boot(module) {
  try {
    const go = new Go();
    const instance = await WebAssembly.instantiate(module, go.importObject);
    go.run(instance); // returns only when the module exits, which it never does
  } catch (e) {
    self.postMessage({ type: "fatal", error: String((e && e.message) || e) });
  }
}

async function pixelsOf(blob, cap) {
  // Ask for the decode at a bounded size in one step where possible: the
  // browser scales during decode, which is cheaper than decoding then resizing.
  let bmp = await createImageBitmap(blob);
  // The file's own size, before any cap. Kept because the geometry the decoder
  // returns is in the coordinates of whatever it was handed, and only this
  // function still knows what that was relative to the file.
  const natural = { w: bmp.width, h: bmp.height };
  const limit = cap || MAX_SIDE;
  let shrunk = false;
  if (Math.max(bmp.width, bmp.height) > limit) {
    shrunk = true;
    const scale = limit / Math.max(bmp.width, bmp.height);
    const scaled = await createImageBitmap(bmp, {
      resizeWidth: Math.round(bmp.width * scale),
      resizeHeight: Math.round(bmp.height * scale),
      resizeQuality: "high",
    });
    bmp.close();
    bmp = scaled;
  }

  const canvas = new OffscreenCanvas(bmp.width, bmp.height);
  const ctx = canvas.getContext("2d", { willReadFrequently: true });
  ctx.drawImage(bmp, 0, 0);
  return { data: ctx.getImageData(0, 0, bmp.width, bmp.height), bmp, shrunk, natural };
}

async function thumbnailOf(bmp) {
  const scale = THUMB / Math.max(bmp.width, bmp.height);
  const w = Math.max(1, Math.round(bmp.width * scale));
  const h = Math.max(1, Math.round(bmp.height * scale));
  const canvas = new OffscreenCanvas(w, h);
  canvas.getContext("2d").drawImage(bmp, 0, 0, w, h);
  return canvas.convertToBlob({ type: "image/jpeg", quality: 0.7 });
}

async function run(job) {
  const out = { type: "result", id: job.id, name: job.name, path: job.path, codes: [] };
  let bmp = null;
  try {
    let pass = await pixelsOf(job.blob, job.cap || MAX_FAST);
    bmp = pass.bmp;
    let res = qrDecode(pass.data.width, pass.data.height, pass.data.data);

    // Only a picture that was actually shrunk can have lost something worth
    // looking at again.
    if (!job.cap && !(res.codes || []).length && pass.shrunk) {
      const full = await pixelsOf(job.blob, MAX_SIDE);
      bmp.close();
      pass = full;
      bmp = full.bmp;
      res = qrDecode(full.data.width, full.data.height, full.data.data);
    }

    if (res.error) out.error = res.error;
    // Everything the engine found, including the geometry of the codes it
    // located and could not read. The mapping is a named function with a test
    // because this is exactly where the boxes used to be dropped - see
    // result.js. The frame goes with them: a box means nothing without the
    // pixel dimensions it was measured in, and after a retry those are the
    // second pass's, not the first's.
    Object.assign(out, Result.carry(res, {
      w: pass.data.width,
      h: pass.data.height,
      naturalW: pass.natural.w,
      naturalH: pass.natural.h,
    }));

    out.thumb = await thumbnailOf(bmp);
  } catch (e) {
    // Two different things land here and the message must not pick one: a
    // format this browser has no decoder for (HEIC outside Safari, a camera
    // raw), or a file that is damaged. A strict decoder rejects a truncated
    // PNG that a lenient one would have read, so "unsupported" alone would be
    // a guess.
    // The sentence is named, not written: a worker has no string table, and
    // hardcoding English here left this one line untranslated in every locale
    // while the advice under it translated.
    //
    // The rule, and the reason these two fields both exist: the UI localises,
    // the export does not. errorKey is what a person reads, so it follows their
    // language; error is what the CSV and JSON exports carry, so it stays this
    // one English string that a script can match on and that means the same
    // thing to whoever opens the file next.
    out.errorKey = "err.unopenable";
    out.error = "could not be read — unsupported format, or a damaged file";
    out.detail = String((e && e.message) || e);
  } finally {
    if (bmp) bmp.close();
  }
  self.postMessage(out);
}

// crop re-reads one region the user pointed at, at the source's own
// resolution.
//
// No cap, unlike the first pass. The caps exist because most images in a folder
// hold nothing and paying full price for all of them makes the page feel slow.
// This is the opposite case: the user has looked at the picture, decided there
// is a code in that rectangle, and spent their attention saying so. Spending a
// second of ours in return is the trade they already made.
//
// The rectangle arrives in the coordinates of createImageBitmap(blob) — the
// natural, orientation-applied image — and is handed straight to
// createImageBitmap's own crop arguments, which are in that same space. No
// rotation term, for the reason in coords.js: both sides inherit whatever the
// browser decided about EXIF.
async function crop(job) {
  const out = { type: "cropResult", id: job.id, codes: [] };
  let bmp = null, cut = null;
  try {
    bmp = await createImageBitmap(job.blob);
    const r = job.rect;
    // Clamp again here rather than trusting the caller. The page clamps to the
    // image it drew, this clamps to the image it decoded, and a disagreement
    // between the two would be a crop of a region that does not exist.
    const x = Math.max(0, Math.min(Math.round(r.x), bmp.width - 1));
    const y = Math.max(0, Math.min(Math.round(r.y), bmp.height - 1));
    const w = Math.max(1, Math.min(Math.round(r.w), bmp.width - x));
    const h = Math.max(1, Math.min(Math.round(r.h), bmp.height - y));

    cut = await createImageBitmap(bmp, x, y, w, h);
    const canvas = new OffscreenCanvas(cut.width, cut.height);
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    ctx.drawImage(cut, 0, 0);
    const data = ctx.getImageData(0, 0, cut.width, cut.height);

    const res = qrDecode(data.width, data.height, data.data);
    if (res.error) out.error = res.error;
    Object.assign(out, Result.carry(res, {
      w: data.width, h: data.height, naturalW: data.width, naturalH: data.height,
    }));
    // Where the crop sat in the source, so a caller can put its geometry back
    // where it came from without remembering what it asked for.
    out.origin = { x, y, w, h };
  } catch (e) {
    out.errorKey = "err.unopenable";
    out.error = "could not be read — unsupported format, or a damaged file";
    out.detail = String((e && e.message) || e);
  } finally {
    if (cut) cut.close();
    if (bmp) bmp.close();
  }
  self.postMessage(out);
}

// encode is the other direction: payloads back into module matrices, so the
// page can write them out as PNG, JPEG, SVG or PDF.
//
// It lives here for the same reason decoding does - the engine is in the
// worker, and the page has no wasm instance of its own. Routing it through the
// same queue also keeps a batch of a hundred codes off the thread that has to
// keep the UI moving.
function encodeBatch(job) {
  const matrices = (job.texts || []).map((t) => {
    try {
      const m = qrEncode(t);
      return m && !m.error ? m : null;
    } catch {
      return null;
    }
  });
  self.postMessage({ type: "encodeResult", id: job.id, matrices });
}

const HANDLERS = { crop, encode: encodeBatch };

self.onmessage = (e) => {
  const job = e.data;
  if (job && job.type === "module") { boot(job.module); return; }
  const go = (job && HANDLERS[job.type]) || run;
  if (ready) go(job);
  else pending.push(job);
};
