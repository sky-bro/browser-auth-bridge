const limit=Number(args.limit);
if (!Number.isInteger(limit)||limit<1||limit>100||!args.query?.trim()) return {error:{code:'invalid_args'}};
const params=new URLSearchParams({search:args.query.trim(),incldead:'0',search_mode:'0'});
const categories={movie:'401',tv:'402',documentary:'411'};
if(args.category!=='all')params.set('cat',categories[args.category]);
const response=await ctx.fetch('/torrents.php?'+params);
if(!response.ok)return {error:{code:'http_error'}};
const doc=new DOMParser().parseFromString(await response.text(),'text/html');
if(doc.querySelector('form[action*="takelogin"]'))return {error:{code:'login_required'}};
const integer=s=>Number((s||'').replace(/,/g,'').match(/\d+/)?.[0]||0);
const candidates=[];
for(const input of doc.querySelectorAll('input[name="checked_torrent[]"]')){
 const id=input.value;if(!/^\d+$/.test(id))continue;
 let row=input.closest('tr');
 // NexusPHP nests the title/download links in a small inner table. Metadata
 // belongs to the enclosing torrent row with the category/date/size columns.
 while(row && row.children.length<6)row=row.parentElement?.closest('tr');
 if(!row || row.querySelectorAll('input[name="checked_torrent[]"]').length!==1)continue;
 const links=[...row.querySelectorAll('a[href]')];
 const parsed=a=>{try{return new URL(a.getAttribute('href'),'https://tjupt.org/torrents.php')}catch(_){return null}};
 const find=(path,hash)=>links.find(a=>{const u=parsed(a);return u?.origin==='https://tjupt.org'&&u.pathname===path&&u.searchParams.get('id')===id&&(!hash||u.hash===hash)});
 const title=find('/details.php'),download=find('/download.php');if(!title||!download)continue;
 const textNodes=node=>node.nodeType===3?node.nodeValue:[...node.childNodes].map(textNodes).join(' ');
 const text=textNodes(row).replace(/\s+/g,' ').trim();const size=text.match(/([\d.]+)\s*(KiB|MiB|GiB|TiB)/i);
 const date=text.match(/\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}/);
 const cat=links.map(parsed).find(u=>u?.origin==='https://tjupt.org'&&u.searchParams.has('cat'))?.searchParams.get('cat');
 candidates.push({torrent_id:id,title:(title.getAttribute('title')||title.textContent).trim(),download_url:'https://tjupt.org/download.php?id='+id,category:Object.entries(categories).find(([,v])=>v===cat)?.[0]||'other',seeders:integer(find('/details.php','#seeders')?.textContent),leechers:integer(find('/details.php','#leechers')?.textContent),snatched:integer(find('/viewsnatches.php')?.textContent),size:size?size[0]:'unknown',size_bytes:size?Math.floor(Number(size[1])*1024**(['kib','mib','gib','tib'].indexOf(size[2].toLowerCase())+1)):null,published:date?.[0]||'unknown',subtitles:/中字|中文字幕|简体字幕|繁体字幕|中英字幕/.test(text)});
}
const comparators={seeders:(a,b)=>b.seeders-a.seeders||b.snatched-a.snatched,snatched:(a,b)=>b.snatched-a.snatched||b.seeders-a.seeders,'size-asc':(a,b)=>(a.size_bytes??Infinity)-(b.size_bytes??Infinity),'date-desc':(a,b)=>b.published.localeCompare(a.published)};
return candidates.sort(comparators[args.sort]).slice(0,limit);
