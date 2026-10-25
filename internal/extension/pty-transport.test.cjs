const test=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const scope={URL};vm.runInNewContext(fs.readFileSync(__dirname+'/pty-transport.js','utf8'),scope);
test('consumer endpoint is bound to installed adapter and exact origin',()=>{
 const e={adapter:'ttyd',origin:'https://terminal.example.test',pageUrl:'https://terminal.example.test/',socketUrl:'wss://terminal.example.test/ws'};
 const valid=x=>scope.PTYTransport.validateEndpoint(x,e.origin,'ttyd');assert.equal(valid(e),true);
 for(const change of [{adapter:'other'},{origin:'https://other.test'},{socketUrl:'wss://other.test/ws'},{pageUrl:'https://user:secret@terminal.example.test/'},{socketUrl:'ws://terminal.example.test/ws'},{pageUrl:'https://terminal.example.test/#code'}])assert.equal(valid({...e,...change}),false);
});
