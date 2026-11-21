import * as THREE from 'three';
import html2canvas from 'html2canvas';
import { PaperFoldSheet } from './scene/paper-foldsheet.js';

/*
 * Hidden single-sheet interaction for the existing, accessible Profile DOM.
 * No second report, no added controls, no layout mutation when idle.
 * Rasterize THE PRESENT REPORT only when a reader grabs its edge; fold the
 * resulting exact content UV on the D4 mesh. Return to live DOM on recovery.
 */
const main=document.getElementById('main');
const W=7.8;
const clamp=(n,a,b)=>Math.max(a,Math.min(b,n));
const fine=matchMedia('(pointer:fine)');
const reduced=matchMedia('(prefers-reduced-motion:reduce)');
const small=matchMedia('(max-width:700px)');
let state='idle', pending=null, model=null, renderer=null, scene=null;
let camera=null,front=null,back=null,overlay=null,frame=0,lastTime=0;
let paperTexture=null,paperHeight=null,layout=null,captureEpoch=0,drag=null;
let savedMemories=[],scars=[],lastError=null;
let cachedFrame=null,cachedRevision=-1,snapshotInFlight=null,cacheTimer=0;
const inkTile=new Image();
inkTile.src='./assets/newsprint-ink.png';
let inkPixelData=null;

function preservePrintInCapture(snapshot){
 if(!inkTile.complete||!inkTile.naturalWidth)return snapshot;
 try {
  if(!inkPixelData){
   const tile=document.createElement('canvas');
   tile.width=inkTile.naturalWidth;tile.height=inkTile.naturalHeight;
   const tileCtx=tile.getContext('2d',{willReadFrequently:true});
   tileCtx.drawImage(inkTile,0,0);
   inkPixelData={width:tile.width,height:tile.height,
     pixels:tileCtx.getImageData(0,0,tile.width,tile.height).data};
  }
  const ctx=snapshot.getContext('2d',{willReadFrequently:true});
  const image=ctx.getImageData(0,0,snapshot.width,snapshot.height);
  const data=image.data,mask=inkPixelData.pixels;
  const tileW=inkPixelData.width,tileH=inkPixelData.height;
  // Only pigment-dark pixels are affected; already-light paper does not
  // acquire random noise. Ink defects stay attached to each glyph when UVs fold.
  for(let y=0;y<snapshot.height;y++){
   const maskRow=(y%tileH)*tileW*4;
   const pixelRow=y*snapshot.width*4;
   for(let x=0;x<snapshot.width;x++){
    const p=pixelRow+x*4;
    const luminance=(data[p]*.30+data[p+1]*.59+data[p+2]*.11);
    if(luminance>180||data[p+3]<200)continue;
    const a=mask[maskRow+(x%tileW)*4+3];
    if(a>=248)continue;
    const drop=(255-a)/255*.86*Math.min(1,(205-luminance)/150);
    if(drop<=0)continue;
    data[p]=Math.round(data[p]+(245-data[p])*drop);
    data[p+1]=Math.round(data[p+1]+(241-data[p+1])*drop);
    data[p+2]=Math.round(data[p+2]+(230-data[p+2])*drop);
   }
  }
  ctx.putImageData(image,0,0);
 }catch(error){
  console.warn('Print pigment capture fallback',error);
 }
 return snapshot;
}


