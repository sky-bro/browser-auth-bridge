const response = await ctx.fetch('/clientarea.php?action=services');
if (!response.ok) return {error: {code: 'http_error'}};
const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
if (doc.querySelector('form input[type="password"],form[action*="dologin.php"]')) return {logged_in: false};
const authenticated = !!doc.querySelector('a[href*="logout.php"],a[href*="action=productdetails"]');
return {logged_in: authenticated ? true : null};
