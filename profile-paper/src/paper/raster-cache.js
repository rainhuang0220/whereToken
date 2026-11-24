// Resources belong to the cache only after admission. A caller still owns
// any oversized resource and can use it for the current revision.
export class RasterCache {
  constructor(maxBytes, maxEntries, dispose) {
    this.maxBytes = maxBytes;
    this.maxEntries = maxEntries;
    this.dispose = dispose;
    this.entries = new Map();
    this.bytes = 0;
  }

  has(key) { return this.entries.has(key); }
  owns(key, resource) { return this.entries.get(key)?.resource === resource; }
  keys() { return this.entries.keys(); }

  get(key) {
    const entry = this.entries.get(key);
    if (!entry) return undefined;
    this.entries.delete(key);
    this.entries.set(key, entry);
    return entry.resource;
  }

  put(key, resource, bytes) {
    const previous = this.entries.get(key);
    if (previous?.resource === resource) {
      this.entries.delete(key);
      this.bytes -= previous.bytes;
    }
    if (bytes > this.maxBytes) return false;
    this.delete(key);
    this.entries.set(key, { resource, bytes });
    this.bytes += bytes;
    while (this.bytes > this.maxBytes || this.entries.size > this.maxEntries) {
      this.delete(this.keys().next().value);
    }
    return true;
  }

  delete(key) {
    const entry = this.entries.get(key);
    if (!entry) return;
    this.entries.delete(key);
    this.bytes -= entry.bytes;
    this.dispose(entry.resource);
  }

  clear() {
    for (const key of this.keys()) this.delete(key);
  }
}