function allowed(){
 return document.documentElement.dataset.activityPalette==='newsprint' &&
   fine.matches && !reduced.matches && !small.matches && !document.hidden;
}
function getSheet(){
 const r=main.getBoundingClientRect();
 return {x:r.left,y:r.top,width:r.width,height:r.height,
   top:r.top+window.scrollY,left:r.left+window.scrollX,H:W*r.height/r.width};
}
function cornerAt(e){
 if(!allowed()||e.button!==0||e.pointerType==='touch')return null;
 if(e.target.closest('button,a,input,textarea,select,summary,[role=tab],[role=button],[contenteditable]'))return null;
 const r=getSheet();
 if(r.width<400||r.height<300||!main.contains(e.target))return null;
 const px=e.clientX-r.x,py=e.clientY-r.y;
 if(px<0||py<0||px>r.width||py>r.height)return null;
 const edgeX=Math.min(px,r.width-px);
 const edgeY=Math.min(py,r.height-py);
 // No pointer traps in the report interior. Only the unmarked paper edge.
 if(!(edgeX<26 || (edgeY<30&&edgeX<65)))return null;
 return {u:clamp(px/r.width,.001,.999),v:clamp(1-py/r.height,.001,.999),rect:r};
}
function clearScars(){
 for(const mark of scars)mark.remove();
 scars=[];
}
function renderScars(){
 clearScars();
 if(!layout||!savedMemories.length)return;
 const rect=main.getBoundingClientRect();
 for(const record of savedMemories.slice(-3)){
  const h=record.hinge;
  const start={x:(h.start.x+W/2)/W*rect.width,
    y:(layout.H/2-h.start.y)/layout.H*rect.height};
  const end={x:(h.end.x+W/2)/W*rect.width,
    y:(layout.H/2-h.end.y)/layout.H*rect.height};
  const len=Math.hypot(end.x-start.x,end.y-start.y);
  if(len<20)continue;
  const div=document.createElement('div');
  div.className='wt-newsprint-scar';
  Object.assign(div.style,{
   position:'absolute',pointerEvents:'none',zIndex:'2',
   left:start.x+'px',top:start.y+'px',width:len+'px',height:'2px',
   transformOrigin:'0 50%',
   transform:'rotate('+Math.atan2(end.y-start.y,end.x-start.x)+'rad)',
   background:'linear-gradient(to bottom,rgba(21,19,15,.04),rgba(24,21,16,.19) 48%,rgba(255,250,237,.25) 70%,transparent)',
   opacity:clamp(record.strength*.38,.08,.26).toString(),
  });
  main.appendChild(div);scars.push(div);
 }
}
function dispose(){
 if(frame)cancelAnimationFrame(frame);
 frame=0;lastTime=0;
 if(overlay)overlay.remove();
 if(renderer){renderer.dispose();renderer.forceContextLoss();}
 if(paperTexture)paperTexture.dispose();
 if(paperHeight)paperHeight.dispose();
 if(front){front.material.dispose();}
 if(back){back.material.dispose();}
 if(model){model.foldedGeometry.dispose();model.restingGeometry.dispose();}
 overlay=renderer=scene=camera=front=back=paperTexture=paperHeight=model=null;
 main.style.opacity='';
 drag=null;
}
function restore(){
 if(model?.memories?.length)savedMemories=model.memories.slice();
 dispose();state='idle';pending=null;renderScars();
 cachedFrame=null;cachedRevision=-1;
 ++captureEpoch;
 schedulePrewarm(500);
}
function invalidate(){
 ++captureEpoch;
 cachedFrame=null;cachedRevision=-1;
 if(state!=='idle')restore();
 if(!allowed())clearScars();
 schedulePrewarm(500);
}
function schedulePrewarm(delay=650){
 if(cacheTimer)clearTimeout(cacheTimer);
 cacheTimer=0;
 if(!allowed()||state!=='idle')return;
 cacheTimer=setTimeout(()=>{cacheTimer=0;prewarm();},delay);
}
async function captureNativePage(){
 // Capture only current native report DOM; never synthesize a duplicate.
 const r=main.getBoundingClientRect();
 const scale=Math.min(1.1,2400/Math.max(1,r.height),1700/Math.max(1,r.width));
 const snapshot=await html2canvas(main,{
  backgroundColor:'#f3f0e8',scale,logging:false,
  useCORS:false,allowTaint:false,removeContainer:true,
  onclone:doc=>doc.documentElement.classList.add('wt-fold-capture')
 });
 preservePrintInCapture(snapshot);
 return snapshot;
}
function prewarm(){
 if(!allowed()||state!=='idle')return;
 const revision=captureEpoch;
 if(cachedFrame&&cachedRevision===revision)return;
 if(snapshotInFlight&&snapshotInFlight.revision===revision)return;
 const promise=captureNativePage();
 snapshotInFlight={revision,promise};
 promise.then(snapshot=>{
  if(revision===captureEpoch&&allowed()){
   cachedFrame=snapshot;cachedRevision=revision;
  }
  if(snapshotInFlight?.revision===revision)snapshotInFlight=null;
 }).catch(error=>{
  if(snapshotInFlight?.revision===revision)snapshotInFlight=null;
  lastError=error?.message||String(error);
 });
}
async function snapshotForRevision(revision){
 if(cachedFrame&&cachedRevision===revision)return cachedFrame;
 if(snapshotInFlight&&snapshotInFlight.revision===revision)
   return snapshotInFlight.promise;
 const promise=captureNativePage();
 snapshotInFlight={revision,promise};
 try{
  const image=await promise;
  if(revision===captureEpoch&&allowed()){
   cachedFrame=image;cachedRevision=revision;
  }
  return image;
 }finally{
  if(snapshotInFlight?.revision===revision)snapshotInFlight=null;
 }
}
function shadow(){
 const receiving=new THREE.Mesh(new THREE.PlaneGeometry(W,layout.H),
   new THREE.ShadowMaterial({opacity:.14}));
 receiving.position.z=-.07;receiving.receiveShadow=true;
 scene.add(receiving);
}
function initGeometry(snapshot){
 layout=getSheet();model=new PaperFoldSheet({width:W,height:layout.H});
 if(savedMemories.length){
  model.memories=savedMemories.slice();
  model.refreshRestingGeometry();
 }
 overlay=document.createElement('canvas');
 overlay.setAttribute('aria-hidden','true');
 overlay.className='wt-fold-overlay';
 Object.assign(overlay.style,{
  position:'absolute',left:layout.left+'px',top:layout.top+'px',
  width:layout.width+'px',height:layout.height+'px',
  zIndex:'8',margin:'0',padding:'0',
  pointerEvents:'auto',touchAction:'none',background:'transparent'
 });
 document.body.appendChild(overlay);
 renderer=new THREE.WebGLRenderer({canvas:overlay,alpha:true,antialias:true,powerPreference:'default'});
 renderer.setClearColor(0xffffff,0);
 renderer.setPixelRatio(Math.min(devicePixelRatio||1,1.15));
 renderer.setSize(layout.width,layout.height,false);
 renderer.toneMapping=THREE.ACESFilmicToneMapping;
 renderer.toneMappingExposure=1.30;
 renderer.outputColorSpace=THREE.SRGBColorSpace;
 renderer.shadowMap.enabled=true;
 renderer.shadowMap.type=THREE.PCFSoftShadowMap;
 scene=new THREE.Scene();
 camera=new THREE.OrthographicCamera(-W/2,W/2,layout.H/2,-layout.H/2,.1,60);
 camera.position.set(0,0,24);camera.lookAt(0,0,0);
 scene.add(new THREE.AmbientLight('#ffffff',1.40));
 const sun=new THREE.DirectionalLight('#fff9ef',2.1);
 sun.position.set(-6,7,11);sun.castShadow=true;
 sun.shadow.mapSize.set(1024,1024);
 const bound=Math.max(W,layout.H)*1.3;
 Object.assign(sun.shadow.camera,{
  left:-bound,right:bound,top:bound,bottom:-bound,near:.1,far:60});
 sun.shadow.bias=-.00025;sun.shadow.normalBias=.022;scene.add(sun);
 const bounce=new THREE.DirectionalLight('#e9ecee',.56);
 bounce.position.set(8,-3,8);scene.add(bounce);
 paperTexture=new THREE.CanvasTexture(snapshot);
 paperTexture.colorSpace=THREE.SRGBColorSpace;
 paperTexture.anisotropy=Math.min(4,renderer.capabilities.getMaxAnisotropy());
 paperHeight=new THREE.TextureLoader().load('./assets/newsprint-height.jpg');
 paperHeight.wrapS=paperHeight.wrapT=THREE.RepeatWrapping;
 paperHeight.repeat.set(2.0,2.0*layout.H/W);
 const face=new THREE.MeshStandardMaterial({
   map:paperTexture,roughness:1,metalness:0,side:THREE.FrontSide,
   bumpMap:paperHeight,bumpScale:.020
 });
 const reverse=new THREE.MeshStandardMaterial({
   color:'#f1ede3',roughness:1,metalness:0,side:THREE.BackSide,
   bumpMap:paperHeight,bumpScale:.025
 });
 front=new THREE.Mesh(model.renderGeometry,face);front.castShadow=true;
 back=new THREE.Mesh(model.renderGeometry,reverse);
 scene.add(front,back);shadow();
 overlay.addEventListener('pointerdown',onOverlayDown);
 overlay.addEventListener('pointermove',onOverlayMove);
 overlay.addEventListener('pointerup',onOverlayUp);
 overlay.addEventListener('pointercancel',onOverlayUp);
 overlay.addEventListener('lostpointercapture',onOverlayUp);
 main.style.opacity='0';
 clearScars();
}
function tick(now){
 frame=0;
 if(!renderer||!model||state==='idle')return;
 const dt=lastTime?clamp((now-lastTime)/1000,1/240,.16):1/60;
 lastTime=now;model.step(dt);
 if(front.geometry!==model.renderGeometry){
   front.geometry=model.renderGeometry;back.geometry=model.renderGeometry;
 }
 renderer.render(scene,camera);
 if(model.phase==='resting'&&!drag){
   restore();return;
 }
 if(model.phase==='latched'&&!drag){
   // The print stays folded until the user unfolds it or clicks elsewhere.
   return;
 }
 frame=requestAnimationFrame(tick);
}
function wake(){
 if(renderer&&!frame)frame=requestAnimationFrame(tick);
}
function setFoldMotion(x,y,t){
 if(!drag||!model)return;
 const dx=(x-drag.x)*W/layout.width*.82;
 const dy=-(y-drag.y)*layout.H/layout.height*.82;
 model.dragDelta(dx,dy,t);wake();
}
function planeUVAt(e){
 const r=overlay.getBoundingClientRect();
 return {
  x:(e.clientX-r.left)/r.width*W-W/2,
  y:layout.H/2-(e.clientY-r.top)/r.height*layout.H
 };
}
function acquireFold(e){
 if(!model)return false;
 const p=planeUVAt(e);
 const ray=new THREE.Raycaster();
 ray.setFromCamera(new THREE.Vector2(p.x/(W/2),p.y/(layout.H/2)),camera);
 const hits=ray.intersectObjects([front,back],false);
 const flap=hits.find(hit=>{
  const u=hit.uv;if(!u||!model.hinge)return false;
  return model.q({x:(u.x-.5)*W,y:(u.y-.5)*layout.H})>model.bandWidth*.3;
 });
 const match=flap||hits[0];
 const u=match?.uv?.x??clamp((p.x+W/2)/W,0,1);
 const v=match?.uv?.y??clamp((p.y+layout.H/2)/layout.H,0,1);
 return model.grabUV(u,v,e.timeStamp);
}
function onOverlayDown(e){
 if(!allowed()){restore();return;}
 if(e.button!==0)return;
 if(!acquireFold(e)){restore();return;}
 e.preventDefault();
 drag={id:e.pointerId,x:e.clientX,y:e.clientY};
 overlay.setPointerCapture(e.pointerId);
 state='folding';wake();
}
function onOverlayMove(e){
 if(!drag||e.pointerId!==drag.id)return;
 setFoldMotion(e.clientX,e.clientY,e.timeStamp);
}
function onOverlayUp(e){
 if(!drag||e.pointerId!==drag.id)return;
 drag=null;
 model.release(e.timeStamp);
 state='folding';wake();
}
function onMainMove(e){
 if(!pending||pending.id!==e.pointerId)return;
 pending.x=e.clientX;pending.y=e.clientY;
 if(model&&drag)setFoldMotion(e.clientX,e.clientY,e.timeStamp);
}
function onMainUp(e){
 if(!pending||pending.id!==e.pointerId)return;
 pending.x=e.clientX;pending.y=e.clientY;
 pending.pressed=false;
 if(model&&drag){drag=null;model.release(e.timeStamp);wake();}
}
async function captureAndFold(input,epoch){
 try {
  const snapshot=await snapshotForRevision(epoch);
  if(epoch!==captureEpoch||!allowed()||!pending)return;
  try{initGeometry(snapshot);}catch(err){
   lastError=err?.message||String(err);restore();return;
  }
  const grab=input.grab;
  if(!model.grabUV(grab.u,grab.v,performance.now())){
    restore();return;
  }
  drag={id:input.id,x:input.startX,y:input.startY};
  state='folding';
  if(pending.pressed){
   setFoldMotion(pending.x,pending.y,performance.now());
  }else{
   const moved=Math.hypot(pending.x-input.startX,pending.y-input.startY);
   if(moved<30){
    // Even a plain click on an unseen corner reveals a tiny paper lift.
    const ix=grab.u>.5?-1:1,iy=grab.v>.5?-1:1;
    model.dragDelta(ix*.15,iy*.15,performance.now());
   }else{
    setFoldMotion(pending.x,pending.y,performance.now());
   }
   drag=null;model.release(performance.now());pending=null;
  }
  wake();
 }catch(error){
  lastError=error?.message||String(error);
  restore();
 }
}
function onMainDown(e){
 if(state!=='idle')return;
 const grab=cornerAt(e);
 if(!grab)return;
 e.preventDefault();e.stopPropagation();
 const epoch=captureEpoch;
 pending={id:e.pointerId,pressed:true,x:e.clientX,y:e.clientY};
 const input={id:e.pointerId,startX:e.clientX,startY:e.clientY,
   rect:grab.rect,grab};
 try{main.setPointerCapture(e.pointerId);}catch(_){}
 state='preparing';
 captureAndFold(input,epoch);
}
function onGlobalDown(e){
 if(state==='folding'&&overlay&&!overlay.contains(e.target)){
   restore();
 }
}
main.addEventListener('pointerdown',onMainDown,true);
main.addEventListener('pointermove',onMainMove,true);
main.addEventListener('pointerup',onMainUp,true);
main.addEventListener('pointercancel',onMainUp,true);
document.addEventListener('pointerdown',onGlobalDown,true);
document.addEventListener('keydown',e=>{
 if(e.key==='Escape'&&state!=='idle')restore();
});
window.addEventListener('wt:profile-render',invalidate);
window.addEventListener('scroll',()=>{
 if(state==='folding'&&!drag)restore();
},{passive:true});
window.addEventListener('resize',invalidate,{passive:true});
fine.addEventListener('change',invalidate);
reduced.addEventListener('change',invalidate);
small.addEventListener('change',invalidate);
schedulePrewarm(250);
window.__WT_PAPER3D__={
 getDiagnostics:()=>({
  ready:true,mode:'single DOM report · edge Easter egg',
  active:state!=='idle',state,
  snapshotReady:!!cachedFrame&&cachedRevision===captureEpoch,
  snapshotWarming:!!snapshotInFlight,
  extraReportSections:document.querySelectorAll('#wt-physical-paper').length,
  inkAsset:'newsprint-ink.png',
  error:lastError,
  ...(model?.getDiagnostics()||{})
 })
};
