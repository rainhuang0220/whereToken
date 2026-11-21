import * as THREE from 'three';
import html2canvas from 'html2canvas';
import { PaperFoldSheet } from './scene/paper-foldsheet.js';
import { buildMaterialCanvases, loadLocalImage } from './paper/browser-maps.js';

/**
 * The only Newspaper: one permanently resident D4 physical paper mesh,
 * sharing the EXACT footprint of the existing Profile DOM. A light-scattering
 * scanned sheet is drawn BEHIND native, printed text and interactive controls.
 * The same native DOM is optically captured onto the same mesh as UV ink
 * during folding; all real HTML returns after a gentle fold. The physics,
 * lighting, paper normals, roughness and residual crease geometry stay alive.
 */
const root=document.documentElement;
const main=document.getElementById('main');
const W=7.8;
const clamp=(n,a,b)=>Math.max(a,Math.min(b,n));
const fine=matchMedia('(pointer:fine)');
const reduced=matchMedia('(prefers-reduced-motion:reduce)');
const small=matchMedia('(max-width:700px)');
let renderer=null,canvas=null,scene=null,camera=null,front=null,back=null;
let model=null,material=null,backMaterial=null,blankTexture=null,printTexture=null;
let normalTexture=null,roughTexture=null,paperScan=null,layout=null,ground=null;
let active=false,folding=false,drag=null,frame=0,lastTime=0,animatedUntil=0;
let queued=0,revision=0,readyRevision=-1,printTask=null,lastError=null;
let cachedInk=null,lastCaptureMs=0,lightTarget={x:-4,y:6},sun=null;
const inkImage=new Image();inkImage.src='./assets/newsprint-ink.png';

