"use strict";
importScripts("config.js", "site-config.js", "browser-exec.js", "stream-transport.js", "pty-transport.js", "services.js");
const sessions = new Map();
const brokers = new Map();
const providers = new Map();
const validID = v => typeof v === "string" && /^[a-f0-9]{64}$/.test(v);
function send(provider, message) { try { brokers.get(provider.id)?.postMessage(message); } catch (_) {} }
async function close(state, notify = true) {
 if (sessions.get(state.id) !== state) return;
 sessions.delete(state.id);
 try { state.port?.postMessage({type:"close"}); state.port?.disconnect(); } catch (_) {}
 if (state.tabOwned) { try { await chrome.tabs.remove(state.tabId); } catch (_) {} }
 if (notify) send(state.provider, {type:"status",session:state.id,state:"closed"});
}
async function attach(provider, m) {
 if (!validID(m.session) || m.service !== provider.id || sessions.has(m.session)) return;
 const state = {id:m.session, provider}; sessions.set(m.session,state);
 const alive = () => sessions.get(state.id) === state;
 try {
  if (m.operation === "browser") {
   const result = await BrowserExec.execute(m.payload,provider.origin,m.allowTab === true && !m.noOpenTab,alive,"native:"+provider.id);
   if (alive()) send(provider,{type:"result",session:state.id,result});
   await close(state); return;
  }
  const p = m.payload;
  if (m.operation !== "pty" || p?.version !== 1 || p.provider !== provider.id ||
      typeof p.target !== "string" || p.target.length<1 || p.target.length>256 ||
      !Number.isInteger(p.cols) || p.cols<1 || p.cols>10000 || !Number.isInteger(p.rows) || p.rows<1 || p.rows>10000 ||
      !(p.endpoint ? PTYTransport.validateEndpoint(p.endpoint,provider.origin,provider.ptyAdapter) &&
        !p.recipe :
        StreamTransport.validate(p.recipe,provider.origin))) throw new Error("pty_scope_denied");
  const pageUrl = p.endpoint ? p.endpoint.pageUrl : p.recipe.pageUrl;
  const tabs = await chrome.tabs.query({url:BridgeSiteConfig.pattern(provider.origin)});
  let tab = tabs.find(t=>t.url===pageUrl);
  if (!alive()) return;
  if (!tab) {
   if (!m.allowTab || m.noOpenTab) throw new Error("page_required");
   tab = await chrome.tabs.create({url:pageUrl,active:false});state.tabOwned=true;
  }
  state.tabId=tab.id;
  if (!alive()) {if(state.tabOwned) await chrome.tabs.remove(tab.id);return;}
  let ready=false;
  for (let i=0;i<100;i++) {
   if (!alive()) return;
   const current=await chrome.tabs.get(tab.id);
   if (new URL(current.url).origin!==provider.origin) throw new Error("page_scope_mismatch");
   if(current.status==="complete"){ready=true;break;}
   await new Promise(resolve=>setTimeout(resolve,100));
  }
  if (!ready) throw new Error("page_timeout");
  const scripts = p.endpoint ? provider.ptyScripts : [];
  if (p.endpoint && (!Array.isArray(scripts) || !scripts.length || scripts.some(s=>typeof s!=='string' || !/^[a-zA-Z0-9_/-]+\.js$/.test(s) || s.includes('..')))) throw new Error('adapter_not_installed');
  await chrome.scripting.executeScript({target:{tabId:tab.id},files:["stream-transport.js","pty-transport.js",...scripts,"pty-content.js"]});
  if (!alive()) return;
  const port=chrome.tabs.connect(tab.id,{name:"pty-v1"});state.port=port;
  port.onMessage.addListener(message=>{
   if(!alive())return;
   if(["stdout","header","status","error"].includes(message?.type)) send(provider,{...message,session:state.id});
   if(message?.type==="error" || message?.state==="closed") void close(state);
  });
  port.onDisconnect.addListener(()=>{void close(state);});
  port.postMessage({type:"open",payload:p,adapter:provider.ptyAdapter});
 } catch (_) {
  if(alive()) send(provider,{type:"error",session:state.id,error:"bridge_operation_failed"});
  await close(state);
 }
}
async function sync(provider) {
 const granted = await chrome.permissions.contains({origins:[BridgeSiteConfig.pattern(provider.origin)]});
 if(!granted) {
  const port=brokers.get(provider.id);brokers.delete(provider.id);
  for(const s of Array.from(sessions.values()))if(s.provider===provider)await close(s,false);
  try{port?.disconnect();}catch(_){}
  await BrowserExec.revoke(provider.origin,"native:"+provider.id);return;
 }
 if(brokers.has(provider.id))return;
 const port=chrome.runtime.connectNative(provider.nativeHost);brokers.set(provider.id,port);
 port.onMessage.addListener(m=>{
  if (providers.get(provider.id) !== provider || brokers.get(provider.id) !== port) return;
  if(m.type==="attach"){void attach(provider,m);return;}
  const state=sessions.get(m.session);
  if(!state || state.provider!==provider)return;
  if(m.type==="close"){void close(state);return;}
  if(m.type==="stdin" || m.type==="resize") {try{state.port?.postMessage(m);}catch(_){void close(state);}}
 });
 port.onDisconnect.addListener(()=>{
  if(brokers.get(provider.id)!==port)return;
  brokers.delete(provider.id);
  for(const s of Array.from(sessions.values()))if(s.provider===provider)void close(s,false);
  // No automatic retry loop: reopening the popup grant or restarting Chrome reconnects.
 });
 port.postMessage({type:"broker"});
}
async function stopProvider(provider) {
 const port = brokers.get(provider.id);
 brokers.delete(provider.id);
 for (const state of Array.from(sessions.values())) if (state.provider === provider) await close(state, false);
 try { port?.disconnect(); } catch (_) {}
 await BrowserExec.revoke(provider.origin,"native:"+provider.id);
}
async function reconcile() {
 const sites = await BridgeSiteConfig.read();
 for (const definition of BridgeProviders) {
  let origin = null;
  try {
   const registered=globalThis.BrowserAuthBridge?.site(definition.id);
   origin=definition.registryManaged ? (registered && !registered.disabled && registered.available!==false ? registered.origin : null) : BridgeSiteConfig.origin(definition,sites);
  } catch (_) { /* Fail closed on invalid stored origins. */ }
  const previous = providers.get(definition.id);
  if (previous && previous.origin !== origin) {
   providers.delete(definition.id);
   await stopProvider(previous);
  }
  if (!origin) continue;
  if (!providers.has(definition.id)) providers.set(definition.id, {...definition, origin});
  await sync(providers.get(definition.id));
 }
}
// Serialize permission and configuration changes so a stale origin can never
// overwrite a newer selection or reconnect an obsolete provider.
let pending = Promise.resolve();
function syncAll() {
 pending = pending.catch(() => {}).then(reconcile);
 return pending;
}
chrome.permissions.onAdded.addListener(() => { void syncAll().catch(() => {}); });
chrome.permissions.onRemoved.addListener(() => { void syncAll().catch(() => {}); });
chrome.storage?.onChanged?.addListener((changes, area) => {
 if (area === "local" && (changes[BridgeSiteConfig.key] || changes.babCatalog || changes.babConnected)) void syncAll().catch(() => {});
});
chrome.runtime.onMessage.addListener((message, sender, respond) => {
 if (sender.id !== chrome.runtime.id || message?.type !== "bridge:refresh-sites") return;
 void syncAll().then(() => respond({ok:true}), () => respond({ok:false}));
 return true;
});
chrome.runtime.onStartup.addListener(() => { void syncAll().catch(() => {}); });
chrome.runtime.onInstalled.addListener(() => { void syncAll().catch(() => {}); });
globalThis.BridgeSyncProviders=syncAll;
void syncAll().catch(() => {});

importScripts("bab-background.js");
