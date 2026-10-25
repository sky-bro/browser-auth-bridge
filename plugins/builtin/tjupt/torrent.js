if(!/^\d+$/.test(args.id))return {error:{code:'invalid_args'}};
const r=await ctx.fetch('/download.php?id='+args.id);if(!r.ok)return {error:{code:'http_error'}};
const bytes=new Uint8Array(await r.arrayBuffer());
if(bytes[0]!==100 || bytes[bytes.length-1]!==101)return {error:{code:'invalid_torrent_or_login'}};
return ctx.artifact(bytes);
