// Exercise generated request handling with deterministic socket events, including
// one-byte UTF-8 fragments and errors on the authenticated wrapper itself.
const fs=require('fs'),path=require('path'),{EventEmitter}=require('events');
const root=path.resolve(process.argv[2]||'clients/typescript');
let events=[],wrapper;
class Socket extends EventEmitter {
  connect(port,host,ready){queueMicrotask(ready);return this;}
  setNoDelay(){}
  write(){for(const [name,data] of events)queueMicrotask(()=>this.emit(name,data));return true;}
  end(){}
  destroy(){this.destroyed=true;}
}
const moduleShim={exports:{}};
const fakeRequire=name=>name==='net'?{Socket}:name==='./ViiperDevice'?{ViiperDevice:class{}}:
  name==='./utils/auth'?{performAuthHandshake:async()=>wrapper=new Socket()}:require(name);
new Function('require','module','exports',fs.readFileSync(path.join(root,'dist/ViiperClient.js'),'utf8'))(fakeRequire,moduleShim,moduleShim.exports);
const Client=moduleShim.exports.ViiperClient;
const reset=()=>['error',new Error('synthetic connection reset')];
const bytes=text=>[...Buffer.from(text)].map(b=>['data',Buffer.from([b])]);
(async()=>{
  let passed=0;
  for(const encrypted of [false,true]) for(const [data,fail] of [
    [[...bytes('{"server":"é🎮","version":"test"}\n'),reset()],false],
    [[...bytes('{\n"server":"é🎮",\n"version":"test"\n}\n'),reset()],false],
    [[...bytes('{"server":"é🎮","version":"test"}'),['end']],false],
    [[...bytes('{"server":"partial"'),reset()],true],
    [[...bytes('{"server":"partial"'),['end']],true],
    [[...bytes('{"status":409,"title":"Conflict"}\n')],true],
    [[...bytes('not JSON\n')],true],
    [[['close']],true],
  ]) {
    events=data;
    const query=new Client('synthetic',3242,encrypted?'synthetic-key':'').ping();
    let result,rejected=false;
    try {
      result=await Promise.race([query,new Promise((_,reject)=>setTimeout(()=>reject(new Error('timeout')),1000))]);
    }catch(error){if(!fail || error.message==='timeout')throw error;rejected=true;}
    if(!rejected && (fail || result.server!=='é🎮'))throw new Error('incorrect result');
    passed++;
  }
  console.log(`PASS: ${passed} generated TypeScript reply cases`);
})().catch(e=>{console.error(e);process.exitCode=1;});
