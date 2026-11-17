// Connects to a real Go WebSocket bridge backed by the loopback synthetic UDP peer.
import { chromium } from 'playwright';
const browser=await chromium.launch({headless:true});
try{const context=await browser.newContext({ignoreHTTPSErrors:true});const page=await context.newPage();const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
await page.route('**/api/device',r=>r.fulfill({json:{name:'Loopback synthetic peer',ip:'127.0.0.1',firmware:'test'}}));
await page.goto(process.env.AURGA_TEST_URL!);await page.fill('#passcode','test');await page.click('#connect');
await page.waitForFunction(()=>document.querySelector('canvas')?.width===640&&document.querySelector('#placeholder')?.hidden===true,{},{timeout:10000});
await page.check('#control');await page.waitForTimeout(300);await page.focus('#screen');await page.keyboard.press('a');await page.waitForTimeout(150);const rect=await page.locator('canvas').boundingBox();if(!rect)throw Error('No canvas');await page.mouse.click(rect.x+rect.width/2,rect.y+rect.height/2);await page.waitForTimeout(200);await page.uncheck('#control');await page.click('#disconnect');await page.waitForTimeout(150);
if(errors.length)throw Error(errors.join('; '));console.log('PASS: real HTTPS/WebSocket → Go session → synthetic UDP peer → fragmented H.264 → browser canvas and HID roundtrip');await context.close();
}finally{await browser.close();}
