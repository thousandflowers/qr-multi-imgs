// A minimal ZIP writer, store-only.
//
// Store-only is not a shortcut here, it is the right call: the payload is
// JPEGs and PNGs, which are already compressed, so deflating them costs time
// and saves nothing. It also means no library, which matters because the
// page's Content-Security-Policy forbids fetching one.
//
// ponytail: no deflate, no zip64. Add zip64 if anyone ever needs an archive
// past 4 GB or 65535 files, which is not what a folder of receipts is.

(function (global) {
  "use strict";

  const CRC = (() => {
    const t = new Uint32Array(256);
    for (let i = 0; i < 256; i++) {
      let c = i;
      for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      t[i] = c >>> 0;
    }
    return t;
  })();

  function crc32(bytes) {
    let c = 0xffffffff;
    for (let i = 0; i < bytes.length; i++) c = CRC[(c ^ bytes[i]) & 0xff] ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
  }

  // MS-DOS packed date and time, which is all a ZIP header can carry.
  function dosStamp(d) {
    const time = (d.getHours() << 11) | (d.getMinutes() << 5) | (d.getSeconds() >> 1);
    const date = ((d.getFullYear() - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate();
    return { time, date };
  }

  // Names are stored as UTF-8 with bit 11 set, so an accented filename arrives
  // intact instead of as mojibake on the other side.
  const utf8 = (s) => new TextEncoder().encode(s);

  function build(entries, when) {
    const stamp = dosStamp(when || new Date());
    const parts = [];
    const central = [];
    let offset = 0;

    for (const e of entries) {
      const name = utf8(e.name);
      const data = e.data instanceof Uint8Array ? e.data : new Uint8Array(e.data);
      const sum = crc32(data);

      const local = new DataView(new ArrayBuffer(30));
      local.setUint32(0, 0x04034b50, true);
      local.setUint16(4, 20, true);          // version needed
      local.setUint16(6, 0x0800, true);      // bit 11: the name is UTF-8
      local.setUint16(8, 0, true);           // stored, not deflated
      local.setUint16(10, stamp.time, true);
      local.setUint16(12, stamp.date, true);
      local.setUint32(14, sum, true);
      local.setUint32(18, data.length, true);
      local.setUint32(22, data.length, true);
      local.setUint16(26, name.length, true);
      local.setUint16(28, 0, true);
      parts.push(new Uint8Array(local.buffer), name, data);

      const dir = new DataView(new ArrayBuffer(46));
      dir.setUint32(0, 0x02014b50, true);
      dir.setUint16(4, 20, true);            // version made by
      dir.setUint16(6, 20, true);            // version needed
      dir.setUint16(8, 0x0800, true);
      dir.setUint16(10, 0, true);
      dir.setUint16(12, stamp.time, true);
      dir.setUint16(14, stamp.date, true);
      dir.setUint32(16, sum, true);
      dir.setUint32(20, data.length, true);
      dir.setUint32(24, data.length, true);
      dir.setUint16(28, name.length, true);
      dir.setUint32(42, offset, true);
      central.push(new Uint8Array(dir.buffer), name);

      offset += 30 + name.length + data.length;
    }

    let centralSize = 0;
    for (const c of central) centralSize += c.length;

    const end = new DataView(new ArrayBuffer(22));
    end.setUint32(0, 0x06054b50, true);
    end.setUint16(8, entries.length, true);
    end.setUint16(10, entries.length, true);
    end.setUint32(12, centralSize, true);
    end.setUint32(16, offset, true);

    return new Blob([...parts, ...central, new Uint8Array(end.buffer)],
                    { type: "application/zip" });
  }

  // Two files may well want the same name once names come from QR contents.
  // The archive must not silently keep only one of them.
  function uniqueName(taken, name) {
    if (!taken.has(name)) { taken.add(name); return name; }
    const dot = name.lastIndexOf(".");
    const stem = dot > 0 ? name.slice(0, dot) : name;
    const ext = dot > 0 ? name.slice(dot) : "";
    for (let n = 2; ; n++) {
      const candidate = stem + "-" + n + ext;
      if (!taken.has(candidate)) { taken.add(candidate); return candidate; }
    }
  }

  // Strip whatever a file system or another operating system would choke on,
  // and the control characters a QR payload can legally contain.
  function safeName(s, maxLen) {
    const cleaned = String(s == null ? "" : s)
      .replace(/[\u0000-\u001f\u007f]/g, " ")
      .replace(/[\/\\?%*:|"<>]/g, "-")
      .replace(/\s+/g, " ")
      .replace(/^[.\s-]+/, "")
      .replace(/[.\s]+$/, "")
      .slice(0, maxLen || 80);
    return cleaned || "senza-nome";
  }

  function selfTest() {
    const eq = (got, want, what) => {
      if (got !== want) {
        throw new Error(what + ": got " + JSON.stringify(got) + ", want " + JSON.stringify(want));
      }
    };
    // A known vector: CRC32 of "123456789" is 0xCBF43926.
    eq(crc32(new TextEncoder().encode("123456789")), 0xcbf43926, "crc32");

    eq(safeName("a/b:c*?.png"), "a-b-c--.png", "unsafe characters become dashes");
    eq(safeName("   "), "senza-nome", "an empty name still gets one");
    eq(safeName("x".repeat(200)).length, 80, "names are bounded");
    eq(safeName("line\nbreak.png"), "line break.png", "control characters go");

    const taken = new Set();
    eq(["x.png", "x.png", "x.png"].map((n) => uniqueName(taken, n)).join(" "),
       "x.png x-2.png x-3.png", "collisions are numbered, not dropped");
    return "zip self-test passed";
  }

  global.Zip = { build, uniqueName, safeName, crc32, selfTest };

  // Running the file directly is the test.
  if (typeof process !== "undefined" && process.argv && process.argv.includes("--test")) {
    console.log(selfTest());
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
