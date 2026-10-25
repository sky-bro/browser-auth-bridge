/* Generic browser operations. Business scripts and output schemas live in CLIs. */
(() => {
  "use strict";
  const leases = new Map();
  const reservations = new Map();
  const busy = new Set();
  const validID = value => typeof value === "string" && /^[a-f0-9]{64}$/.test(value);
  const originOf = value => { try { return new URL(value).origin; } catch (_) { return ""; } };
  const pattern = origin => {const url=new URL(origin);return `${url.protocol}//${url.hostname}/*`;};
  const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
  function validate(p, allowedOrigin) {
    if (!p || p.version !== 1 || p.origin !== allowedOrigin || !validID(p.browserSession) ||
        !["open", "eval", "close"].includes(p.action) ||
        Object.keys(p).some(k => !["version","origin","browserSession","action","url","match","script","args"].includes(k))) return false;
    if (p.action === "open") return typeof p.url === "string" && p.url.length <= 8192 && originOf(p.url) === allowedOrigin &&
      (p.match === undefined || p.match === "origin") && !p.script && p.args === undefined && !new URL(p.url).username && !new URL(p.url).password;
    if (p.action === "eval") return typeof p.script === "string" && p.script.length > 0 && p.script.length <= 256*1024 && !p.url && p.match === undefined && (!p.args || typeof p.args === "object");
    return !p.url && p.match === undefined && !p.script && p.args === undefined;
  }
  async function authorized(origin) {
    if (!await chrome.permissions.contains({origins: [pattern(origin)]})) throw new Error("origin_permission_required");
    if (!await chrome.permissions.contains({permissions: ["debugger"]})) throw new Error("script_permission_required");
  }
  async function ready(tabId, origin, alive, expectedURL) {
    for (let i=0;i<100;i++) {
      if (!alive()) throw new Error("browser_canceled");
      const tab = await chrome.tabs.get(tabId);
      if (tab.status === "complete" && !tab.pendingUrl) {
        if (originOf(tab.url) !== origin) throw new Error("page_scope_mismatch");
        if (!expectedURL || new URL(tab.url).href === new URL(expectedURL).href) return tab;
      }
      await delay(100);
    }
    throw new Error("page_timeout");
  }
  async function release(key) {
    const lease=leases.get(key);
    if (!lease) return;
    lease.canceled=true;leases.delete(key);reservations.delete(lease.tabId);
    if (lease.attached) { try { await chrome.debugger.detach({tabId:lease.tabId}); } catch (_) {} }
    if (lease.owned) {try {await chrome.tabs.remove(lease.tabId);}catch(_){}}
  }
  async function execute(p, allowedOrigin, allowCreate, alive=()=>true, owner="") {
    if (!validate(p,allowedOrigin)) throw new Error("browser_scope_denied");
    const key=owner+":"+allowedOrigin+":"+p.browserSession;
    if (busy.has(key)) throw new Error("browser_session_busy");
    busy.add(key);
    let timer;
    let active=true, leaseRef=null;
    const running=()=>active && alive() && !leaseRef?.canceled;
    // Reclaim abandoned workflow leases when a later caller arrives.
    for (const [oldKey, oldLease] of leases) {
      if (oldKey !== key && !busy.has(oldKey) && Date.now() - oldLease.lastUsed > 120000) await release(oldKey);
    }
    try {
      return await Promise.race([(async()=>{
        if (p.action === "close") {await release(key);return {version:1};}
        await authorized(allowedOrigin);
        if (!running()) throw new Error("browser_canceled");
        let lease=leases.get(key);leaseRef=lease;
        if (p.action === "open") {
          if (lease) {
            let current;
            try {current=await chrome.tabs.get(lease.tabId);} catch (_) {await release(key);lease=null;leaseRef=null;}
            const matches = p.match === "origin" ? originOf(current?.url) === allowedOrigin : current?.url === p.url;
            if (lease && !matches) {
              if (lease.owned) {
                // Generic navigation is allowed only to the already authorized origin.
                if (lease.attached) {await chrome.debugger.detach({tabId:lease.tabId});lease.attached=false;}
                await chrome.tabs.update(lease.tabId,{url:p.url});
              } else {await release(key);lease=null;leaseRef=null;}
            }
          }
          if (!lease) {
            const tabs=await chrome.tabs.query({url:pattern(allowedOrigin)});
            if (!running()) throw new Error("browser_canceled");
            const tab=tabs.find(t=>(p.match === "origin" ? originOf(t.url) === allowedOrigin : t.url === p.url) && !reservations.has(t.id));
            if (tab) {
              lease={tabId:tab.id,origin:allowedOrigin,owner,owned:false,lastUsed:Date.now()};
              reservations.set(tab.id,key);leases.set(key,lease);
            } else {
              if (!allowCreate) throw new Error("page_required");
              // Each live browser session owns its tab; simultaneous workflows cannot mix pages.
              const created=await chrome.tabs.create({url:p.url,active:false});
              lease={tabId:created.id,origin:allowedOrigin,owner,owned:true,lastUsed:Date.now()};
              if (!running()) {await chrome.tabs.remove(created.id);throw new Error("browser_canceled");}
              reservations.set(created.id,key);leases.set(key,lease);
            }
          }
          leaseRef=lease;
          await authorized(allowedOrigin);
          await ready(lease.tabId,allowedOrigin,running,p.match === "origin" ? undefined : p.url);
          lease.lastUsed=Date.now();
          return {version:1,tabId:lease.tabId,owned:lease.owned};
        }
        if (!lease) throw new Error("browser_session_missing");
        await ready(lease.tabId,allowedOrigin,running);
        if (!lease.attached) {await chrome.debugger.attach({tabId:lease.tabId},"1.3");lease.attached=true;}
        // Recheck after attach; do not run user code if navigation changed the target.
        const check=await chrome.debugger.sendCommand({tabId:lease.tabId},"Runtime.evaluate",{expression:"location.origin",returnByValue:true});
        if (check.result?.value !== allowedOrigin) throw new Error("page_scope_mismatch");
        await authorized(allowedOrigin);
        if (!running()) throw new Error("browser_canceled");
        const expression=`(async () => { if (location.origin !== ${JSON.stringify(allowedOrigin)}) throw new Error('page_scope_mismatch'); const args = ${JSON.stringify(p.args ?? {})}; return await (async () => {\n${p.script}\n})(); })()`;
        const result=await chrome.debugger.sendCommand({tabId:lease.tabId},"Runtime.evaluate",{
          expression,awaitPromise:true,returnByValue:true,timeout:45000,
        });
        if (!running()) throw new Error("browser_canceled");
        if (result.exceptionDetails) throw new Error("browser_script_failed");
        if (result.result?.subtype === "error") throw new Error("browser_script_failed");
        const value=result.result?.value ?? null;
        if (new TextEncoder().encode(JSON.stringify(value)).length>768*1024) throw new Error("browser_result_too_large");
        lease.lastUsed=Date.now();
        return {version:1,tabId:lease.tabId,owned:lease.owned,value};
      })(),new Promise((_,reject)=>{timer=setTimeout(()=>{active=false;reject(new Error("browser_outcome_unknown"));},55000);})]);
    } catch(error) {
      await release(key);
      const codes=["browser_scope_denied","browser_session_busy","origin_permission_required","script_permission_required","browser_canceled","page_scope_mismatch","page_timeout","page_required","browser_session_missing","browser_script_failed","browser_result_too_large","browser_outcome_unknown"];
      throw new Error(codes.includes(error?.message)?error.message:"browser_execution_failed");
    } finally {active=false;clearTimeout(timer);busy.delete(key);}
  }
  async function revoke(origin,owner) {
    for(const [key,lease] of leases) if((!origin || lease.origin===origin) && (owner===undefined || lease.owner===owner)) await release(key);
  }
  chrome.permissions.onRemoved.addListener(removed=>{
    if(removed.permissions?.includes("debugger")) void revoke();
    else void (async()=>{
      // Chrome host grants omit ports and can contain wildcard hosts. Recheck
      // actual permission rather than comparing a match pattern to an origin.
      for(const [key,lease] of [...leases]) {
        let granted=false;
        try {const url=new URL(lease.origin);granted=await chrome.permissions.contains({origins:[`${url.protocol}//${url.hostname}/*`]});}catch(_){}
        if(!granted && leases.get(key)===lease)await release(key);
      }
    })();
  });
  chrome.tabs.onRemoved.addListener(tabId=>{const key=reservations.get(tabId);if(key) void release(key);});
  globalThis.BrowserExec=Object.freeze({execute,validate,revoke});
})();
