// Dynamic BAB providers come only from the installed Native Host. Page scripts
// cannot register plugins, request permissions, or select another host.
(() => {
'use strict';
const host='org.browserauthbridge.bab';
const sessions=new Map();
let port=null, sites=[], catalogReady=false;
const validID=s=>typeof s==='string' && /^[a-f0-9]{64}$/.test(s);
function validCatalog(value) {
 if (!Array.isArray(value) || value.length>100) return false;
 const ids=new Set();
 return value.every(site=>{
  if (!site || Object.keys(site).some(k=>!['id','origin','label','origins','available','disabled'].includes(k)) || !/^[a-z][a-z0-9-]{0,47}$/.test(site.id) || ids.has(site.id) || typeof site.label!=='string' || site.label.length>256) return false;
  try { const u=new URL(site.origin); if(u.origin!==site.origin || /[\\*%\s]/.test(site.origin) || u.username || u.password || (u.protocol!=='https:' && !(u.protocol==='http:' && ['localhost','127.0.0.1','[::1]'].includes(u.hostname)))) return false; } catch (_) { return false; }
  if(site.disabled!==undefined && typeof site.disabled!=='boolean')return false;
  if(site.available!==undefined && typeof site.available!=='boolean')return false;
  if(site.origins!==undefined && (!Array.isArray(site.origins) || site.origins.length>8))return false;
  const origins=[site.origin,...(site.origins||[])];
  if(new Set(origins).size!==origins.length)return false;
  for(const origin of origins){try{const u=new URL(origin);if(u.origin!==origin || /[\\*%\s]/.test(origin) || u.username || u.password || (u.protocol!=='https:' && !(u.protocol==='http:' && ['localhost','127.0.0.1','[::1]'].includes(u.hostname))))return false;}catch(_){return false;}}
  ids.add(site.id); return true;
 });
}
const pattern=origin=>{const u=new URL(origin);return `${u.protocol}//${u.hostname}/*`;};
const granted=origin=>chrome.permissions.contains({origins:[pattern(origin)]});
function send(message) { try {port?.postMessage(message);} catch (_) {} }
function finish(id) { sessions.delete(id); send({type:'status',session:id,state:'closed'}); }
async function setCatalog(value) {
 if (!validCatalog(value)) {
  catalogReady=false;sessions.clear();
  if(globalThis.BrowserAuthBridge)await BrowserAuthBridge.setCatalog([]);
  for(const site of sites)for(const origin of [site.origin,...(site.origins||[])])await BrowserExec.revoke(origin,"bab:"+site.id);
  throw new Error('invalid_catalog');
 }
 const old=sites; sites=value;catalogReady=true;
 if(globalThis.BrowserAuthBridge)await BrowserAuthBridge.setCatalog(sites);
 if(typeof BridgeProviders!=="undefined" && globalThis.BridgeSiteConfig){
  const configured=await BridgeSiteConfig.read();
  for(const definition of BridgeProviders.filter(provider=>provider.editableOrigin)){
   const registered=sites.find(site=>site.id===definition.id);
   if(registered && !registered.disabled && configured[definition.id]!==registered.origin)await BridgeSiteConfig.save(definition.id,registered.origin);
   else if(!registered || registered.disabled)await BridgeSiteConfig.remove(definition.id);
  }
 }
 await chrome.storage.local.set({babCatalog:sites});
 for (const site of old) if (!sites.some(s=>s.id===site.id && !s.disabled && s.origin===site.origin && JSON.stringify(s.origins||[])===JSON.stringify(site.origins||[]))) {
  for (const [id,session] of sessions) if (session.site===site.id) sessions.delete(id);
  for(const origin of [site.origin,...(site.origins||[])])await BrowserExec.revoke(origin,"bab:"+site.id);
 }
}
async function status(site) {
 const selected=site ? sites.filter(s=>s.id===site) : sites;
 return Promise.all(selected.map(async s=>{
  const scopes=await Promise.all([s.origin,...(s.origins||[])].map(async origin=>({origin,authorized:await granted(origin)})));
  return {site:s.id,origin:s.origin,available:s.available!==false,enabled:!s.disabled,authorized:scopes.every(scope=>scope.authorized),scopes,login:'unchecked'};
 }));
}
async function attach(m) {
 if (!validID(m.session) || sessions.has(m.session) || !catalogReady) return;
 const state={site:m.service};sessions.set(m.session,state);
 const alive=()=>sessions.get(m.session)===state;
 try {
  let result;
  if(m.service==='bab') {
   if(m.operation==='catalog') {await setCatalog(m.payload); result={synced:true};}
   else if(m.operation==='manage') {
    const p=m.payload;
    if (!p || Object.keys(p).some(k=>!['action','site'].includes(k)) || (p.site && !sites.some(s=>s.id===p.site))) throw new Error('scope');
    if(p.action==='status') result=await status(p.site);
    else if(p.action==='revoke' && p.site) {
     const site=sites.find(s=>s.id===p.site);
     await chrome.permissions.remove({origins:[site.origin,...(site.origins||[])].map(pattern)});
     for(const origin of [site.origin,...(site.origins||[])])await BrowserExec.revoke(origin);result={site:p.site,authorized:false};
    } else throw new Error('scope');
   } else throw new Error('scope');
  } else {
   const site=sites.find(s=>s.id===m.service);
   if (!site || site.disabled || m.operation!=='browser' || !await granted(site.origin)) throw new Error('origin_permission_required');
   result=await BrowserExec.execute(m.payload,site.origin,m.allowTab===true && !m.noOpenTab,alive,"bab:"+site.id);
  }
  if(alive()) send({type:'result',session:m.session,result});
 } catch (_) { if(alive()) send({type:'error',session:m.session,error:'bab_operation_failed'}); }
 finally {if(alive())finish(m.session);}
}
function connect() {
 if(port)return;
 catalogReady=false;
 const active=chrome.runtime.connectNative(host);port=active;
 active.onMessage.addListener(m=>{
  if(port!==active)return;
  if(m.type==='catalog') {void setCatalog(m.payload).catch(()=>{catalogReady=false;});return;}
  if(m.type==='attach') {void attach(m);return;}
  if(m.type==='close') sessions.delete(m.session);
 });
 active.onDisconnect.addListener(()=>{
  if(port!==active)return;port=null;catalogReady=false;sessions.clear();
  if(globalThis.BrowserAuthBridge)void BrowserAuthBridge.setCatalog([]).catch(()=>{});
  for(const site of sites)for(const origin of [site.origin,...(site.origins||[])])void BrowserExec.revoke(origin,"bab:"+site.id);
  void chrome.storage.local.set({babConnected:false});
  // Do not repeatedly launch missing/broken native hosts. User can reconnect.
 });
 active.postMessage({type:'broker'});
 void chrome.storage.local.set({babConnected:true});
}
chrome.permissions.onRemoved.addListener(()=>{
 for(const site of sites) void granted(site.origin).then(ok=>{if(!ok){for(const [id,s]of sessions)if(s.site===site.id)sessions.delete(id);return BrowserExec.revoke(site.origin);}});
});
chrome.runtime.onMessage.addListener((m,sender,respond)=>{
 if(sender.id!==chrome.runtime.id || typeof sender.url!=='string' || sender.url.split('#')[0]!==chrome.runtime.getURL('bab.html'))return;
 if(m?.type==='bab:reconnect') {try{port?.disconnect();}catch(_){}port=null;sessions.clear();connect();respond({ok:true});return;}
 if(m?.type==='bab:sites') {void status().then(rows=>respond({connected:!!port && catalogReady,sites:rows,catalog:sites}));return true;}
});
chrome.runtime.onStartup.addListener(connect);
chrome.runtime.onInstalled.addListener(connect);
connect();
// Test hook stays in the isolated extension worker, never exposed to web pages.
if(typeof module!=='undefined')module.exports={validCatalog};
})();
