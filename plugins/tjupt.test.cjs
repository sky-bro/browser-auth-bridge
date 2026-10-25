const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

async function run(name, args = {}, options = {}) {
 const requests=[], artifacts=[];
 const response={ok:options.ok!==false,text:async()=>'<fixture>',arrayBuffer:async()=>Uint8Array.from(options.bytes || []).buffer};
 const document=options.document || {querySelector:()=>null,querySelectorAll:()=>[]};
 const scope={args,URL,URLSearchParams,Uint8Array,DOMParser:class {parseFromString(){return document;}},ctx:{
  fetch:async url=>{requests.push(url);return response;},
  artifact:bytes=>{artifacts.push([...bytes]);return {artifact:{key:'fixture',size:bytes.length}};},
 }};
 const script=fs.readFileSync(path.join(__dirname,'builtin/tjupt',name+'.js'),'utf8');
 const value=await vm.runInNewContext('(async()=>{'+script+'})()',scope);
 return {value:JSON.parse(JSON.stringify(value)),requests,artifacts};
}

test('login checks distinguish a login page, authenticated identity and unknown state',async()=>{
 const login=await run('whoami',{}, {document:{querySelector:s=>s.startsWith('form')?{}:null}});
 assert.deepEqual(login.value,{logged_in:false});
 const user={textContent:' Fixture reader ',closest:()=>({})};
 const known=await run('whoami',{}, {document:{querySelector:s=>s.includes('logout')?{}:null,querySelectorAll:()=>[user]}});
 assert.deepEqual(known.value,{logged_in:true,identity:'Fixture reader'});
 const unknown=await run('whoami');assert.deepEqual(unknown.value,{logged_in:null,identity:null});
});
test('invalid identifiers and empty searches never issue a request',async()=>{
 for(const name of ['details','torrent']) {
  const result=await run(name,{id:'../private'});
  assert.equal(result.value.error.code,'invalid_args');assert.equal(result.requests.length,0);
 }
 const result=await run('search',{query:' ',limit:'20'});
 assert.equal(result.value.error.code,'invalid_args');assert.equal(result.requests.length,0);
});
test('torrent metadata rejects HTML and returns binary content only through artifact API',async()=>{
 const bad=await run('torrent',{id:'100'},{bytes:[60,104,116,109,108,62]});
 assert.equal(bad.value.error.code,'invalid_torrent_or_login');assert.equal(bad.artifacts.length,0);
 const bytes=[100,52,58,105,110,102,111,100,101,101];
 const good=await run('torrent',{id:'100'},{bytes});
 assert.deepEqual(good.requests,['/download.php?id=100']);assert.deepEqual(good.artifacts,[bytes]);
 assert.deepEqual(good.value,{artifact:{key:'fixture',size:bytes.length}});
});
function release(id, seeders, title, downloadOrigin='') {
 const input={value:id};
 const link=(href,textContent,titleValue)=>({textContent,getAttribute:key=>key==='href'?href:key==='title'?titleValue:null});
 const links=[link('/details.php?id='+id,title,title),link(downloadOrigin+'/download.php?id='+id,'download'),link('/torrents.php?cat=401','movie'),link('/details.php?id='+id+'#seeders',String(seeders)),link('/details.php?id='+id+'#leechers','2'),link('/viewsnatches.php?id='+id,'5')];
 const row={children:Array(8),childNodes:[{nodeType:3,nodeValue:'2024-01-02 03:04:05 1.50 GiB 中英字幕'}],querySelectorAll:selector=>selector.startsWith('input')?[input]:links};
 const inner={children:Array(2),parentElement:{closest:()=>row}};
 input.closest=()=>inner;
 return input;
}
test('search handles nested release tables and rejects foreign download links',async()=>{
 const rows=[release('101',4,'Example A'),release('102',12,'Example B'),release('103',99,'Foreign','https://other.example.test')];
 const result=await run('search',{query:'Example & test',limit:'2',category:'movie',sort:'seeders'}, {document:{querySelector:()=>null,querySelectorAll:()=>rows}});
 assert.equal(result.value.length,2);assert.equal(result.value[0].torrent_id,'102');
 assert.equal(result.value[0].seeders,12);assert.equal(result.value[0].size,'1.50 GiB');assert.equal(result.value[0].subtitles,true);
 assert.equal(result.value[0].download_url,'https://tjupt.org/download.php?id=102');
 const url=new URL(result.requests[0],'https://tjupt.org');assert.equal(url.searchParams.get('search'),'Example & test');assert.equal(url.searchParams.get('cat'),'401');
});
test('search and details report missing login before returning page contents',async()=>{
 const document={querySelector:s=>s.startsWith('form')?{}:null};
 const search=await run('search',{query:'Example',limit:'2',category:'all',sort:'seeders'},{document});
 const details=await run('details',{id:'100'},{document});
 assert.equal(search.value.error.code,'login_required');assert.equal(details.value.error.code,'login_required');
});
