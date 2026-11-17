import { chromium, webkit } from 'playwright';
const engine=process.env.AURGA_BROWSER==='webkit'?'webkit':'chromium';
const browser=await (engine==='webkit'?webkit:chromium).launch({headless:true});
try {
 const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1024,height:768},hasTouch:true,isMobile:true});
 const page=await context.newPage(),errors:string[]=[];
 page.on('pageerror',e=>errors.push(e.message));
 const absoluteURL=new URL(process.env.AURGA_TEST_URL!);absoluteURL.searchParams.set('mouse','absolute');
 await page.goto(absoluteURL.toString());
 await page.waitForSelector('#gate[open]');
 await page.fill('#passcode','test');await page.click('#connect');
 async function ready(){await page.waitForFunction(()=>document.querySelector('#placeholder')?.hidden===true&&controlling,{},{timeout:10000});}
 async function drained(){await page.waitForFunction(()=>reportQueue.length===0&&!inputPending);}
 await ready();
 if(await page.locator('#gate').evaluate(e=>e.open))throw Error('Interstitial still visible');
 if(await page.locator('header,.tools,.note').count())throw Error('Page chrome remains');
 const cookies=await context.cookies();
 if(!cookies.some(c=>c.name==='aurga_session'&&c.httpOnly&&c.secure&&c.sameSite==='Strict'))throw Error('Secure cookie missing');
 await page.focus('#screen');await page.keyboard.press('a');await drained();
 const rect=await page.locator('canvas').boundingBox();if(!rect)throw Error('No canvas');
 await page.mouse.click(rect.x+rect.width/2,rect.y+rect.height/2);await drained();
 await page.mouse.click(rect.x+rect.width/2,rect.y+rect.height/2,{button:'right'});
 if(engine==='chromium')await page.mouse.wheel(0,100);else await page.evaluate(({x,y})=>document.querySelector('#screen').dispatchEvent(new WheelEvent('wheel',{deltaY:100,clientX:x,clientY:y,bubbles:true,cancelable:true})),{x:rect.x+rect.width/2,y:rect.y+rect.height/2});await drained();
 // Real browser touch gesture; verifies the Touch Events path without Pointer Events duplication.
 await page.touchscreen.tap(rect.x+rect.width*.7,rect.y+rect.height*.6);await drained();
 if(engine==='chromium'){
  const cdp=await context.newCDPSession(page);
  await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:rect.x+rect.width*.2,y:rect.y+rect.height*.3}]});
  await cdp.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:rect.x+rect.width*.4,y:rect.y+rect.height*.5}]});
  await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
 }else{
  // WebKit exposes only a native tap in Playwright. Exercise drag handler with
  // DOM events; this does not assert physical iPad drag/keyboard acceptance.
  await page.evaluate(({x,y,w,h})=>{const target=document.querySelector('#screen');for(const [type,fx,fy] of [['touchstart',.2,.3],['touchmove',.4,.5],['touchend',.4,.5]]){const event=new Event(type,{bubbles:true,cancelable:true});Object.defineProperty(event,'changedTouches',{value:[{identifier:17,clientX:x+w*fx,clientY:y+h*fy}]});target.dispatchEvent(event);}}, {x:rect.x,y:rect.y,w:rect.width,h:rect.height});
 }
 await drained();
 const button=await page.locator('#keyboard').boundingBox();if(!button)throw Error('No keyboard button');
 await page.touchscreen.tap(button.x+button.width/2,button.y+button.height/2);
 if(!(await page.locator('#mobile-input').evaluate(e=>document.activeElement===e)))throw Error('Touch gesture did not focus mobile input');
 await page.evaluate(()=>document.querySelector('#mobile-input').dispatchEvent(new InputEvent('beforeinput',{inputType:'insertText',data:'Hi!',bubbles:true,cancelable:true})));
 await page.keyboard.press('Backspace');await page.keyboard.press('Enter');
 // Safari can blur during keyboard transitions: committed software taps must survive.
 await page.locator('#mobile-input').dispatchEvent('blur');await drained();
 await page.click('#info');await drained();
 await page.waitForFunction(()=>deviceInput?.acknowledged===inputDiagnostics.acknowledged&&deviceInput.sent>0&&deviceInput.received===deviceInput.sent);
 const diagnostic=await page.locator('#input-status').textContent();
 if(!diagnostic?.includes('Counts only')||!diagnostic.includes('Input: enabled'))throw Error('Missing input counters');
 const counts=await page.evaluate(()=>inputDiagnostics);
 if(!counts.touch||!counts.keyboard||!counts.text||!counts.sent||!counts.acknowledged)throw Error('Browser event/report counts missing');
 await page.reload();await ready();
 if(await page.inputValue('#passcode'))throw Error('Credential retained in field');
 // Default compatibility path: relative mouse and trackpad touch.
 const relativeURL=new URL(process.env.AURGA_TEST_URL!);
 await page.goto(relativeURL.toString());await ready();
 const r=await page.locator('canvas').boundingBox();if(!r)throw Error('No relative canvas');
 const x=r.x+r.width/2,y=r.y+r.height/2;
 await page.mouse.move(x,y);await drained();
 await page.mouse.move(x+60,y+20);await drained();
 await page.mouse.move(x-60,y-20);await drained();
 await page.mouse.down();await drained();await page.mouse.move(x,y);await drained();await page.mouse.up();await drained();
 await page.touchscreen.tap(x,y);await drained();
 const touchEvent=async(type:string,dx=0)=>{await page.evaluate(({type,x,y})=>{const e=new Event(type,{bubbles:true,cancelable:true});Object.defineProperty(e,'changedTouches',{value:[{identifier:71,clientX:x,clientY:y}]});document.querySelector('#screen').dispatchEvent(e);},{type,x:x+dx,y});};
 await touchEvent('touchstart');await page.waitForTimeout(550);await drained();await touchEvent('touchmove',45);await drained();await touchEvent('touchend',45);await drained();
 const sent=await page.evaluate(()=>inputDiagnostics.sent);
 await touchEvent('touchstart');await touchEvent('touchcancel');await page.waitForTimeout(550);
 if(await page.evaluate(()=>inputDiagnostics.sent)!==sent)throw Error('Cancelled touch sent a delayed click');
 await page.waitForFunction(()=>deviceInput?.motion>0&&deviceInput?.clicks>0&&deviceInput?.relative>0);
 if(errors.length)throw Error(errors.join('; '));
 console.log(`PASS (${engine}): real HTTPS/UDP bridge, native touch tap, drag handler/release, touch keyboard focus, atomic software-key taps, Enter/Backspace, blur queue preservation and cookie reload`);
 await context.close();
}finally{await browser.close();}
