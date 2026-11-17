// Optional browser acceptance test; Bun/Playwright are development tools only.
import { chromium } from 'playwright';
const base=process.env.AURGA_TEST_URL??'https://127.0.0.1:9443';
const fixture=await Bun.file(process.env.AURGA_TEST_FRAME??'/workspace/tmp/aurga-web/tests/blue.h264').bytes();
const browser=await chromium.launch({headless:true});
try {
 const context=await browser.newContext({ignoreHTTPSErrors:true});const page=await context.newPage();const errors:string[]=[];const commands:any[]=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.route('**/api/device',route=>route.fulfill({json:{name:'Synthetic AURGA',ip:'192.168.1.94',firmware:'test'}}));
 await page.routeWebSocket('**/ws',socket=>{socket.onMessage(message=>{if(typeof message!=='string')return;const command=JSON.parse(message);commands.push(command);if(command.type==='connect'){socket.send(JSON.stringify({type:'codec',codec:'avc'}));socket.send(JSON.stringify({type:'status',message:'Connected — synthetic video'}));socket.send(Buffer.from(fixture));}});});
 await page.goto(base);await page.fill('#passcode','test');await page.click('#connect');try{await page.waitForFunction(()=>document.querySelector('canvas')?.width===640&&document.querySelector('#placeholder')?.hidden===true,{},{timeout:15000});}catch(e){console.error('Browser state:',await page.locator('body').innerText(), 'errors:', errors, 'commands:',commands.map(x=>x.type), await page.evaluate(()=>({parameters:Array.from(parameters.keys()),state:decoder?.state,configured,needKey,queue:queuedVideo,codec,canvas:[canvas.width,canvas.height]})));throw e;}
 await page.focus('#screen');await page.keyboard.press('a');if(commands.some(x=>x.type==='input'))throw Error('Input emitted in read-only mode');
 await page.check('#control');await page.focus('#screen');await page.keyboard.press('a');await page.waitForTimeout(100);
 if(!commands.some(x=>x.type==='input'&&x.report[0]===1&&x.report[3]===4))throw Error('Keyboard down report missing');
 if(!commands.some(x=>x.type==='input'&&x.report[0]===1&&x.report.slice(3).every((v:number)=>v===0)))throw Error('Keyboard release missing');
 const rect=await page.locator('canvas').boundingBox();if(!rect)throw Error('No canvas');await page.mouse.click(rect.x+rect.width/2,rect.y+rect.height/2);await page.waitForTimeout(100);
 if(!commands.some(x=>x.type==='input'&&x.report[0]===4&&x.report[1]===1))throw Error('Mouse down missing');
 await page.uncheck('#control');await page.click('#disconnect');await page.waitForTimeout(100);if(errors.length)throw Error(errors.join('; '));
 console.log('PASS: synthetic H.264 decode/render, read-only guard, keyboard, mouse, releases and disconnect; no page errors');
 await context.close();
} finally {await browser.close();}
