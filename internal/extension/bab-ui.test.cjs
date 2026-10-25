const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const settle=async()=>{for(let i=0;i<6;i++)await new Promise(r=>setImmediate(r));};
class Element{
 constructor(tag){this.tag=tag;this.children=[];this.textContent='';this.disabled=false;this.attributes={};}
 append(...nodes){this.children.push(...nodes);}
 replaceChildren(...nodes){this.children=nodes;}
 setAttribute(name,value){this.attributes[name]=value;}
 scrollIntoView(){}
}
function flatten(element){return[element,...element.children.flatMap(flatten)];}
async function harness({authorized=false,connected=true,disabled=false}={}){
 const elements=Object.fromEntries(['setup','state','sites','setup-details','reconnect','connection','connection-label','command-count','copy-setup'].map(id=>[id,new Element('div')]));
 const catalog=['console','inventory','ttyd','library'].map(id=>({id,origin:'https://'+id+'.example.test',label:id,available:id!=='inventory',disabled:id==='library'&&disabled,...(id==='console'?{origins:['https://catalog.example.test']}:{})}));
 const requests=[],removed=[],opened=[],copied=[];let onChange;
 const chrome={runtime:{id:'test',sendMessage:async()=>({connected,catalog,sites:catalog.map(s=>({site:s.id,authorized}))})},permissions:{request:async p=>{requests.push(p);return true;},remove:async p=>{removed.push(p);return true;}},storage:{onChanged:{addListener(fn){onChange=fn;}}},tabs:{create:async p=>{opened.push(p);}}};
 vm.runInNewContext(fs.readFileSync(__dirname+'/bab.js','utf8'),{URL,chrome,navigator:{clipboard:{writeText:async text=>copied.push(text)}},document:{getElementById:id=>elements[id],createElement:tag=>new Element(tag)},location:{hash:''}});
 await settle();return{elements,catalog,requests,removed,opened,copied,onChange};
}
test('registry renders compact command cards without independent integration controls',async()=>{
 const h=await harness(),{elements}=h;
 assert.equal(elements.sites.children.length,4);
 const nodes=flatten(elements.sites);assert.deepEqual(nodes.filter(e=>e.tag==='strong').map(e=>e.textContent),['bab console','bab inventory','bab ttyd','bab library']);
 assert.equal(nodes.some(e=>['input','select'].includes(e.tag)),false);
 assert.equal(elements['setup-details'].open,false);
 assert.equal(elements['command-count'].textContent,'4 个');
 const inventory=elements.sites.children[1];assert.equal(flatten(inventory).some(e=>e.textContent==='授权'),false);
 const grants=flatten(elements.sites.children[0]).filter(e=>e.textContent==='授权');assert.equal(grants.length,2);
 await grants[1].onclick();assert.equal(h.requests[0].origins[0],'https://catalog.example.test/*');
});
test('authorized sites keep open action and put revocation inside an explicit menu',async()=>{
 const h=await harness({authorized:true}),row=h.elements.sites.children[3],nodes=flatten(row);
 assert.equal(nodes.some(e=>e.textContent==='授权'),false);
 const menu=nodes.find(e=>e.tag==='details');assert.ok(menu);assert.equal(menu.open,undefined);
 assert.ok(flatten(menu).some(e=>e.textContent==='撤销授权'));assert.equal(h.removed.length,0);
 await nodes.find(e=>e.attributes['aria-label']==='打开 library.example.test').onclick();
 assert.equal(h.opened[0].url,'https://library.example.test/');
 await flatten(menu).find(e=>e.textContent==='撤销授权').onclick();assert.equal(h.removed[0].origins[0],'https://library.example.test/*');
});
test('setup expands only when needed and user-opened settings survive a refresh',async()=>{
 const h=await harness();h.elements['setup-details'].open=true;h.onChange({babCatalog:{}},'local');await settle();assert.equal(h.elements['setup-details'].open,true);
 await h.elements['copy-setup'].onclick();assert.equal(h.copied[0],'bab setup --extension-id test');
 const offline=await harness({connected:false});assert.equal(offline.elements['setup-details'].open,true);assert.equal(offline.elements.state.hidden,false);
});


test('disabled cards show their state and copy an explicit enable command',async()=>{
 const h=await harness({disabled:true}),nodes=flatten(h.elements.sites.children[3]);
 assert.equal(nodes.some(node=>node.textContent==='已停用'),true);
 assert.equal(nodes.some(node=>node.textContent==='授权'),false);
 await nodes.find(node=>node.textContent==='复制启用命令').onclick();
 assert.equal(h.copied[0],'bab plugin enable library');assert.equal(h.requests.length,0);
});
