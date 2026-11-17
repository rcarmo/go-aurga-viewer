import { chromium, webkit } from 'playwright';
if(!process.env.AURGA_TEST_FRAME)throw Error('Set AURGA_TEST_FRAME or run make browser-test');
const fixture=await Bun.file(process.env.AURGA_TEST_FRAME).bytes();const browser=await (process.env.AURGA_BROWSER==='webkit'?webkit:chromium).launch({headless:true});
try{const context=await browser.newContext({ignoreHTTPSErrors:true});const page=await context.newPage();const errors:string[]=[];const commands:any[]=[];let authorized=false,peer:any;
await page.addInitScript(()=>{window.__draws=0;const draw=CanvasRenderingContext2D.prototype.drawImage;CanvasRenderingContext2D.prototype.drawImage=function(...args){window.__draws++;return Reflect.apply(draw,this,args);};});page.on('pageerror',e=>errors.push(e.message));await page.route('**/api/auth',r=>{if(r.request().method()==='POST')authorized=true;return r.fulfill({status:authorized?204:401});});
await page.routeWebSocket('**/ws',socket=>{peer=socket;socket.onMessage(message=>{if(typeof message!=='string')return;const command=JSON.parse(message);commands.push(command);if(command.type==='connect'){socket.send(JSON.stringify({type:'codec',codec:'avc'}));socket.send(JSON.stringify({type:'status',message:'Connected'}));socket.send(Buffer.from([0,...fixture]));socket.send(JSON.stringify({type:'control',message:'true'}));socket.send(JSON.stringify({type:'audio',channels:2,sampleRate:48000}));socket.send(Buffer.alloc(1921));const pcm=Buffer.alloc(1921);pcm[0]=1;socket.send(pcm);}if(command.type==='input')socket.send(JSON.stringify({type:'input-ack'}));});});
await page.goto(process.env.AURGA_TEST_URL??'http://127.0.0.1:19443');await page.fill('#passcode','test');await page.click('#connect');await page.waitForFunction(()=>document.querySelector('canvas')?.width===640&&document.querySelector('#placeholder')?.hidden===true&&controlling,{},{timeout:10000});await page.focus('#screen');await page.keyboard.press('a');await page.waitForTimeout(100);if(!commands.some(x=>x.type==='input'&&x.report[3]===4))throw Error('Default keyboard not active');await page.click('#keyboard');await page.fill('#mobile-input','Hi!');await page.waitForFunction(()=>reportQueue.length===0&&!inputPending);if(!commands.some(x=>x.type==='input'&&x.report[1]===2&&x.report[3]===11))throw Error('Mobile uppercase missing');if(!(await page.evaluate(()=>audioTime>0)))throw Error('PCM not scheduled');// Signed relative movement must be split, not silently clipped at 127.
const motionStart=commands.length;
await page.evaluate(()=>{relativeDX=300;relativeDY=-300;mouse(0);});await page.waitForFunction(()=>!inputPending&&!reportQueue.length);
const motion=commands.slice(motionStart).filter(c=>c.type==='input'&&c.report[0]===2);
const signed=(n:number)=>n>127?n-256:n;
if(motion.reduce((n,c)=>n+signed(c.report[2]),0)!==300||motion.reduce((n,c)=>n+signed(c.report[3]),0)!==-300)throw Error('Relative displacement clipped');
// Simulate Safari interrupting an already-unlocked context. Preserve format
// and rebuild synchronously from the next trusted gesture, then schedule PCM.
const scheduled=await page.evaluate(()=>inputDiagnostics.audioScheduled);
await page.evaluate(()=>{window.__oldAudio=audioContext;Object.defineProperty(audioContext,'state',{get:()=> 'interrupted',configurable:true});audioContext.dispatchEvent(new Event('statechange'));});
await page.click('#info');await page.waitForFunction(()=>audioContext!==window.__oldAudio&&audioContext.state==='running'&&audioFormat?.sampleRate===48000);
const resumedPCM=Buffer.alloc(1921);resumedPCM[0]=1;peer.send(resumedPCM);await page.waitForFunction(n=>inputDiagnostics.audioScheduled>n,scheduled);
// Uncaptured mouse fallback (e.g. iPad Safari): dwelling at an edge pans;
// leaving the edge stops it. No real KVM is contacted by this routed socket.
await page.evaluate(()=>screen.requestPointerLock=undefined);
const edge=await page.locator('canvas').boundingBox();if(!edge)throw Error('No video area');
const edgeStart=commands.length;await page.mouse.move(edge.x+edge.width-2,edge.y+edge.height/2);await page.waitForTimeout(150);await page.waitForFunction(()=>!inputPending&&!reportQueue.length);
if(!commands.slice(edgeStart).some(c=>c.type==='input'&&c.report[0]===2&&c.report[2]>0&&c.report[2]<128))throw Error('Edge pan did not move');
await page.mouse.move(edge.x+edge.width/2,edge.y+edge.height/2);await page.waitForFunction(()=>!inputPending&&!reportQueue.length);
const stopped=commands.filter(c=>c.type==='input').length;await page.waitForTimeout(150);if(commands.filter(c=>c.type==='input').length!==stopped)throw Error('Edge pan did not stop');
console.log('PASS: full signed movement, bounded mouse edge pan and interrupted audio gesture recovery');
// Reproduce a stalled decoder and more than 32 incoming video messages.
const draws=await page.evaluate(()=>window.__draws);
await page.evaluate(()=>{const original=VideoDecoder.isConfigSupported.bind(VideoDecoder);const wait=new Promise(resolve=>window.__releaseDecode=()=>{VideoDecoder.isConfigSupported=original;resolve();});VideoDecoder.isConfigSupported=async config=>{await wait;return original(config);};resetVideo();});
peer.send(Buffer.from([0,...fixture]));await page.waitForFunction(()=>queuedVideo>0);
for(let i=0;i<64;i++)peer.send(Buffer.from([0,...fixture]));
await page.waitForFunction(()=>inputDiagnostics.videoRecoveries>0||ws.readyState!==WebSocket.OPEN);
if(!(await page.evaluate(()=>ws.readyState===WebSocket.OPEN&&!gate.open)))throw Error('Video overload disconnected viewer');
const acknowledged=await page.evaluate(()=>inputDiagnostics.acknowledged);
await page.focus('#screen');await page.keyboard.press('b');await page.waitForFunction(n=>inputDiagnostics.acknowledged>n,acknowledged);
await page.evaluate(()=>window.__releaseDecode());await page.waitForFunction(()=>queuedVideo===0);
peer.send(Buffer.from([0,...fixture]));await page.waitForFunction(n=>window.__draws>n,draws);
const recoveredDraws=await page.evaluate(()=>window.__draws);peer.send(JSON.stringify({type:'video-reset'}));peer.send(Buffer.from([0,...fixture]));await page.waitForFunction(n=>window.__draws>n,recoveredDraws);
if(commands.filter(x=>x.type==='connect').length!==1)throw Error('Recovery reconnected the WebSocket');
if(errors.length)throw Error(errors.join('; '));console.log('PASS: stalled decoder, video burst and server gap recover without reconnect; input ACKs continue');console.log('PASS: minimal interstitial UI, H.264 rendering, default input, acknowledged mobile ASCII typing and PCM scheduling');await context.close();}finally{await browser.close();}
