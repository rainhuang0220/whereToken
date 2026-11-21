import { chromium, webkit, firefox } from '@playwright/test';
import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { join, resolve, extname } from 'node:path';
const root=resolve('public-profile');
const server=http.createServer(async(req,res)=>{
 const pathname=new URL(req.url,'http://127.0.0.1').pathname;
 const prefix='/whereToken/profile/';
 if(!pathname.startsWith(prefix)){res.writeHead(404).end('unknown');return;}
 const name=pathname.slice(prefix.length)||'index.html';
 if(name.includes('..')){res.writeHead(403).end('forbidden');return;}
 try {
  const data=await readFile(join(root,name));
  const suffix=extname(name),mime={
   '.js':'text/javascript','.css':'text/css','.jpg':'image/jpeg',
   '.json':'application/json','.html':'text/html','.svg':'image/svg+xml'
  }[suffix]||'application/octet-stream';
  res.writeHead(200,{'content-type':mime});res.end(data);
 }catch(e){res.writeHead(404).end('missing '+name);}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const url='http://127.0.0.1:'+server.address().port+'/whereToken/profile/';
for(const [name,engine,options] of [
 ['chrome',chromium,{headless:false,channel:'chrome'}],
 ['webkit',webkit,{headless:true}],
 ['firefox',firefox,{headless:true}]
]){
 let browser;
 try{
  browser=await engine.launch(options);
  const page=await browser.newPage({viewport:{width:1440,height:1000}});
  const errors=[],requests=[],warnings=[];
  page.on('pageerror',e=>errors.push(e.stack||e.message));
  page.on('console',m=>{
   if(m.type()==='error')errors.push(m.text());
   if(m.type()==='warning')warnings.push(m.text())
  });
  page.on('request',r=>requests.push(new URL(r.url()).pathname));
  await page.goto(url,{waitUntil:'domcontentloaded',timeout:30000});
  await page.waitForFunction(()=>document.querySelector('#hero-value')?.textContent?.trim()!=='—',{timeout:20000});
  const initial=await page.evaluate(()=>({
   palette:document.documentElement.getAttribute('data-activity-palette'),
   hero:document.querySelector('#hero-value').textContent,
   asOf:document.querySelector('#freshness').dataset.asOfDate,
   snap:window.__WT_NEWSprint?.snapshot?.as_of_date
  }));
  // Newsprint is deliberately a visitor choice, not a forced publisher change.
  await page.getByRole('tab',{name:'Newsprint',exact:true}).click();
  await page.waitForFunction(()=>window.__WT_PAPER3D__?.getDiagnostics()?.ready,{timeout:35000}).catch(()=>{});
  await page.waitForTimeout(1600);
  const before=await page.evaluate(()=>({
   paper:window.__WT_PAPER3D__?.getDiagnostics(),
   hero:document.querySelector('#hero-value')?.textContent,
   section:document.querySelector('#wt-physical-paper')?.hidden,
   print:document.querySelector('#wt-paper-flat-image')?.naturalWidth,
   palette:document.documentElement.getAttribute('data-activity-palette')
  }));
  await page.screenshot({path:'profile-paper/review-'+name+'-newsprint.png'});
  if(before.paper?.ready&&before.paper?.active){
   const scene=page.locator('#wt-paper-scene');
   const rect=await scene.boundingBox();
   await page.mouse.move(rect.x+rect.width*.68,rect.y+rect.height*.17);
   await page.mouse.down();
   await page.mouse.move(rect.x+rect.width*.47,rect.y+rect.height*.39,{steps:18});
   await page.waitForTimeout(400);
   const dragging=await page.evaluate(()=>window.__WT_PAPER3D__.getDiagnostics());
   await page.screenshot({path:'profile-paper/review-'+name+'-folded.png'});
   await page.mouse.up();
   await page.waitForTimeout(700);
   await page.locator('[data-paper-flat]').click();
   const flatVisible=await page.locator('#wt-paper-flat-image').isVisible();
   await page.locator('[data-paper-flat]').click();
   await page.getByRole('tab',{name:'7d',exact:true}).click();
   await page.waitForTimeout(700);
   const changed=await page.evaluate(()=>({
    date:window.__WT_NEWSprint?.snapshot?.as_of_date,
    range:window.__WT_NEWSprint?.selection?.range,
    metric:window.__WT_NEWSprint?.selection?.metric,
    paper:window.__WT_PAPER3D__?.getDiagnostics()
   }));
   await page.getByRole('tab',{name:'Cobalt',exact:true}).click();
   await page.waitForTimeout(150);
   const other=await page.evaluate(()=>({
    palette:document.documentElement.getAttribute('data-activity-palette'),
    hidden:document.querySelector('#wt-physical-paper').hidden,
    hero:document.querySelector('#hero-value').textContent
   }));
   console.log(name,JSON.stringify({initial,before,dragging:{phase:dragging.phase,angle:dragging.angleDegrees},
    flatVisible,changed,other,errors,requests,warnings}));
  }else console.log(name,JSON.stringify({initial,before,errors,requests,warnings}));
 }catch(e){console.log(name,'FAILED',e.stack);}
 finally{await browser?.close()}
}
await new Promise(resolve=>server.close(resolve));