function enabled(){
 return root.dataset.activityPalette==='newsprint'&&fine.matches&&
   !reduced.matches&&!small.matches;
}
function sheet(){
 const r=main.getBoundingClientRect();
 return {width:r.width,height:r.height,H:W*r.height/Math.max(1,r.width),
  x:r.left+window.scrollX,y:r.top+window.scrollY};
}
function makeTexture(source,color=THREE.SRGBColorSpace){
 const t=new THREE.CanvasTexture(source);
 t.colorSpace=color;t.minFilter=THREE.LinearMipmapLinearFilter;
 t.magFilter=THREE.LinearFilter;
 t.anisotropy=Math.min(6,renderer.capabilities.getMaxAnisotropy());
 return t;
}
function releaseTexture(texture){texture?.dispose();}
function setPaperTexture(texture){
 if(!material||!texture)return;
 material.map=texture;material.needsUpdate=true;
 renderNow();
}
function renderNow(){
 if(!active||!renderer||document.hidden)return;
 renderer.render(scene,camera);
}
function wake(duration=700){
 animatedUntil=Math.max(animatedUntil,performance.now()+duration);
 if(active&&!frame)frame=requestAnimationFrame(tick);
}
function tick(now){
 frame=0;
 if(!active||!renderer||document.hidden)return;
 const dt=lastTime?clamp((now-lastTime)/1000,1/240,.10):1/60;
 lastTime=now;
 model.step(dt);
 if(front.geometry!==model.renderGeometry){
  front.geometry=model.renderGeometry;back.geometry=model.renderGeometry;
 }
 const alpha=1-Math.exp(-dt/0.28);
 sun.position.x+=(lightTarget.x-sun.position.x)*alpha;
 sun.position.y+=(lightTarget.y-sun.position.y)*alpha;
 renderNow();
 if(!drag&&model.phase==='resting'&&folding)unfolded();
 if(drag||model.phase==='settling'||now<animatedUntil){
  frame=requestAnimationFrame(tick);
 }else lastTime=0;
}
function positionCanvas(){
 if(!canvas||!renderer)return;
 const r=sheet();
 const resized=!layout||Math.abs(r.width-layout.width)>.5||
   Math.abs(r.height-layout.height)>.5;
 layout=r;
 canvas.style.left=r.x+'px';canvas.style.top=r.y+'px';
 canvas.style.width=r.width+'px';canvas.style.height=r.height+'px';
 if(resized){
  renderer.setPixelRatio(Math.min(devicePixelRatio||1,1.7));
  renderer.setSize(Math.max(1,r.width),Math.max(1,r.height),false);
  camera.left=-W/2;camera.right=W/2;
  camera.top=r.H/2;camera.bottom=-r.H/2;
  camera.updateProjectionMatrix();
  if(model&&Math.abs(model.height-r.H)>.01){
   const memories=model.memories.slice();
   model.foldedGeometry.dispose();model.restingGeometry.dispose();
   model=new PaperFoldSheet({width:W,height:r.H});
   if(memories.length){model.memories=memories;model.refreshRestingGeometry();}
   front.geometry=model.renderGeometry;back.geometry=model.renderGeometry;
   if(ground){ground.geometry.dispose();ground.geometry=new THREE.PlaneGeometry(W,r.H);}
  }
 }
 renderNow();
}
function inkOnTexture(snapshot){
 if(!inkImage.complete||!inkImage.naturalWidth)return;
 try{
  if(!cachedInk){
   const tmp=document.createElement('canvas');
   tmp.width=inkImage.naturalWidth;tmp.height=inkImage.naturalHeight;
   const ctx=tmp.getContext('2d',{willReadFrequently:true});
   ctx.drawImage(inkImage,0,0);
   cachedInk={width:tmp.width,height:tmp.height,
    data:ctx.getImageData(0,0,tmp.width,tmp.height).data};
  }
  const ctx=snapshot.getContext('2d',{willReadFrequently:true});
  const source=ctx.getImageData(0,0,snapshot.width,snapshot.height);
  const pixels=source.data,mask=cachedInk.data;
  for(let y=0;y<snapshot.height;y++){
   const row=y*snapshot.width*4,mRow=(y%cachedInk.height)*cachedInk.width*4;
   for(let x=0;x<snapshot.width;x++){
    const p=row+x*4;
    const luma=pixels[p]*.299+pixels[p+1]*.587+pixels[p+2]*.114;
    if(luma>183)continue;
    const ink=mask[mRow+(x%cachedInk.width)*4+3];
    if(ink>248)continue;
    const d=(255-ink)/255*.88*Math.min(1,(205-luma)/145);
    if(d<=0)continue;
    pixels[p]=Math.round(pixels[p]+(246-pixels[p])*d);
    pixels[p+1]=Math.round(pixels[p+1]+(242-pixels[p+1])*d);
    pixels[p+2]=Math.round(pixels[p+2]+(233-pixels[p+2])*d);
   }
  }
  ctx.putImageData(source,0,0);
 }catch(error){lastError='Ink compositor: '+String(error);}
}
async function captureNative(){
 const r=sheet();
 const scale=Math.min(1.4,3000/Math.max(1,r.height),1850/Math.max(1,r.width));
 const begin=performance.now();
 const image=await html2canvas(main,{
  backgroundColor:'#f7f6f1',scale,logging:false,
  useCORS:false,allowTaint:false,removeContainer:true,
  onclone:doc=>{
   doc.documentElement.classList.remove('wt-physical-print','wt-paper-folding');
   doc.documentElement.classList.add('wt-fold-capture');
  }
 });
 inkOnTexture(image);
 lastCaptureMs=performance.now()-begin;
 return image;
}
function queueCapture(delay=500){
 if(queued)clearTimeout(queued);
 const current=++revision;
 readyRevision=-1;
 queued=0;
 if(!active)return;
 queued=setTimeout(async()=>{
  queued=0;
  if(!active||folding||current!==revision)return;
  const task=captureNative();
  printTask=task;
  try{
   const image=await task;
   if(!active||current!==revision)return;
   const next=makeTexture(image);
   releaseTexture(printTexture);
   printTexture=next;
   readyRevision=current;
  }catch(error){
   lastError='Capture: '+String(error);
  }finally{
   if(printTask===task)printTask=null;
  }
 },delay);
}
function enterFold(){
 if(folding||!model||!printTexture||readyRevision!==revision)return false;
 folding=true;root.classList.add('wt-paper-folding');
 // The complete visible report was printed into the material before the
 // presentation change. DOM remains in place and is never removed.
 setPaperTexture(printTexture);
 wake(1300);
 return true;
}
function unfolded(){
 if(!folding)return;
 folding=false;root.classList.remove('wt-paper-folding');
 setPaperTexture(blankTexture);
 // Preserve actual geometric plastic crease memory in the RESIDENT D4 mesh.
 if(model.memories.length)model.refreshRestingGeometry();
 wake(350);
}
function inspect(e){
 if(!active||!model||!canvas)return null;
 const rect=main.getBoundingClientRect();
 const u=(e.clientX-rect.left)/rect.width;
 const v=1-(e.clientY-rect.top)/rect.height;
 if(u<0||u>1||v<0||v>1)return null;
 return {u,v,rect};
}
function isControl(e){
 return !!e.target.closest('button,a,input,textarea,select,summary,'+
 '[role=tab],[role=button],[contenteditable],dialog');
}
function pickFoldFlap(contact){
 if(!model.hinge||model.phase!=='latched')return null;
 const ndc=new THREE.Vector2(contact.u*2-1,contact.v*2-1);
 const ray=new THREE.Raycaster();ray.setFromCamera(ndc,camera);
 const hits=ray.intersectObjects([front,back],false);
 const hit=hits.find(p=>p.uv&&model.q({
  x:(p.uv.x-.5)*W,y:(p.uv.y-.5)*layout.H
 })>model.bandWidth*.30);
 if(hit)return {u:hit.uv.x,v:hit.uv.y};
 // Near-fold-tip tolerance: avoid invisible pixel-sized hotspots.
 for(const [u,v] of [[0,0],[0,1],[1,0],[1,1]]){
  const x=(u-.5)*W,y=(v-.5)*layout.H;
  if(model.q({x,y})<model.bandWidth*.3)continue;
  const mapped=model.mapVertex(x,y);
  const screen=new THREE.Vector3(mapped.x,mapped.y,mapped.z).project(camera);
  const px=(screen.x+1)/2,py=(screen.y+1)/2;
  if(Math.hypot((px-contact.u)*layout.width,
     (py-contact.v)*layout.height)<24)return {u,v};
 }
 return null;
}
function candidate(e){
 if(!active||!enabled()||e.button!==0||e.pointerType==='touch')return null;
 const q=inspect(e);
 if(!q)return null;
 if(model.phase==='latched'){
  const folded=pickFoldFlap(q);
  if(folded)return {...q,...folded};
 }
 if(isControl(e))return null;
 const x=q.u*layout.width,y=(1-q.v)*layout.height;
 const edgeX=Math.min(x,layout.width-x);
 const edgeY=Math.min(y,layout.height-y);
 if(edgeX<25||(edgeY<28&&edgeX<76))return q;
 return null;
}
function begin(e){
 if(drag||!active||!enabled())return;
 const q=candidate(e);
 if(!q)return;
 if(!enterFold()&&model.phase!=='latched')return;
 if(!model.grabUV(q.u,q.v,e.timeStamp)){
  if(model.phase==='resting')unfolded();
  return;
 }
 document.getSelection()?.removeAllRanges();
 drag={id:e.pointerId,x:e.clientX,y:e.clientY,u:q.u,v:q.v,moved:false};
 e.preventDefault();e.stopPropagation();
 try{main.setPointerCapture(e.pointerId);}catch(_){}
 wake(1500);
}
function move(e){
 if(!drag||drag.id!==e.pointerId||!model)return;
 const dx=(e.clientX-drag.x)*W/layout.width*.85;
 const dy=-(e.clientY-drag.y)*layout.H/layout.height*.85;
 drag.moved=Math.max(Math.abs(e.clientX-drag.x),
                     Math.abs(e.clientY-drag.y))>10;
 model.dragDelta(dx,dy,e.timeStamp);
 wake(1500);
}
function end(e){
 if(!drag||drag.id!==e.pointerId)return;
 if(!drag.moved&&model.phase==='dragging'){
  const sx=drag.u>.5?-1:1,sy=drag.v>.5?-1:1;
  model.dragDelta(sx*.28,sy*.28,e.timeStamp);
 }
 drag=null;
 model.release();
 wake(1500);
}
function outside(e){
 if(!folding||drag)return;
 if(model.phase==='latched'&&!main.contains(e.target)){
  // Outside click is a graceful reset, not a persistent invisible overlay.
  model.reset({clearMemories:false});unfolded();
 }
}
function init3D(scan){
 paperScan=scan;
 layout=sheet();
 canvas=document.createElement('canvas');
 canvas.id='wt-paper-resident';
 canvas.setAttribute('aria-hidden','true');
 canvas.className='wt-paper-resident';
 document.body.appendChild(canvas);
 renderer=new THREE.WebGLRenderer({
  canvas,alpha:true,antialias:true,powerPreference:'default'
 });
 renderer.outputColorSpace=THREE.SRGBColorSpace;
 renderer.toneMapping=THREE.NoToneMapping;
 renderer.setClearColor(0xffffff,0);
 renderer.shadowMap.enabled=true;
 renderer.shadowMap.type=THREE.PCFSoftShadowMap;
 scene=new THREE.Scene();
 camera=new THREE.OrthographicCamera(-W/2,W/2,layout.H/2,-layout.H/2,.1,75);
 camera.position.set(0,0,26);camera.lookAt(0,0,0);
 const ambient=new THREE.AmbientLight('#ffffff',2.58);
 scene.add(ambient);
 sun=new THREE.DirectionalLight('#fffdf7',.30);
 sun.position.set(-4,6,12);
 sun.castShadow=true;
 sun.shadow.mapSize.set(1024,1024);
 const shadowBound=Math.max(W,layout.H)*1.20;
 Object.assign(sun.shadow.camera,{
  left:-shadowBound,right:shadowBound,top:shadowBound,
  bottom:-shadowBound,near:.1,far:65
 });
 sun.shadow.bias=-.00026;
 sun.shadow.normalBias=.019;
 sun.shadow.radius=2.5;
 scene.add(sun);
 const bounce=new THREE.DirectionalLight('#faf8f0',.11);
 bounce.position.set(7,-3,10);scene.add(bounce);
 model=new PaperFoldSheet({width:W,height:layout.H});
 const blank=document.createElement('canvas');
 blank.width=768;blank.height=Math.max(384,Math.round(768*layout.H/W));
 const c=blank.getContext('2d');
 c.fillStyle='#ffffff';c.fillRect(0,0,blank.width,blank.height);
 const maps=buildMaterialCanvases(blank,scan,{
  fiber:.54,roughness:.99,absorption:1.36,pinholes:0,
  spread:.40,mottle:.42,opaquePrint:true,paperColor:[.952,.948,.932]
 });
 blankTexture=makeTexture(maps.albedo);
 normalTexture=makeTexture(maps.normal,THREE.NoColorSpace);
 roughTexture=makeTexture(maps.roughness,THREE.NoColorSpace);
 material=new THREE.MeshStandardMaterial({
  map:blankTexture,roughnessMap:roughTexture,
  normalMap:normalTexture,normalScale:new THREE.Vector2(.21,.21),
  roughness:.96,metalness:0,side:THREE.FrontSide
 });
 backMaterial=new THREE.MeshStandardMaterial({
  map:blankTexture,roughnessMap:roughTexture,
  normalMap:normalTexture,normalScale:new THREE.Vector2(.26,.26),
  roughness:1,metalness:0,side:THREE.BackSide
 });
 front=new THREE.Mesh(model.renderGeometry,material);
 back=new THREE.Mesh(model.renderGeometry,backMaterial);
 front.castShadow=true;back.castShadow=true;
 // Transparent shadow receiver is coextensive with the SAME paper footprint.
 // It creates real contact gradients under a fold, never a separate desk/card.
 ground=new THREE.Mesh(new THREE.PlaneGeometry(W,layout.H),
   new THREE.ShadowMaterial({opacity:.14}));
 ground.position.z=-.045;
 ground.receiveShadow=true;
 scene.add(ground,front,back);
 active=true;
 layout=null;
 positionCanvas();
 root.classList.add('wt-physical-print');
 renderNow();
 // WebGL mesh is alive before anyone has touched an edge.
 window.dispatchEvent(new Event('wt:paper3d-ready'));
 queueCapture(250);
 wake(450);
}
function dispose(){
 if(queued)clearTimeout(queued);queued=0;
 if(frame)cancelAnimationFrame(frame);frame=0;lastTime=0;
 root.classList.remove('wt-physical-print','wt-paper-folding');
 releaseTexture(printTexture);releaseTexture(blankTexture);
 releaseTexture(normalTexture);releaseTexture(roughTexture);
 front?.material.dispose();back?.material.dispose();
 ground?.geometry.dispose();ground?.material.dispose();ground=null;
 model?.foldedGeometry.dispose();model?.restingGeometry.dispose();
 renderer?.dispose();renderer?.forceContextLoss();canvas?.remove();
 canvas=renderer=scene=camera=front=back=model=material=backMaterial=null;
 blankTexture=printTexture=normalTexture=roughTexture=null;
 active=folding=false;drag=null;
 readyRevision=-1;revision++;
}
let launching=false;
async function sync(){
 if(!enabled()){
  if(active)dispose();
  return;
 }
 if(!active){
  if(launching)return;
  launching=true;
  try{
   const scan=await loadLocalImage('./assets/newsprint-height.jpg');
   if(enabled()&&!active)init3D(scan);
  }catch(error){
   lastError='WebGL unavailable: '+String(error);
   root.classList.remove('wt-physical-print','wt-paper-folding');
  }finally{launching=false;}
 }else if(!folding){
  positionCanvas();
  queueCapture(350);
 }
}
document.addEventListener('pointerdown',begin,true);
document.addEventListener('pointermove',e=>{
 if(drag){move(e);return;}
 if(!active||folding||!enabled())return;
 const q=inspect(e);if(!q)return;
 lightTarget.x=-4+(q.u-.5)*.85;
 lightTarget.y=6+(q.v-.5)*.80;
 wake(250);
},true);
document.addEventListener('pointerup',end,true);
document.addEventListener('pointercancel',end,true);
document.addEventListener('pointerdown',outside,false);
document.addEventListener('keydown',e=>{
 if(e.key==='Escape'&&folding){
  model?.reset({clearMemories:false});unfolded();
 }
});
window.addEventListener('scroll',()=>{
 if(active&&!drag)positionCanvas();
},{passive:true});
window.addEventListener('resize',()=>{
 if(active)positionCanvas();
 sync();
},{passive:true});
window.addEventListener('wt:profile-render',sync);
fine.addEventListener('change',sync);
reduced.addEventListener('change',sync);
small.addEventListener('change',sync);
document.addEventListener('visibilitychange',()=>{
 if(!document.hidden&&active)wake(400);
});
window.__WT_PAPER3D__={
 getDiagnostics:()=>({
  ready:active,physicalMeshResident:!!model&&!!renderer,
  materialMaps:!!normalTexture&&!!roughTexture,
  presenting:folding?'UV-printed folded D4 mesh':'resident D4 paper plus native pigment',
  active,folding,snapshotReady:readyRevision===revision,
  captureMs:Math.round(lastCaptureMs),
  geometryVertices:model?.restingGeometry?.attributes?.position?.count||0,
  extraReportSections:document.querySelectorAll('#wt-physical-paper').length,
  error:lastError,...(model?.getDiagnostics()||{})
 }),
 refresh:sync
};
sync();
