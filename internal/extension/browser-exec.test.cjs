const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const test=require('node:test');
const source=fs.readFileSync(require.resolve('./browser-exec.js'),'utf8');
const origin='https://console.example.test';
const sid='a'.repeat(64);
const command=(action,extra={})=>({version:1,origin,browserSession:sid,action,...extra});
function setup({grant=true,redirect=false,existing=false,scriptError=false,hang=false}={}) {
 let next=1,removedPermission,onTabRemoved;const tabs=new Map(),calls=[],removed=[],permissions=[];let deadline;
 if(existing)tabs.set(next++,{id:1,url:origin+'/product',status:'complete'});
 const chrome={
  permissions:{async contains(request){permissions.push(request);return grant},onRemoved:{addListener(fn){removedPermission=fn}}},
  tabs:{async query(){return [...tabs.values()]},async create({url,active}){assert.equal(active,false);const t={id:next++,url,status:'complete'};tabs.set(t.id,t);return t},async get(id){if(!tabs.has(id))throw Error('gone');return tabs.get(id)},async update(id,opts){Object.assign(tabs.get(id),opts);return tabs.get(id)},async remove(id){removed.push(id);tabs.delete(id)},onRemoved:{addListener(fn){onTabRemoved=fn}}},
  debugger:{async attach(t){calls.push(['attach',t.tabId])},async detach(t){calls.push(['detach',t.tabId])},async sendCommand(t,method,params){calls.push([method,params]);if(params.expression==='location.origin')return {result:{value:redirect?'https://evil.test':origin}};if(hang)return new Promise(()=>{});if(scriptError)return {exceptionDetails:{text:'secret'}};return {result:{value:{ok:true}}}}},
 };
 const context={chrome,URL,TextEncoder,console,setTimeout(fn,ms){if(ms===55000){deadline=fn;return 1};return setTimeout(fn,ms)},clearTimeout(){}};
 vm.runInNewContext(source,context);
 return {api:context.BrowserExec,tabs,calls,removed,permissions,revoke:()=>removedPermission({permissions:['debugger']}),revokeHost:()=>{grant=false;removedPermission({origins:[origin+"/*"]});},timeout:()=>deadline()};
}
test('no business endpoint whitelist: arbitrary scripts accepted only for the configured origin',()=>{
 const {api}=setup();assert.equal(api.validate(command('eval',{script:'return await fetch("/a/new/api").then(r=>r.json())'}),origin),true);
 for(const p of [command('open',{url:'https://evil.test'}),command('open',{url:origin.replace('https://','https://user@')+'/'}),command('eval',{origin:'https://evil.test',script:'return 1'}),command('eval',{script:'return 1',cdpMethod:'anything'}),command('close',{url:origin})])assert.equal(api.validate(p,origin),false);
});
test('requires separate permission and honors no-open policy',async()=>{
 let s=setup({grant:false});await assert.rejects(s.api.execute(command('open',{url:origin+'/product'}),origin,true),/permission_required/);assert.equal(s.tabs.size,0);
 s=setup();await assert.rejects(s.api.execute(command('open',{url:origin+'/product'}),origin,false),/page_required/);assert.equal(s.tabs.size,0);
});
test('CLI opens, executes arguments, navigates owned tab, then closes its own page',async()=>{
 const s=setup();const opened=await s.api.execute(command('open',{url:origin+'/product'}),origin,true);
 const result=await s.api.execute(command('eval',{script:'return args',args:{product:'adb5'}}),origin,false);
 assert.equal(result.value.ok,true);assert.ok(s.calls.some(c=>c[0]==='Runtime.evaluate'&&c[1].expression.includes('"product":"adb5"')));
 await s.api.execute(command('open',{url:origin+'/anything'}),origin,true);
 assert.equal(s.tabs.size,1);assert.equal(s.tabs.get(opened.tabId).url,origin+'/anything');
 await s.api.execute(command('close'),origin,false);assert.equal(s.tabs.size,0);
});
test('existing user page is neither navigated nor closed',async()=>{
 const s=setup({existing:true});await s.api.execute(command('open',{url:origin+'/product'}),origin,false);
 await assert.rejects(s.api.execute(command('open',{url:origin+'/different'}),origin,false),/page_required/);
 assert.equal(s.tabs.get(1).url,origin+'/product');assert.equal(s.removed.length,0);
});
test('origin matching reuses a same-origin user page without navigation',async()=>{
 const s=setup({existing:true});const opened=await s.api.execute(command('open',{url:origin+'/',match:'origin'}),origin,false);
 assert.equal(opened.tabId,1);assert.equal(s.tabs.get(1).url,origin+'/product');
 await s.api.execute(command('close'),origin,false);assert.equal(s.tabs.has(1),true);assert.equal(s.removed.length,0);
});
test('concurrent workflows cannot reserve the same user tab',async()=>{
 const s=setup({existing:true});const p=command('open',{url:origin+'/product'});
 const [a,b]=await Promise.all([s.api.execute(p,origin,true),s.api.execute({...p,browserSession:'b'.repeat(64)},origin,true)]);
 assert.notEqual(a.tabId,b.tabId);
});
test('navigation at debugger attach is rechecked before script execution',async()=>{
 const s=setup({redirect:true});await s.api.execute(command('open',{url:origin+'/product'}),origin,true);
 await assert.rejects(s.api.execute(command('eval',{script:'return 1'}),origin,false),/page_scope_mismatch/);
 assert.equal(s.calls.filter(c=>c[0]==='Runtime.evaluate').length,1);
});
test('script errors do not export exception content or retry',async()=>{
 const s=setup({scriptError:true});await s.api.execute(command('open',{url:origin+'/product'}),origin,true);
 await assert.rejects(s.api.execute(command('eval',{script:'throw Error("secret")'}),origin,false),/^Error: browser_script_failed$/);
 assert.equal(s.calls.filter(c=>c[0]==='Runtime.evaluate').length,2);
});
test('timeout reports unknown outcome and releases debugger; never reruns script',async()=>{
 const s=setup({hang:true});await s.api.execute(command('open',{url:origin+'/product'}),origin,true);
 const promise=s.api.execute(command('eval',{script:'return 1'}),origin,false);
 await new Promise(setImmediate);s.timeout();await assert.rejects(promise,/browser_outcome_unknown/);
 assert.ok(s.calls.some(c=>c[0]==='detach'));
});
test('revoking debugger detaches owned connections',async()=>{
 const s=setup();await s.api.execute(command('open',{url:origin+'/product'}),origin,true);await s.api.execute(command('eval',{script:'return 1'}),origin,false);
 s.revoke();await new Promise(setImmediate);
 await assert.rejects(s.api.execute(command('eval',{script:'return 1'}),origin,false),/browser_session_missing/);
});

