// Read-only, memory-only disposable-secret audit. Never return storage contents,
// names, values or errors. Unrelated browser data is allowed and left untouched.
function secretNeedles(values: string[]) {
  return values.flatMap((v) => {
    const variants = [v, JSON.stringify(v), encodeURIComponent(v)];
    if (v.startsWith("Bearer ")) variants.push(v.slice(7));
    try {
      const scalar: unknown = JSON.parse(v);
      if (typeof scalar === "string") variants.push(scalar);
    } catch {
      /* ordinary text */
    }
    return variants.filter(Boolean);
  });
}
export function containsSecretText(text: string, values: string[]) {
  return secretNeedles(values).some((v) => text.includes(v));
}
export async function auditSecretPersistence(values: string[]) {
  const needles = secretNeedles(values);
  let safe = true,
    complete = true,
    bytes = 0,
    records = 0;
  const counts = {
    localEntries: 0,
    sessionEntries: 0,
    databases: 0,
    indexedRecords: 0,
    cacheEntries: 0,
  };
  const inspectText = (text: string) => {
    bytes += new TextEncoder().encode(text).length;
    if (bytes > 16 * 1024 * 1024) throw new Error("Audit bound exceeded");
    if (needles.some((v) => text.includes(v))) safe = false;
  };
  const inspect = async (
    value: unknown,
    seen = new Set<object>(),
  ): Promise<void> => {
    if (typeof value === "string") return inspectText(value);
    if (value instanceof Blob) {
      if (value.size > 16 * 1024 * 1024 - bytes)
        throw new Error("Audit bound exceeded");
      return inspectText(await value.text());
    }
    if (value instanceof ArrayBuffer || ArrayBuffer.isView(value)) {
      const view =
        value instanceof ArrayBuffer
          ? new Uint8Array(value)
          : new Uint8Array(value.buffer, value.byteOffset, value.byteLength);
      if (view.byteLength > 16 * 1024 * 1024 - bytes)
        throw new Error("Audit bound exceeded");
      return inspectText(new TextDecoder().decode(view));
    }
    if (value && typeof value === "object" && !seen.has(value)) {
      seen.add(value);
      if (value instanceof Map)
        for (const [k, v] of value) {
          await inspect(k, seen);
          await inspect(v, seen);
        }
      else if (value instanceof Set)
        for (const v of value) await inspect(v, seen);
      else
        for (const [k, v] of Object.entries(value)) {
          inspectText(k);
          await inspect(v, seen);
        }
    }
  };
  try {
    for (const [storage, key] of [
      [localStorage, "localEntries"],
      [sessionStorage, "sessionEntries"],
    ] as const) {
      counts[key] = storage.length;
      for (let i = 0; i < storage.length; i++) {
        const name = storage.key(i);
        if (name !== null) {
          inspectText(name);
          inspectText(storage.getItem(name) ?? "");
        }
      }
    }
    if (typeof indexedDB.databases !== "function")
      throw new Error("Inventory unavailable");
    const databases = await indexedDB.databases();
    counts.databases = databases.length;
    for (const entry of databases) {
      if (!entry.name) throw new Error("Inventory unavailable");
      inspectText(entry.name);
      const db = await new Promise<IDBDatabase>((resolve, reject) => {
        const open = indexedDB.open(entry.name!);
        let failed = false;
        const fail = () => {
          failed = true;
          clearTimeout(timer);
          reject(new Error("Inventory unavailable"));
        };
        const timer = setTimeout(fail, 5000);
        open.onupgradeneeded = () => open.transaction?.abort();
        open.onblocked = open.onerror = fail;
        open.onsuccess = () => {
          clearTimeout(timer);
          if (failed) {
            open.result.close();
            return;
          }
          resolve(open.result);
        };
      });
      try {
        for (const name of db.objectStoreNames) {
          inspectText(name);
          const entries = await new Promise<unknown[]>((resolve, reject) => {
            const out: unknown[] = [];
            const tx = db.transaction(name, "readonly");
            tx.onabort = tx.onerror = () =>
              reject(new Error("Inventory unavailable"));
            tx.oncomplete = () => resolve(out);
            const cursor = tx.objectStore(name).openCursor();
            cursor.onsuccess = () => {
              const row = cursor.result;
              if (!row) return;
              if (++records > 10000) {
                tx.abort();
                return;
              }
              out.push(row.key, row.value);
              counts.indexedRecords++;
              row.continue();
            };
          });
          for (const value of entries) await inspect(value);
        }
      } finally {
        db.close();
      }
    }
    for (const name of await caches.keys()) {
      inspectText(name);
      const cache = await caches.open(name);
      for (const request of await cache.keys()) {
        if (++records > 10000) throw new Error("Audit bound exceeded");
        counts.cacheEntries++;
        inspectText(request.url);
        request.headers.forEach((v, k) => {
          inspectText(k);
          inspectText(v);
        });
        const response = await cache.match(request);
        if (!response || response.type === "opaque")
          throw new Error("Inventory unavailable");
        response.headers.forEach((v, k) => {
          inspectText(k);
          inspectText(v);
        });
        if (response.body) {
          const reader = response.body.getReader();
          const decoder = new TextDecoder();
          let tail = "";
          try {
            while (true) {
              const chunk = await reader.read();
              if (chunk.done) {
                inspectText(tail + decoder.decode());
                break;
              }
              const text = decoder.decode(chunk.value, { stream: true });
              inspectText(tail + text);
              tail = (tail + text).slice(
                -Math.max(...needles.map((v) => v.length), 1),
              );
            }
          } finally {
            await reader.cancel();
          }
        }
      }
    }
  } catch {
    complete = false;
  }
  return { ...counts, storageSafe: safe && complete, auditComplete: complete };
}
