const r=await ctx.fetch('/torrents.php');
if (!r.ok) return {error:{code:'http_error'}};
const doc=new DOMParser().parseFromString(await r.text(),'text/html');
if (doc.querySelector('form[action*="takelogin"]')) return {logged_in:false};
const user=[...doc.querySelectorAll('a[href*="userdetails.php?id="]')].find(a=>a.closest('#info_block') || a.closest('#userbar'));
// A download link or logout link is positive evidence; absence is unknown.
const logged=!!doc.querySelector('a[href*="logout.php"],input[name="checked_torrent[]"]');
return {logged_in:logged?true:null,identity:user?.textContent.trim()||null};