test('open waits for the requested URL even while the old page reports complete', async () => {
 const s=setup();
 await s.api.execute(command('open',{url:origin+'/product'}),origin,true);
 const tab=s.tabs.get(1);
 let url=tab.url,reads=0;
 Object.defineProperty(tab,'url',{get(){reads++;return reads<4?origin+'/product':url},set(value){url=value},configurable:true});
 await s.api.execute(command('open',{url:origin+'/clusters'}),origin,true);
 assert.ok(reads>=4,'must poll past the stale complete page');
 assert.equal(tab.url,origin+'/clusters');
});

test('revocation closes only bridge-owned pages, including origins with explicit ports',async()=>{
 const s=setup(),portOrigin=origin+':9443';
 await s.api.execute(command('open',{origin:portOrigin,url:portOrigin+'/page'}),portOrigin,true);
 assert.ok(s.permissions.some(p=>p.origins?.[0]===origin+"/*"));
 assert.equal(s.permissions.some(p=>p.origins?.[0]?.includes(":9443")),false);
 s.revokeHost();await new Promise(setImmediate);await new Promise(setImmediate);
 assert.equal(s.tabs.size,0);
 const user=setup({existing:true});await user.api.execute(command('open',{url:origin+'/product'}),origin,false);
 user.revokeHost();await new Promise(setImmediate);assert.equal(user.tabs.size,1);
});
test('plugin-scoped cleanup leaves another plugin on the same host running',async()=>{
 const s=setup(),open=command('open',{url:origin+'/product'});
 const a=await s.api.execute(open,origin,true,()=>true,'bab:first');
 const b=await s.api.execute(open,origin,true,()=>true,'bab:second');
 assert.notEqual(a.tabId,b.tabId);
 await s.api.revoke(origin,'bab:first');
 assert.equal(s.tabs.has(a.tabId),false);assert.equal(s.tabs.has(b.tabId),true);
 const result=await s.api.execute(command('eval',{script:'return 1'}),origin,false,()=>true,'bab:second');
 assert.equal(result.value.ok,true);
});
test('a lost owned page can be replaced without retaining cancellation from its old lease',async()=>{
 const s=setup();await s.api.execute(command('open',{url:origin+'/product'}),origin,true);
 s.tabs.clear();await s.api.execute(command('open',{url:origin+'/product'}),origin,true);
 assert.equal(s.tabs.size,1);
});
