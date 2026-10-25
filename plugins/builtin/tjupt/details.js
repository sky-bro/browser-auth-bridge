if(!/^\d+$/.test(args.id))return {error:{code:'invalid_args'}};
const r=await ctx.fetch('/details.php?id='+args.id+'&hit=1');if(!r.ok)return {error:{code:'http_error'}};
const doc=new DOMParser().parseFromString(await r.text(),'text/html');
if(doc.querySelector('form[action*="takelogin"]'))return {error:{code:'login_required'}};
const description=(doc.querySelector('#kdescr')?.textContent||'').trim();
return {torrent_id:args.id,title:(doc.querySelector('h1')?.textContent||doc.title).trim(),description:description.slice(0,24000)};
