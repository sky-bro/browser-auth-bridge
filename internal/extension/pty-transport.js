// Adapter identity and scripts come from the installed provider configuration.
// A socket payload can select an installed adapter, never supply executable code.
globalThis.PTYTransport = Object.freeze({
  validateEndpoint(endpoint, origin, adapter) {
    if (!endpoint || endpoint.adapter !== adapter || !/^[a-z][a-z0-9-]{0,31}$/.test(adapter) || endpoint.origin !== origin) return false;
    try {
      const base = new URL(origin);
      if (base.origin !== origin || !(base.protocol === 'https:' || base.protocol === 'http:' && ['127.0.0.1', 'localhost', '[::1]'].includes(base.hostname))) return false;
      return [[endpoint.pageUrl, base.protocol], [endpoint.socketUrl, base.protocol === 'https:' ? 'wss:' : 'ws:']].every(([value, protocol]) => {
        if (typeof value !== 'string' || value.length > 8192) return false;
        const url = new URL(value);
        return url.protocol === protocol && url.host === base.host && !url.username && !url.password && !url.hash;
      });
    } catch (_) { return false; }
  },
});
