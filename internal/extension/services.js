// Compatibility adapters belong to installed plugins. The core owns only their
// grant-aware lifecycle; no site names, native hosts or business scripts live here.
(() => {
'use strict';
const services = new Map();
let owner = null, installed = [], pending = Promise.resolve();
const packages = globalThis.BridgePluginPackages || [];
function allowed(service) {
 const site = installed.find(site => site.id === service.plugin);
 return site && site.available !== false && !site.disabled && [site.origin,...(site.origins || [])].some(origin => BridgeSiteConfig.pattern(origin) === service.origin);
}
async function reconcile() {
 const previous=(await chrome.storage.local.get('babServiceIDs')).babServiceIDs || [];
 const stale=previous.filter(id=>typeof id==='string' && !services.has(id));
 if(stale.length){const rows=await chrome.scripting.getRegisteredContentScripts({ids:stale});if(rows.length)await chrome.scripting.unregisterContentScripts({ids:rows.map(row=>row.id)});}
 await chrome.storage.local.set({babServiceIDs:[...services.keys()]});
 for (const service of services.values()) {
  const registered = await chrome.scripting.getRegisteredContentScripts({ids:[service.id]});
  if (registered.length) await chrome.scripting.unregisterContentScripts({ids:[service.id]});
  if (!allowed(service) || !await chrome.permissions.contains({origins:[service.origin]})) {
   await service.stop(); continue;
  }
  if (service.contentScript) await chrome.scripting.registerContentScripts([{
   id:service.id, matches:service.contentScript.matches, js:service.contentScript.js,
   runAt:service.contentScript.runAt || 'document_start', persistAcrossSessions:true,
  }]);
  await service.start();
 }
}
function sync() { pending = pending.catch(() => {}).then(reconcile); return pending; }
globalThis.BrowserAuthBridge = Object.freeze({
 registerService(service) {
  if (!owner || !service || !/^[a-z][a-z0-9-]{0,47}$/.test(service.id) || services.has(service.id) ||
      !owner.origins.some(origin => BridgeSiteConfig.pattern(origin) === service.origin) ||
      typeof service.start !== 'function' || typeof service.stop !== 'function') throw new Error('invalid_plugin_service');
  if (service.contentScript) {
   const spec=service.contentScript;
   if (!Array.isArray(spec.matches) || !spec.matches.length || spec.matches.some(match=>!match.startsWith(service.origin.slice(0,-1))) ||
       !Array.isArray(spec.js) || !spec.js.length || spec.js.some(path=>path!=='stream-transport.js' && !owner.files.includes(path))) throw new Error('invalid_plugin_content_script');
  }
  services.set(service.id,{...service,plugin:owner.id});
 },
 site(id) { return installed.find(site=>site.id===id) || null; },
 setCatalog(sites) { installed=sites; return Promise.all([sync(),globalThis.BridgeSyncProviders?.()]); },
});
for (const definition of packages) {
 owner=definition;
 try { if(definition.background.length) importScripts(...definition.background); }
 finally { owner=null; }
}
chrome.permissions.onAdded.addListener(()=>{void sync().catch(()=>{});});
chrome.permissions.onRemoved.addListener(()=>{void sync().catch(()=>{});});
// No service starts until the native host supplies the installed registry.
void sync().catch(()=>{});
})();
