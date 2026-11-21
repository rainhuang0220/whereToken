import * as THREE from 'three';
import { createPortraitPrint } from './print/portrait-sheet.js';
import { projectSnapshot } from './print/profile-data.js';
import { buildMaterialCanvases, loadLocalImage } from './paper/browser-maps.js';
import { PaperFoldSheet } from './scene/paper-foldsheet.js';

const host=document.getElementById('wt-physical-paper');
const board=document.getElementById('wt-paper-stage');
const canvas=document.getElementById('wt-paper-scene');
const flat=document.getElementById('wt-paper-flat-image');
const status=document.getElementById('wt-paper-status');
const hint=document.getElementById('wt-paper-hint');
const action=name=>host.querySelector('[data-paper-'+name+']');
const clamp=(x,low,high)=>Math.max(low,Math.min(high,x));
const W=7.8,H=11.7;
let renderer,scene,camera,front,back,frontMaterial,backMaterial,model,spot,mainLight,fillLight;
let paperScan=null,lastKey='',lastPrintValue='',frame=0,lastTime=0,awake=0,active=false;
let printTexture,normalTexture,roughTexture,backPrintTexture;
let pointer=null,orbit=null,zoom=1,yaw=.035,pitch=-.07,grazing=false,flatMode=false;
const fine=window.matchMedia('(pointer:fine)');
const reduced=window.matchMedia('(prefers-reduced-motion:reduce)');
const small=window.matchMedia('(max-width:700px)');

