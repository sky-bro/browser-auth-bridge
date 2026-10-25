'use strict';
const $=id=>document.getElementById(id);
const setupCommand=`bab setup --extension-id ${chrome.runtime.id}`;
$('setup').textContent=setupCommand;
const pattern=origin=>{const url=new URL(origin);return `${url.protocol}//${url.hostname}/*`;};
let revision=0;
let connectionKnown=false;
function element(tag,className,text){
 const node=document.createElement(tag);
 if(className)node.className=className;
 if(text!==undefined)node.textContent=text;
 return node;
}
function message(text){$('state').textContent=text;$('state').hidden=!text;}
function button(label,className,action){
 const node=element('button',className,label);node.type='button';node.onclick=action;return node;
}
function scopeRow(site,origin,authorized){
 const row=element('div','scope-row');
 const hostname=element('span','hostname',new URL(origin).host);hostname.title=origin;
 row.append(element('span',`scope-dot${authorized?' authorized':''}`),hostname);
 const actions=element('div','scope-actions');
 if(site.available!==false && !site.disabled && !authorized){
  actions.append(button('授权','authorize',async()=>{
   // Request synchronously from the click so Chrome retains the user gesture.
   const request=chrome.permissions.request({origins:[pattern(origin)]});
   try{if(!await request){message('未授予访问权限。');return;}await render();}catch(_){message('无法完成授权，请重试。');}
  }));
 }
 const open=button('↗','icon-button open-site',()=>chrome.tabs.create({url:origin+'/',active:true}));
 open.title='打开网站';open.setAttribute('aria-label',`打开 ${new URL(origin).host}`);actions.append(open);
 if(authorized){
  const menu=element('details','scope-menu');
  const summary=element('summary','','⋯');summary.title='管理授权';summary.setAttribute('aria-label',`管理 ${new URL(origin).host} 的授权`);
  menu.append(summary,button('撤销授权','revoke',async()=>{
   try{await chrome.permissions.remove({origins:[pattern(origin)]});await render();}catch(_){message('无法撤销授权，请重试。');}
  }));
  actions.append(menu);
 }
 row.append(actions);return row;
}
async function render(){
 const generation=++revision;
 try{
  const info=await chrome.runtime.sendMessage({type:'bab:sites'});
  if(generation!==revision)return;
  const connected=!!info?.connected;
  $('connection').className=`connection${connected?' is-connected':''}`;
  $('connection-label').textContent=connected?'已连接':'未连接';
  message(connected?'':'连接尚未建立，请检查连接设置或点击右上角重连。');
  // Do not close settings that the user deliberately opened when the list updates.
  if(!connectionKnown || !connected)$('setup-details').open=!connected;
  connectionKnown=true;
  const catalog=info?.catalog||[];
  $('command-count').textContent=`${catalog.length} 个`;
  const list=$('sites');list.replaceChildren();
  for(const site of catalog){
   const row=element('article',site.available===false || site.disabled?'unavailable':'');row.title=site.label;
   const header=element('div','command-header');
   const check=info.sites?.find(item=>item.site===site.id);
   const origins=[site.origin,...(site.origins||[])];
   const authorized=origins.map(origin=>check?.scopes?.find(scope=>scope.origin===origin)?.authorized ?? check?.authorized ?? false);
   const complete=authorized.every(Boolean),any=authorized.some(Boolean);
   const badge=site.disabled?['','已停用']:site.available===false?['','未安装']:complete?['authorized','已授权']:any?['pending','部分授权']:['pending','待授权'];
   header.append(element('span','command-icon',site.id.slice(0,2).toUpperCase()),element('strong','',`bab ${site.id}`),element('span',`badge ${badge[0]}`.trim(),badge[1]));
   row.append(header);
   if(site.available===false)row.append(element('p','unavailable-note','安装命令后即可使用'));
   origins.forEach((origin,index)=>row.append(scopeRow(site,origin,authorized[index])));
   const control=element('details','plugin-control');control.append(element('summary','','插件开关'));
   const operation=site.disabled?'enable':'disable';
   control.append(button(site.disabled?'复制启用命令':'复制停用命令','',async()=>{
    try {await navigator.clipboard.writeText(`bab plugin ${operation} ${site.id}`);message('已复制命令，在终端运行后生效。');}catch(_){message(`请运行 bab plugin ${operation} ${site.id}`);}
   }));
   row.append(control);list.append(row);
   if(location.hash.slice(1)===site.id)row.scrollIntoView();
  }
  if(!catalog.length)list.append(element('p','empty',connected?'还没有注册命令':'连接后显示已注册命令'));
 }catch(_){
  if(generation===revision){$('connection-label').textContent='未连接';$('connection').className='connection';message('无法连接扩展后台，请重新加载扩展。');$('setup-details').open=true;}
 }
}
$('reconnect').onclick=async()=>{
 try{await chrome.runtime.sendMessage({type:'bab:reconnect'});await render();}catch(_){message('重新连接失败，请重新加载扩展。');}
};
$('copy-setup').onclick=async()=>{
 try{await navigator.clipboard.writeText(setupCommand);$('copy-setup').textContent='已复制';}catch(_){message('无法复制，请选中上方命令手动复制。');}
};
chrome.storage?.onChanged?.addListener((changes,area)=>{if(area==='local'&&(changes.babCatalog||changes.babConnected))void render();});
void render();
