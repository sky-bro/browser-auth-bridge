"use strict";
// Only origins are user-configurable. Protocol scripts and native host names
// always come from the packaged provider catalog.
globalThis.BridgeSiteConfig = Object.freeze({
 key: "bridgeSites",
 normalize(value) {
  if (typeof value !== "string") throw new Error("Enter a site URL");
  const url = new URL(value.trim());
  const loopback = url.hostname === "localhost" || url.hostname === "[::1]" || /^127(?:\.\d{1,3}){3}$/.test(url.hostname);
  if (url.hostname.includes("*") || url.username || url.password || url.pathname !== "/" || url.search || url.hash ||
      (url.protocol !== "https:" && !(url.protocol === "http:" && loopback))) {
   throw new Error("Use an HTTPS site origin without a path, credentials, query or fragment (HTTP is allowed on loopback)");
  }
  return url.origin;
 },
 pattern(origin) {
  const url = new URL(this.normalize(origin));
  return `${url.protocol}//${url.hostname}/*`;
 },
 origin(provider, sites) {
  // Fixed providers retain their packaged scope. Stored data cannot change an
  // integration's host identity, adapter, scripts, or fixed-origin policy.
  const value = provider.editableOrigin ? (Object.hasOwn(sites, provider.id) ? sites[provider.id] : provider.origin) : provider.origin;
  return value ? this.normalize(value) : null;
 },
 async read() {
  if (!chrome.storage?.local) {
   let version = "unknown";
   try { version = chrome.runtime.getManifest().version; } catch (_) {}
   throw new Error(`Storage permission is unavailable (loaded version ${version}). Reload Browser Auth Bridge on Chrome's extensions page to apply the updated manifest, then reopen this popup.`);
  }
  const data = await chrome.storage.local.get(this.key);
  const sites = data[this.key];
  return sites && typeof sites === "object" && !Array.isArray(sites) ? sites : {};
 },
 async remove(id) {
  const sites=await this.read();delete sites[id];await chrome.storage.local.set({[this.key]:sites});
 },
 async save(id, origin) {
  const sites = await this.read();
  await chrome.storage.local.set({[this.key]: {...sites, [id]: this.normalize(origin)}});
 },
});
