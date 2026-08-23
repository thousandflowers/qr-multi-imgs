// One decoding engine, off the main thread.
//
// The split of labour matters: the browser turns a file into pixels, because it
// already ships native decoders for everything it can display (JPEG, PNG, WebP,
// AVIF, GIF — and HEIC on Safari/macOS), and Go's wasm build does the part the
// browser has no answer for, which is finding and reading the codes.
//
// Nothing here uploads anything. The file never leaves the tab.

importScripts("wasm_exec.js");

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
  for (const job of pending.splice(0)) run(job);
  self.postMessage({ type: "ready" });
};

(async () => {
  try {
    const go = new Go();
    // Not instantiateStreaming: a host that serves .wasm with the wrong
    // Content-Type would reject the stream, and this must not depend on that.
    const bytes = await (await fetch("qr.wasm")).arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance); // returns only when the module exits, which it never does
  } catch (e) {
    self.postMessage({ type: "fatal", error: String((e && e.message) || e) });
  }
})();

async function pixelsOf(blob, cap) {
  // Ask for the decode at a bounded size in one step where possible: the
  // browser scales during decode, which is cheaper than decoding then resizing.
  let bmp = await createImageBitmap(blob);
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
  return { data: ctx.getImageData(0, 0, bmp.width, bmp.height), bmp, shrunk };
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
    const fast = await pixelsOf(job.blob, job.cap || MAX_FAST);
    bmp = fast.bmp;
    let res = qrDecode(fast.data.width, fast.data.height, fast.data.data);

    // Only a picture that was actually shrunk can have lost something worth
    // looking at again.
    if (!job.cap && !(res.codes || []).length && fast.shrunk) {
      const full = await pixelsOf(job.blob, MAX_SIDE);
      bmp.close();
      bmp = full.bmp;
      res = qrDecode(full.data.width, full.data.height, full.data.data);
    }

    if (res.error) out.error = res.error;
    out.codes = res.codes || [];
    out.thumb = await thumbnailOf(bmp);
  } catch (e) {
    // Two different things land here and the message must not pick one: a
    // format this browser has no decoder for (HEIC outside Safari, a camera
    // raw), or a file that is damaged. A strict decoder rejects a truncated
    // PNG that a lenient one would have read, so "unsupported" alone would be
    // a guess.
    out.error = "could not be read — unsupported format, or a damaged file";
    out.detail = String((e && e.message) || e);
  } finally {
    if (bmp) bmp.close();
  }
  self.postMessage(out);
}

self.onmessage = (e) => {
  const job = e.data;
  if (ready) run(job);
  else pending.push(job);
};
