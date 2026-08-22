// One decoding engine, off the main thread.
//
// The split of labour matters: the browser turns a file into pixels, because it
// already ships native decoders for everything it can display (JPEG, PNG, WebP,
// AVIF, GIF — and HEIC on Safari/macOS), and Go's wasm build does the part the
// browser has no answer for, which is finding and reading the codes.
//
// Nothing here uploads anything. The file never leaves the tab.

importScripts("wasm_exec.js");

// A photo straight off a phone can be 12 megapixels. Decoding that costs 48 MB
// of RGBA to copy into wasm for detail no QR needs, so cap the long side. A
// code has to survive being about 2 px per module, and 3000 px leaves far more
// than that for anything a camera actually framed.
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

async function pixelsOf(blob) {
  // Ask for the decode at a bounded size in one step where possible: the
  // browser scales during decode, which is cheaper than decoding then resizing.
  let bmp = await createImageBitmap(blob);
  if (Math.max(bmp.width, bmp.height) > MAX_SIDE) {
    const scale = MAX_SIDE / Math.max(bmp.width, bmp.height);
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
  return { data: ctx.getImageData(0, 0, bmp.width, bmp.height), bmp };
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
    const px = await pixelsOf(job.blob);
    bmp = px.bmp;
    const res = qrDecode(px.data.width, px.data.height, px.data.data);
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
