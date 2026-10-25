const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(__dirname+'/services.js','utf8');
function event(){const listeners=[];return {addListener:f=>listeners.push(f),emit:()=>listeners.forEach(f=>f())};}
const settle=async()=>{for(let i=0;i<8;i++)await new Promise(r=>setImmediate(r));};
async function harness(){
 const started=[],stopped=[],registered=new Map(),grants=new Set(),imported=[];
 const scope={URL,console,BridgePluginPackages:[{id:'console',origins:['https://console.example.test'],files:['plugins/console/background.js','plugins/console/content.js'],background:['plugins/console/background.js']}]};
 scope.chrome={storage:{local:{get:async()=>({}),set:async()=>{}}},permissions:{contains:async({origins})=>origins.every(o=>grants.has(o)),onAdded:event(),onRemoved:event()},scripting:{getRegisteredContentScripts:async({ids})=>ids.flatMap(id=>registered.has(id)?[registered.get(id)]:[]),unregisterContentScripts:async({ids})=>ids.forEach(id=>registered.delete(id)),registerContentScripts:async rows=>rows.forEach(row=>registered.set(row.id,row))}};
 scope.importScripts=(...paths)=>{imported.push(...paths);scope.BrowserAuthBridge.registerService({id:'console-stream',origin:'https://console.example.test/*',contentScript:{matches:['https://console.example.test/session*'],js:['plugins/console/content.js']},start:()=>started.push('console'),stop:()=>stopped.push('console')});};
 vm.runInNewContext(fs.readFileSync(__dirname+'/site-config.js','utf8'),scope);
 vm.runInNewContext(source,scope);await settle();
 return {scope,started,stopped,registered,grants,imported};
}
test('plugin services need both installed native registry and explicit browser grant',async()=>{
 const h=await harness();assert.equal(h.started.length,0);assert.deepEqual(h.imported,['plugins/console/background.js']);
 h.grants.add('https://console.example.test/*');h.scope.chrome.permissions.onAdded.emit();await settle();assert.equal(h.started.length,0);
 await h.scope.BrowserAuthBridge.setCatalog([{id:'console',origin:'https://console.example.test'}]);assert.equal(h.started.length,1);assert.equal(h.registered.size,1);
 await h.scope.BrowserAuthBridge.setCatalog([]);assert.equal(h.registered.size,0);assert.ok(h.stopped.length>0);
});
test('changed scopes and revoked permissions stop compatibility services',async()=>{
 const h=await harness();h.grants.add('https://console.example.test/*');
 await h.scope.BrowserAuthBridge.setCatalog([{id:'console',origin:'https://console.example.test'}]);
 h.grants.clear();h.scope.chrome.permissions.onRemoved.emit();await settle();assert.equal(h.registered.size,0);
 const count=h.started.length;
 h.grants.add('https://console.example.test/*');await h.scope.BrowserAuthBridge.setCatalog([{id:'console',origin:'https://another.example.test'}]);assert.equal(h.started.length,count);
});
test('background services cannot register outside their declared package or content files',async()=>{
 const h=await harness();
 assert.throws(()=>h.scope.BrowserAuthBridge.registerService({id:'rogue',origin:'https://console.example.test/*',start(){},stop(){}}));
});


test('reload prunes persisted content registrations from removed plugins',async()=>{
 const h=await harness();h.registered.set('old-consumer',{id:'old-consumer'});
 h.scope.chrome.storage.local.get=async()=>({babServiceIDs:['old-consumer']});
 await h.scope.BrowserAuthBridge.setCatalog([]);assert.equal(h.registered.has('old-consumer'),false);
});


test('disabled registry entries stop services without changing Chrome grants',async()=>{
 const h=await harness();h.grants.add('https://console.example.test/*');
 await h.scope.BrowserAuthBridge.setCatalog([{id:'console',origin:'https://console.example.test'}]);
 await h.scope.BrowserAuthBridge.setCatalog([{id:'console',origin:'https://console.example.test',disabled:true}]);
 assert.equal(h.registered.size,0);assert.equal(h.grants.has('https://console.example.test/*'),true);
});
