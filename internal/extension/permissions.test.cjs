// The former settings URL now redirects to the registry-driven BAB UI.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs');
test('old settings URL has no independent integration controls',()=>{const html=fs.readFileSync(__dirname+'/permissions.html','utf8');assert.ok(html.includes('url=bab.html'));assert.ok(!html.includes('<input') && !html.includes('<select'));});