function selected(){
  const info=window.__WT_NEWSprint;
  return info&&info.snapshot&&info.palette==='newsprint'&&fine.matches&&!reduced.matches&&!small.matches;
}
function texture(image,colorSpace=THREE.NoColorSpace){
 const t=new THREE.CanvasTexture(image);
 t.colorSpace=colorSpace;t.wrapS=THREE.ClampToEdgeWrapping;t.wrapT=THREE.ClampToEdgeWrapping;
 t.minFilter=THREE.LinearMipmapLinearFilter;t.magFilter=THREE.LinearFilter;
 t.anisotropy=Math.min(8,renderer.capabilities.getMaxAnisotropy());
 return t;
}
function disposeTexture(item){if(item)item.dispose();}
function wake(){
 awake=performance.now();
 if(active&&!frame)frame=requestAnimationFrame(tick);
}
function cameraFit(){
 const aspect=board.clientWidth/Math.max(1,board.clientHeight);
 const view=THREE.MathUtils.degToRad(camera.fov);
 const radius=Math.max(H/(2*Math.tan(view/2)),W/(2*Math.tan(view/2)*aspect))*1.23/zoom;
 camera.position.set(Math.sin(yaw)*radius,Math.sin(pitch)*radius,Math.cos(yaw)*Math.cos(pitch)*radius);
 camera.lookAt(0,0,.02);camera.updateProjectionMatrix();
}
function resize(){
 if(!renderer||!board.clientWidth||!board.clientHeight)return;
 renderer.setPixelRatio(Math.min(window.devicePixelRatio||1,1.75));
 renderer.setSize(board.clientWidth,board.clientHeight,false);
 camera.aspect=board.clientWidth/board.clientHeight;
 cameraFit();wake();
}
function rayOf(event){
 const b=canvas.getBoundingClientRect();
 const n=new THREE.Vector2(2*(event.clientX-b.left)/b.width-1,
     1-2*(event.clientY-b.top)/b.height);
 const r=new THREE.Raycaster();r.setFromCamera(n,camera);return r;
}
function onFlap(p){
 return model.q({x:(p.uv.x-.5)*W,y:(p.uv.y-.5)*H})>model.bandWidth*.30;
}
function hit(event,ray){
 const intersections=ray.intersectObjects([front,back],false);
 if(model.phase==='latched'){
  const flap=intersections.find(p=>p.uv&&onFlap(p));
  if(flap)return flap;
  const rect=canvas.getBoundingClientRect();
  let closest=null;
  for(const [u,v] of [[0,0],[0,1],[1,0],[1,1]]){
   if(model.q({x:(u-.5)*W,y:(v-.5)*H})<model.bandWidth*.30)continue;
   const p=model.mapVertex((u-.5)*W,(v-.5)*H);
   const ndc=new THREE.Vector3(p.x,p.y,p.z).project(camera);
   const x=rect.left+(ndc.x+1)*rect.width/2,y=rect.top+(1-ndc.y)*rect.height/2;
   const distance=Math.hypot(x-event.clientX,y-event.clientY);
   if(distance<24&&(!closest||distance<closest.distance))
    closest={uv:new THREE.Vector2(u,v),point:new THREE.Vector3(p.x,p.y,p.z),distance};
  }
  if(closest)return closest;
 }
 return intersections[0]||null;
}
function paintStatus(){
 const diag=model?.getDiagnostics();
 if(!diag)return;
 status.textContent=diag.phase==='latched'?'FOLDED · '+diag.angleDegrees.toFixed(0)+'°':
   diag.phase==='resting'?'PAPER · READY':diag.phase.toUpperCase()+' · '+diag.angleDegrees.toFixed(0)+'°';
}
function tick(now){
 frame=0;if(!active||document.hidden||flatMode)return;
 const dt=lastTime?clamp((now-lastTime)/1000,1/240,.16):1/60;
 lastTime=now;
 model.step(dt);
 if(front.geometry!==model.renderGeometry){
  front.geometry=model.renderGeometry;back.geometry=model.renderGeometry;
 }
 renderer.shadowMap.needsUpdate=true;
 renderer.render(scene,camera);
 paintStatus();
 if(pointer||orbit||model.phase==='settling'||now-awake<1800)frame=requestAnimationFrame(tick);
 else lastTime=0;
}
function repaintPaper(){
 if(!active||!window.__WT_NEWSprint)return;
 const info=window.__WT_NEWSprint;
 const selection=info.selection||{};
 const key=info.snapshot.snapshot_id+'|'+[selection.metric,selection.range,selection.tab,selection.filter,info.lang].join('|');
 if(key===lastKey)return;
 lastKey=key;
 try{
  const projection=projectSnapshot(info.snapshot,selection);
  const original=createPortraitPrint(info.snapshot,info.lang==='zh-CN'?'zh':'en',selection);
  // Preserve the 1200x1800 typography; use a smaller print map for the
  // optical compositor to keep redraws bounded on integrated GPUs.
  const print=document.createElement('canvas');
  print.width=900;print.height=1350;
  print.getContext('2d',{alpha:false}).drawImage(original,0,0,900,1350);
  const maps=buildMaterialCanvases(print,paperScan,{
   fiber:.63,roughness:.99,absorption:1.36,pinholes:.85,
   spread:.45,mottle:.82,opaquePrint:true,paperColor:[.94,.935,.905]
  });
  disposeTexture(printTexture);disposeTexture(normalTexture);disposeTexture(roughTexture);
  printTexture=texture(maps.albedo,THREE.SRGBColorSpace);
  normalTexture=texture(maps.normal);
  roughTexture=texture(maps.roughness);
  frontMaterial.map=printTexture;frontMaterial.normalMap=normalTexture;
  frontMaterial.roughnessMap=roughTexture;frontMaterial.needsUpdate=true;
  flat.src=maps.albedo.toDataURL('image/png');
  flat.alt='Printed whereToken public activity snapshot, '+info.snapshot.as_of_date;
  lastPrintValue=projection.total.display;
  model.reset({clearMemories:true});wake();
  window.dispatchEvent(new Event('wt:paper3d-ready'));
 }catch(e){
  console.warn('Newsprint print map failed, keeping existing readable HTML',e);
  lastKey='';host.hidden=true;
 }
}
function init3D(){
 renderer=new THREE.WebGLRenderer({canvas,antialias:true,alpha:false,powerPreference:'default'});
 renderer.outputColorSpace=THREE.SRGBColorSpace;renderer.toneMapping=THREE.ACESFilmicToneMapping;
 renderer.toneMappingExposure=1.12;
 renderer.shadowMap.enabled=true;renderer.shadowMap.type=THREE.PCFShadowMap;
 renderer.shadowMap.autoUpdate=false;
 scene=new THREE.Scene();scene.background=new THREE.Color('#cbc9c2');
 camera=new THREE.PerspectiveCamera(33,1,.1,110);
 mainLight=new THREE.DirectionalLight('#fff6e7',2.15);
 mainLight.position.set(-7,9,9);mainLight.castShadow=true;
 mainLight.shadow.mapSize.set(2048,2048);
 Object.assign(mainLight.shadow.camera,{left:-11,right:11,top:15,bottom:-15,near:.1,far:50});
 mainLight.shadow.bias=-.00024;mainLight.shadow.normalBias=.025;mainLight.shadow.radius=3.5;
 scene.add(mainLight,mainLight.target);
 fillLight=new THREE.HemisphereLight('#faf8ef','#868276',.75);scene.add(fillLight);
 const bounce=new THREE.DirectionalLight('#f6f0e2',.95);
 bounce.position.set(8,-2,11);scene.add(bounce);
 spot=new THREE.PointLight('#fff4e4',8.5,7.8,2);spot.position.set(0,1.8,3.8);scene.add(spot);
 const table=new THREE.Mesh(new THREE.PlaneGeometry(100,100),
  new THREE.MeshStandardMaterial({color:'#c4c2bc',roughness:1,metalness:0}));
 table.position.z=-.09;table.receiveShadow=true;scene.add(table);
 model=new PaperFoldSheet({width:W,height:H});
 frontMaterial=new THREE.MeshStandardMaterial({
  color:'#fffef9',roughness:.98,metalness:0,side:THREE.FrontSide,
  normalScale:new THREE.Vector2(.29,.29)
 });
 backMaterial=new THREE.MeshStandardMaterial({
  color:'#eeeadf',roughness:1,metalness:0,side:THREE.BackSide
 });
 front=new THREE.Mesh(model.renderGeometry,frontMaterial);
 front.castShadow=true;front.receiveShadow=false;scene.add(front);
 back=new THREE.Mesh(model.renderGeometry,backMaterial);
 back.receiveShadow=false;scene.add(back);
 const blank=document.createElement('canvas');blank.width=300;blank.height=450;
 const blankCtx=blank.getContext('2d');blankCtx.fillStyle='#fff';
 blankCtx.fillRect(0,0,blank.width,blank.height);
 const reverse=buildMaterialCanvases(blank,paperScan,{
  fiber:.69,roughness:.99,absorption:1.36,pinholes:0,
  spread:0,mottle:.64,opaquePrint:true,paperColor:[.945,.933,.904]
 });
 // The reverse side is unprinted; avoid mirrored text.
 backPrintTexture=texture(reverse.albedo,THREE.SRGBColorSpace);
 backMaterial.map=backPrintTexture;backMaterial.needsUpdate=true;
 canvas.addEventListener('contextmenu',e=>e.preventDefault());
 canvas.addEventListener('pointerdown',e=>{
  if(e.button===2){
   orbit={x:e.clientX,y:e.clientY,yaw,pitch};
   canvas.setPointerCapture(e.pointerId);wake();return;
  }
  if(e.button!==0)return;
  const ray=rayOf(e),contact=hit(e,ray);
  if(!contact||!contact.uv||!model.grabUV(contact.uv.x,contact.uv.y,e.timeStamp))return;
  const plane=new THREE.Plane(new THREE.Vector3(0,0,1),0);
  const start=ray.ray.intersectPlane(plane,new THREE.Vector3());
  if(!start)return;
  pointer={id:e.pointerId,start,plane};
  canvas.setPointerCapture(e.pointerId);canvas.style.cursor='grabbing';wake();
 });
 canvas.addEventListener('pointermove',e=>{
  if(orbit){
   yaw=clamp(orbit.yaw+(e.clientX-orbit.x)*.004,-.7,.7);
   pitch=clamp(orbit.pitch+(e.clientY-orbit.y)*.004,-.5,.48);cameraFit();wake();return;
  }
  const ray=rayOf(e);
  if(pointer){
   const p=ray.ray.intersectPlane(pointer.plane,new THREE.Vector3());
   if(p)model.dragDelta(p.x-pointer.start.x,p.y-pointer.start.y,e.timeStamp);
   wake();return;
  }
  const hovered=hit(e,ray);
  const uv=hovered?.uv;
  const edge=uv&&Math.min(uv.x,1-uv.x,uv.y,1-uv.y)<.115;
  canvas.style.cursor=edge||(model.phase==='latched'&&hovered&&onFlap(hovered))?'grab':'default';
  if(hovered){
   spot.position.lerp(new THREE.Vector3(hovered.point.x,hovered.point.y,3.8),.3);wake();
  }
 });
 const end=e=>{
  if(pointer&&pointer.id===e.pointerId)model.release(e.timeStamp);
  pointer=null;orbit=null;canvas.style.cursor='default';wake();
 };
 canvas.addEventListener('pointerup',end);
 canvas.addEventListener('pointercancel',end);
 canvas.addEventListener('lostpointercapture',end);
 // On a vertically scrolling profile, unmodified wheel always scrolls the
 // document. Require Alt/Ctrl/Meta for the 3D paper zoom, unlike the lab demo.
 canvas.addEventListener('wheel',e=>{
  if(!(e.altKey||e.ctrlKey||e.metaKey))return;
  e.preventDefault();zoom=clamp(zoom*(e.deltaY>0?.92:1.09),.6,3.5);
  cameraFit();wake();
 },{passive:false});
 const reset=action('reset'),grazingButton=action('light'),flatButton=action('flat');
 reset?.addEventListener('click',()=>{model.reset({clearMemories:true});wake()});
 grazingButton?.addEventListener('click',()=>{
  grazing=!grazing;grazingButton.setAttribute('aria-pressed',String(grazing));
  mainLight.position.set(grazing?-8:-7,grazing?5:9,grazing?3.5:9);
  mainLight.intensity=grazing?1.85:2.15;fillLight.intensity=grazing?1.05:.75;
  spot.intensity=grazing?4.4:8.5;wake();
 });
 flatButton?.addEventListener('click',()=>{
  flatMode=!flatMode;
  flat.hidden=!flatMode;canvas.hidden=flatMode;
  flatButton.setAttribute('aria-pressed',String(flatMode));
  if(!flatMode){resize();wake();}
 });
 new ResizeObserver(resize).observe(board);
 document.addEventListener('visibilitychange',()=>{if(!document.hidden)wake()});
}
let launching=false,queued=0;
async function activate(){
 if(!selected()){active=false;host.hidden=true;return;}
 host.hidden=false;
 if(!renderer){
  if(launching)return;
  launching=true;
  try{
   paperScan=await loadLocalImage('./assets/newsprint-height.jpg');
   if(!selected()){host.hidden=true;return;}
   init3D();
  }catch(e){
   console.warn('Interactive paper unavailable; leaving complete HTML dashboard',e);
   host.hidden=true;return;
  }finally{launching=false;}
 }
 active=true;resize();
 if(queued)clearTimeout(queued);
 // A wall range/tab selection can rerender five times in quick succession.
 // Coalesce print-map recomposition instead of blocking each DOM interaction.
 queued=setTimeout(()=>{queued=0;repaintPaper();},350);
 wake();
}
function sync(){
 if(!selected()){active=false;host.hidden=true;return;}
 activate();
}
window.addEventListener('wt:profile-render',sync);
fine.addEventListener('change',sync);
reduced.addEventListener('change',sync);
small.addEventListener('change',sync);
if(window.__WT_NEWSprint)sync();
window.__WT_PAPER3D__={
  getDiagnostics:()=>({active,ready:!!renderer,phase:model?.phase||'none',
   palette:window.__WT_NEWSprint?.palette||'',asOf:window.__WT_NEWSprint?.snapshot?.as_of_date||'',
   printKey:lastKey,printedTotal:lastPrintValue,
   ...(model?.getDiagnostics()||{})}),
  refresh:sync
};
