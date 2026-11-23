import * as THREE from 'three';
import html2canvas from 'html2canvas';
import { PaperFoldSheet } from './scene/paper-foldsheet.js';
import { buildMaterialCanvases, loadLocalImage } from './paper/browser-maps.js';

const root=document.documentElement;
const clamp=(n,a,b)=>Math.max(a,Math.min(b,n));
const fine=matchMedia('(pointer:fine)');
const reduced=matchMedia('(prefers-reduced-motion:reduce)');
const small=matchMedia('(max-width:700px)');
const PIXELS_PER_UNIT=100;
const MAX_PIXELS=12_000_000;
// Session-scoped: survives a tab reload, but not closing the tab. Never sent
// to the server, and explicitly discarded on a real palette change.
const CREASE_SESSION_KEY='wheretoken.newsprint.creases.v1';
let renderer=null,canvas=null,scene=null,camera=null,front=null,back=null,ground=null;
let model=null,material=null,backMaterial=null,blankTexture=null,printTexture=null;
let normalTexture=null,roughTexture=null,blankNormal=null,blankRough=null;
let originalNormalCanvas=null,inkSubstrateCanvas=null,lastPrintDark=null;
let sheetShapeRevision=0,materialShapeRevision=-1,fastInkUpdates=0,fullMaterialBuilds=0;
let lastInkUpdateMs=0;
let paperScan=null,layout=null,sun=null,resizeObserver=null,contextLost=false;
let active=false,folding=false,drag=null,frame=0,renderFrame=0,lastTime=0,animatedUntil=0;
let queued=0,revision=0,readyRevision=-1,printTask=null,lastError=null;
let lastCaptureMs=0,lastMaterialMs=0,frameCount=0,effectiveInkScale=0,textureSize=null;
let lightTarget={x:-12,y:8},launching=false,hoverControl=false,focusControl=false;
let keyboardNavigation=false;

function enabled(){
 return root.dataset.activityPalette==='newsprint'&&fine.matches&&
   !reduced.matches&&!small.matches;
}
function measure(){
 const doc=document.documentElement;
 return {
  width:Math.max(1,window.innerWidth),
  height:Math.max(1,doc.scrollHeight,document.body.scrollHeight,window.innerHeight),
  viewportWidth:Math.max(1,window.innerWidth),
  viewportHeight:Math.max(1,window.innerHeight),
  scrollY:window.scrollY
 };
}
function withWorld(rect){
 return {...rect,W:rect.width/PIXELS_PER_UNIT,H:rect.height/PIXELS_PER_UNIT};
}
function pageColor(){
 const probe=document.createElement('canvas');
 probe.width=probe.height=1;
 const context=probe.getContext('2d',{willReadFrequently:true});
 context.fillStyle=getComputedStyle(document.body).backgroundColor;
 context.fillRect(0,0,1,1);
 const data=context.getImageData(0,0,1,1).data;
 const rgb=[data[0],data[1],data[2]];
 return {rgb,css:`rgb(${rgb.join(',')})`,paperColor:rgb.map(value=>value/255),
  dark:(rgb[0]*.2126+rgb[1]*.7152+rgb[2]*.0722)<128};
}
function makeTexture(source,color=THREE.SRGBColorSpace){
 const texture=new THREE.CanvasTexture(source);
 texture.colorSpace=color;
 texture.minFilter=THREE.LinearMipmapLinearFilter;
 texture.magFilter=THREE.LinearFilter;
 texture.anisotropy=Math.min(8,renderer.capabilities.getMaxAnisotropy());
 return texture;
}
function releaseTexture(texture){texture?.dispose();}
function renderNow(){
 if(!active||!renderer||document.hidden||contextLost)return;
 renderer.render(scene,camera);
 frameCount++;
}
function invalidate(){
 if(!active||document.hidden||frame||renderFrame)return;
 renderFrame=requestAnimationFrame(()=>{renderFrame=0;renderNow();});
}
function wake(duration=500){
 animatedUntil=Math.max(animatedUntil,performance.now()+duration);
 if(active&&!frame&&!document.hidden)frame=requestAnimationFrame(tick);
}
function updateCamera(){
 if(!camera||!layout)return;
 camera.left=-layout.viewportWidth/(2*PIXELS_PER_UNIT);
 camera.right=layout.viewportWidth/(2*PIXELS_PER_UNIT);
 camera.top=layout.viewportHeight/(2*PIXELS_PER_UNIT);
 camera.bottom=-layout.viewportHeight/(2*PIXELS_PER_UNIT);
 const centerY=layout.H/2-(layout.scrollY+layout.viewportHeight/2)/PIXELS_PER_UNIT;
 camera.position.set(0,centerY,Math.max(30,layout.H));
 camera.lookAt(0,centerY,0);
 camera.updateProjectionMatrix();
 camera.updateMatrixWorld();
}


function forgetSessionCreases(){
 try{sessionStorage.removeItem(CREASE_SESSION_KEY);}catch{}
}
function rememberSessionCreases(){
 if(!model)return;
 try{
  if(!model.memories.length){forgetSessionCreases();return;}
  sessionStorage.setItem(CREASE_SESSION_KEY,JSON.stringify({
   width:model.width,height:model.height,memories:model.memories
  }));
 }catch{}
}
function restoreSessionCreases(next){
 try{
  const text=sessionStorage.getItem(CREASE_SESSION_KEY);
  if(!text||text.length>12000)return;
  const stored=JSON.parse(text);
  if(!Number.isFinite(stored.width)||!Number.isFinite(stored.height)||
    stored.width<=0||stored.height<=0||
    !Array.isArray(stored.memories)||stored.memories.length>9)return;
  const finitePoint=p=>p&&Number.isFinite(p.x)&&Number.isFinite(p.y);
  const memoryOK=m=>{
   const h=m?.hinge;
   return h&&['corner','edge'].includes(h.kind)&&
    finitePoint(h.start)&&finitePoint(h.end)&&finitePoint(h.origin)&&
    [h.nx,h.ny,h.depth,h.length,m.strength,m.band].every(Number.isFinite)&&
    m.strength>=0&&m.strength<=1&&h.length>0;
  };
  if(!stored.memories.every(memoryOK))return;
  transferCreases(stored,next);
 }catch{}
}
function transferCreases(previous,next){
 if(!previous?.memories?.length)return;
 const sx=next.width/previous.width,sy=next.height/previous.height;
 if(!Number.isFinite(sx)||!Number.isFinite(sy)||sx<=0||sy<=0)return;
 const point=p=>({x:p.x*sx,y:p.y*sy});
 next.memories=previous.memories.map(memory=>{
  const h=memory.hinge;
  const scale=Math.hypot(h.nx/sx,h.ny/sy);
  const start=point(h.start),end=point(h.end);
  return {
   ...memory,
   hinge:{...h,origin:point(h.origin),start,end,
    nx:(h.nx/sx)/scale,ny:(h.ny/sy)/scale,depth:h.depth/scale,
    length:Math.hypot(end.x-start.x,end.y-start.y),
    ...(h.span?{span:h.span*Math.sqrt(sx*sy)}:{})
   }
  };
 });
 next.refreshRestingGeometry();
}

function updateDocument(){
 if(!renderer)return;
 const next=withWorld(measure());
 const previous=layout;
 layout=next;
 const viewportChanged=!previous||previous.viewportWidth!==next.viewportWidth||
   previous.viewportHeight!==next.viewportHeight;
 if(viewportChanged){
  renderer.setPixelRatio(Math.min(devicePixelRatio||1,2));
  renderer.setSize(next.viewportWidth,next.viewportHeight,false);
 }
 const sheetChanged=!model||!previous||Math.abs(previous.width-next.width)>.5||
   Math.abs(previous.height-next.height)>.5;
 if(sheetChanged){
  if(drag){drag=null;finishFold();}
  const previousModel=model;
  model=new PaperFoldSheet({width:next.W,height:next.H});
  transferCreases(previousModel,model);
  if(previousModel?.memories.length)rememberSessionCreases();
  previousModel?.foldedGeometry.dispose();
  previousModel?.restingGeometry.dispose();
  sheetShapeRevision++;
  if(front)front.geometry=model.renderGeometry;
  if(back)back.geometry=model.renderGeometry;
  if(ground){
   ground.geometry.dispose();
   ground.geometry=new THREE.PlaneGeometry(next.W,next.H);
  }
  if(sun){
   const bound=Math.max(next.W,next.H)*1.25;
   Object.assign(sun.shadow.camera,{left:-bound,right:bound,top:bound,bottom:-bound});
   sun.shadow.camera.updateProjectionMatrix();
  }
  renderer.shadowMap.needsUpdate=true;
 }
 updateCamera();
 invalidate();
}
function tick(now){
 frame=0;
 if(!active||!renderer||document.hidden||contextLost)return;
 const dt=lastTime?clamp((now-lastTime)/1000,1/240,.10):1/60;
 lastTime=now;
 model.step(dt);
 const shadowMotion=model.phase==='dragging'||model.phase==='settling';
 if(shadowMotion)renderer.shadowMap.needsUpdate=true;
 if(front.geometry!==model.renderGeometry){
  front.geometry=model.renderGeometry;back.geometry=model.renderGeometry;
  renderer.shadowMap.needsUpdate=true;
 }
 const alpha=1-Math.exp(-dt/.28);
 const movingLight=Math.hypot(lightTarget.x-sun.position.x,lightTarget.y-sun.position.y)>.003;
 sun.position.x+=(lightTarget.x-sun.position.x)*alpha;
 sun.position.y+=(lightTarget.y-sun.position.y)*alpha;
 if(movingLight)renderer.shadowMap.needsUpdate=true;
 renderNow();
 if(!drag&&model.phase==='resting'&&folding)finishFold();
 if(drag||model.phase==='settling'||now<animatedUntil||movingLight){
  frame=requestAnimationFrame(tick);
 }else lastTime=0;
}
function captureScale(rect){
 const maxTexture=renderer.capabilities.maxTextureSize;
 return Math.min(devicePixelRatio||1,2,
  Math.sqrt(MAX_PIXELS/Math.max(1,rect.width*rect.height)),
  maxTexture/rect.width,maxTexture/rect.height);
}
async function captureNative(rect,color){
 const scale=captureScale(rect);
 const begin=performance.now();
 const image=await html2canvas(document.body,{
  backgroundColor:color.dark?color.css:'#ffffff',scale,logging:false,
  useCORS:false,allowTaint:false,removeContainer:true,
  // Exclude resident WebGL before html2canvas clones GPU-backed canvases.
  // Removing it from onclone is too late and stalls every interactive recapture.
  ignoreElements:node=>node.id==='wt-paper-resident'||node.id==='wt-paper',
  width:rect.width,height:rect.height,x:0,y:0,scrollX:0,scrollY:0,
  windowWidth:rect.width,windowHeight:window.innerHeight,
  onclone:doc=>{
   doc.documentElement.classList.remove('wt-physical-print','wt-paper-folding','wt-paper-ui');
   doc.documentElement.classList.add('wt-paper-capture');
   doc.documentElement.classList.toggle('wt-capture-dark',color.dark);
   doc.querySelectorAll('#wt-paper-resident,#wt-paper').forEach(node=>node.remove());
  }
 });
 lastCaptureMs=performance.now()-begin;
 return image;
}
function queueCapture(delay=35){
 if(queued)clearTimeout(queued);
 const current=++revision;
 queued=0;
 if(!active)return;
 if(drag){
  queued=setTimeout(()=>{queued=0;queueCapture(250);},250);
  return;
 }
 // Keep the old resident print and real clickable controls visible while new
 // content is captured. Removing wt-physical-print caused a full-page flash,
 // a ~2s UI stall, and temporarily broke the feeling of physical continuity.
 // Existing folded geometry and crease memories remain resident.
 queued=setTimeout(async()=>{
  queued=0;
  if(!active||current!==revision)return;
  const rect=measure();
  const color=pageColor();
  const task=captureNative(rect,color);
  printTask=task;
  try{
   const image=await task;
   if(!active||current!==revision)return;
   const materialBegin=performance.now();
   const canRefreshInk=!!inkSubstrateCanvas&&!!normalTexture&&!!roughTexture&&
    materialShapeRevision===sheetShapeRevision&&lastPrintDark===color.dark&&
    textureSize?.[0]===image.width&&textureSize?.[1]===image.height;
   let nextPrint;
   if(canRefreshInk){
    // A range, source, or filter click changes PRINTED INK, not cellulose.
    // Composite against the cached real paper substrate using a native 2D
    // blend. Normal, roughness, geometry and existing scars stay unchanged.
    if(color.dark){
     nextPrint=makeTexture(image);
    }else{
     const refreshed=document.createElement('canvas');
     refreshed.width=image.width;refreshed.height=image.height;
     const context=refreshed.getContext('2d');
     context.drawImage(inkSubstrateCanvas,0,0);
     context.globalCompositeOperation='multiply';
     context.drawImage(image,0,0);
     context.globalCompositeOperation='source-over';
     nextPrint=makeTexture(refreshed);
    }
    fastInkUpdates++;
   }else{
    const maps=buildMaterialCanvases(image,paperScan,{
     fiber:.60,roughness:.97,absorption:1.13,pinholes:0,pinholeScale:1.35,
     spread:.50,mottle:.25,opaquePrint:true,
     cssScale:image.width/Math.max(1,rect.width),
     paperColor:color.dark?[1,1,1]:color.paperColor
    });
    if(!active||current!==revision)return;
    // Capture remains authoritative for reverse white-on-dark printing.
    nextPrint=makeTexture(color.dark?image:maps.albedo);
    inkSubstrateCanvas=maps.substrate;
    originalNormalCanvas=document.createElement('canvas');
    originalNormalCanvas.width=maps.normal.width;originalNormalCanvas.height=maps.normal.height;
    originalNormalCanvas.getContext('2d').drawImage(maps.normal,0,0);
    const nextNormal=makeTexture(maps.normal,THREE.NoColorSpace);
    const nextRough=makeTexture(maps.roughness,THREE.NoColorSpace);
    releaseTexture(normalTexture);releaseTexture(roughTexture);
    normalTexture=nextNormal;roughTexture=nextRough;
    material.normalMap=normalTexture;material.roughnessMap=roughTexture;
    backMaterial.normalMap=normalTexture;backMaterial.roughnessMap=roughTexture;
    backMaterial.color.setRGB(...color.paperColor,THREE.SRGBColorSpace);
    material.needsUpdate=true;
    backMaterial.needsUpdate=true;
    if(model?.memories.length)applyCreaseImprint();
    materialShapeRevision=sheetShapeRevision;
    lastPrintDark=color.dark;
    fullMaterialBuilds++;
   }
   releaseTexture(printTexture);
   printTexture=nextPrint;
   material.map=printTexture;
   backMaterial.map=printTexture;
   lastMaterialMs=performance.now()-materialBegin;
   lastInkUpdateMs=lastMaterialMs;
   effectiveInkScale=image.width/Math.max(1,rect.width);
   textureSize=[image.width,image.height];
   readyRevision=current;
   root.classList.add('wt-physical-print');
   syncNativeOverlay();
   renderNow();
  }catch(error){
   lastError='Capture: '+String(error);
   // The prior verified print remains on the sheet if a transient recapture
   // fails. The native document stays fully interactive underneath.
   if(!printTexture)root.classList.remove('wt-physical-print','wt-paper-ui');
  }finally{
   if(printTask===task)printTask=null;
  }
 },delay);
}

// The post-fold scar belongs to the SAME fiber normal map as the printed page.
// Mesh normals alone have a 3px vertex pitch and form dotted highlights.
// This UV-normal imprint supplies subpixel continuous compressed shoulders
// without drawing a fake line or screen-space overlay.
function applyCreaseImprint(){
 if(!originalNormalCanvas||!normalTexture?.image||!layout||!model?.memories.length)return;
 const dst=normalTexture.image,w=dst.width,h=dst.height,W=layout.W,H=layout.H;
 const ctx=dst.getContext('2d',{willReadFrequently:true});
 ctx.drawImage(originalNormalCanvas,0,0);
 const pixels=ctx.getImageData(0,0,w,h);
 const pixelToX=x=>(x/(w-1)-.5)*W;
 const pixelToY=y=>(.5-y/(h-1))*H;
 for(const memory of model.memories){
  const hinge=memory.hinge;
  if(!hinge?.start||!hinge?.end)continue;
  const length=Math.max(.001,hinge.length);
  const dx=(hinge.end.x-hinge.start.x)/length,dy=(hinge.end.y-hinge.start.y)/length;
  const x0=Math.max(0,Math.floor((Math.min(hinge.start.x,hinge.end.x)-.16+W/2)/W*(w-1)));
  const x1=Math.min(w-1,Math.ceil((Math.max(hinge.start.x,hinge.end.x)+.16+W/2)/W*(w-1)));
  const y0=Math.max(0,Math.floor((H/2-Math.max(hinge.start.y,hinge.end.y)-.16)/H*(h-1)));
  const y1=Math.min(h-1,Math.ceil((H/2-Math.min(hinge.start.y,hinge.end.y)+.16)/H*(h-1)));
  const core=.024+.008*memory.strength,pressure=.014+.021*memory.strength;
  for(let y=y0;y<=y1;y++){
   const yy=pixelToY(y);
   for(let x=x0;x<=x1;x++){
    const xx=pixelToX(x);
    const along=(xx-hinge.start.x)*dx+(yy-hinge.start.y)*dy;
    if(along<0||along>length)continue;
    const q=(xx-hinge.origin.x)*hinge.nx+(yy-hinge.origin.y)*hinge.ny+hinge.depth;
    if(Math.abs(q)>core*2.7)continue;
    const fade=Math.min(1,along/.10,(length-along)/.10);
    const gradient=pressure*(2*q/core**2)*Math.exp(-((q/core)**2))*fade;
    const offset=(y*w+x)*4;
    pixels.data[offset]=clamp(pixels.data[offset]-gradient*hinge.nx*78,0,255);
    pixels.data[offset+1]=clamp(pixels.data[offset+1]-gradient*hinge.ny*78,0,255);
   }
  }
 }
 ctx.putImageData(pixels,0,0);normalTexture.needsUpdate=true;
 renderer.shadowMap.needsUpdate=true;
}

function finishFold(){
 folding=false;
 root.classList.remove('wt-paper-folding');
 if(model?.memories.length)model.refreshRestingGeometry();
 if(model?.memories.length)applyCreaseImprint();

 if(renderer)renderer.shadowMap.needsUpdate=true;
 syncNativeOverlay();
 invalidate();
}
function enterFold(){
 if((folding&&model?.phase!=='latched')||!model||readyRevision!==revision||!printTexture)return false;
 folding=true;
 root.classList.add('wt-paper-folding');
 root.classList.remove('wt-paper-ui');
 wake(900);
 return true;
}
function inspect(event){
 if(!active||!model||!layout)return null;
 const x=event.clientX,y=event.clientY;
 if(!Number.isFinite(x)||!Number.isFinite(y)||x<0||y<0||
   x>layout.viewportWidth||y>layout.viewportHeight)return null;
 const documentY=layout.scrollY+y;
 if(documentY<0||documentY>layout.height)return null;
 return {u:x/layout.width,v:1-documentY/layout.height,x,y,documentY};
}
function isControl(event){
 return !!event.target?.closest?.('button,a,input,textarea,select,summary,'+
  '[role=tab],[role=button],[contenteditable],dialog');
}
function pickFoldFlap(contact){
 if(!model.hinge||model.phase!=='latched')return null;
 const ndc=new THREE.Vector2(contact.x/layout.viewportWidth*2-1,
  1-contact.y/layout.viewportHeight*2);
 const ray=new THREE.Raycaster();ray.setFromCamera(ndc,camera);
 const hits=ray.intersectObjects([front,back],false);
 const hit=hits.find(point=>point.uv&&model.q({
  x:(point.uv.x-.5)*layout.W,y:(point.uv.y-.5)*layout.H
 })>model.bandWidth*.30);
 if(hit)return {u:hit.uv.x,v:hit.uv.y};
 for(const [u,v] of [[0,0],[0,1],[1,0],[1,1]]){
  const x=(u-.5)*layout.W,y=(v-.5)*layout.H;
  if(model.q({x,y})<model.bandWidth*.3)continue;
  const mapped=model.mapVertex(x,y);
  const screen=new THREE.Vector3(mapped.x,mapped.y,mapped.z).project(camera);
  const px=(screen.x+1)/2*layout.viewportWidth,py=(1-screen.y)/2*layout.viewportHeight;
  if(Math.hypot(px-contact.x,py-contact.y)<28)return {u,v};
 }
 return null;
}
function candidate(event){
 if(!active||!enabled()||event.button!==0||event.pointerType==='touch'||event.isPrimary===false)return null;
 if(readyRevision!==revision||!root.classList.contains('wt-physical-print'))return null;
 const contact=inspect(event);
 if(!contact)return null;
 if(isControl(event))return null;
 if(model.phase==='latched'){
  const folded=pickFoldFlap(contact);
  return folded?{...contact,...folded}:null;
 }
 const edgeX=Math.min(contact.x,layout.width-contact.x);
 const edgeY=Math.min(contact.documentY,layout.height-contact.documentY);
 // Both axes must approach a corner. A 120px-wide horizontal/vertical
 // strip stole far too many clicks from the actual dashboard.
 if(edgeX<Math.min(84,layout.width*.075)&&edgeY<84)return contact;
 return null;
}
function begin(event){
 if(drag||!active||!enabled())return;
 const contact=candidate(event);
 if(!contact)return;
 // Merely clicking a page corner is NOT a fold and must not steal the click.
 // Activate the physics solver only after intentional movement.
 drag={id:event.pointerId,x:event.clientX,y:event.clientY,
  downAt:event.timeStamp,moved:false,pending:true,contact};
}
function move(event){
 if(!drag||drag.id!==event.pointerId||!model)return;
 if((event.buttons&1)===0||event.timeStamp<=drag.downAt)return;
 const travel=Math.hypot(event.clientX-drag.x,event.clientY-drag.y);
 if(drag.pending){
  if(travel<12)return;
  if(event.timeStamp-drag.downAt<120&&
    travel>Math.min(450,layout.width*.42))return;
  if(!enterFold()||!model.grabUV(drag.contact.u,drag.contact.v,event.timeStamp)){
   drag=null;return;
  }
  drag.pending=false;
  document.getSelection()?.removeAllRanges();
 }
 const dx=(event.clientX-drag.x)/PIXELS_PER_UNIT*.85;
 const dy=-(event.clientY-drag.y)/PIXELS_PER_UNIT*.85;
 drag.moved=true;
 model.dragDelta(dx,dy,event.timeStamp);
 event.preventDefault();event.stopPropagation();
 wake(350);
}
function end(event){
 if(!drag||drag.id!==event.pointerId)return;
 if(drag.pending||!drag.moved){
  drag=null;
  return;
 }
 drag=null;
 model.release();
 rememberSessionCreases();
 wake(1200);
}
function flapPoints(){
 if(!model?.hinge||!layout)return [];
 const points=[];
 for(const [u,v] of [[0,0],[0,1],[1,0],[1,1]]){
  const x=(u-.5)*layout.W,y=(v-.5)*layout.H;
  if(model.q({x,y})<model.bandWidth*.3)continue;
  const mapped=model.mapVertex(x,y);
  const projected=new THREE.Vector3(mapped.x,mapped.y,mapped.z).project(camera);
  points.push({u,v,x:(projected.x+1)/2*layout.viewportWidth,
   y:(1-projected.y)/2*layout.viewportHeight});
 }
 return points;
}
function syncNativeOverlay(){
 const selection=document.getSelection();
 const selectionVisible=!!selection&&!selection.isCollapsed;
 // Opacity:0 DOM controls remain genuinely clickable and can own tooltips
 // without displaying a duplicate of the entire printed sheet. Showing all
 // DOM text on ordinary mouse hover creates massive double-ink ghosting.
 // Use native content only for keyboard accessibility and selected text.
 root.classList.toggle('wt-paper-ui',active&&!drag&&root.classList.contains('wt-physical-print')&&
  ((keyboardNavigation&&focusControl)||selectionVisible));
}
function init3D(scan){
 paperScan=scan;
 layout=withWorld(measure());
 canvas=document.createElement('canvas');
 canvas.id='wt-paper-resident';
 canvas.setAttribute('aria-hidden','true');
 canvas.className='wt-paper-resident';
 document.body.appendChild(canvas);
 renderer=new THREE.WebGLRenderer({canvas,alpha:true,antialias:true,powerPreference:'default'});
 renderer.outputColorSpace=THREE.SRGBColorSpace;
 renderer.toneMapping=THREE.NoToneMapping;
 renderer.setClearColor(0xffffff,0);
 renderer.shadowMap.enabled=true;
 renderer.shadowMap.type=THREE.PCFSoftShadowMap;
 renderer.shadowMap.autoUpdate=false;
 scene=new THREE.Scene();
 camera=new THREE.OrthographicCamera(-1,1,1,-1,.1,250);
 const ambient=new THREE.AmbientLight('#ffffff',1.35);
 scene.add(ambient);
 sun=new THREE.DirectionalLight('#fff8ee',2.75);
 sun.position.set(-12,8,7);
 sun.castShadow=true;
 sun.shadow.mapSize.set(2048,2048);
 sun.shadow.camera.near=.1;sun.shadow.camera.far=100;
 sun.shadow.bias=-.00012;sun.shadow.normalBias=.019;sun.shadow.radius=3;
 scene.add(sun);
 const bounce=new THREE.DirectionalLight('#faf8f0',.30);
 bounce.position.set(7,-3,10);scene.add(bounce);
 const blank=document.createElement('canvas');
 blank.width=blank.height=512;
 const context=blank.getContext('2d');
 context.fillStyle='#ffffff';context.fillRect(0,0,512,512);
 const paperColor=pageColor().paperColor;
 const blankMaps=buildMaterialCanvases(blank,scan,{
  fiber:.66,roughness:.99,absorption:1,pinholes:0,spread:0,mottle:.30,
  opaquePrint:true,paperColor:paperColor.map((value,index)=>value*[.965,.958,.945][index])
 });
 blankTexture=makeTexture(blankMaps.albedo);
 blankNormal=makeTexture(blankMaps.normal,THREE.NoColorSpace);
 blankRough=makeTexture(blankMaps.roughness,THREE.NoColorSpace);
 material=new THREE.MeshPhysicalMaterial({
  map:blankTexture,roughnessMap:blankRough,normalMap:blankNormal,
  normalScale:new THREE.Vector2(.11,.11),roughness:.97,metalness:0,
  specularIntensity:.30,side:THREE.FrontSide
 });
 backMaterial=new THREE.MeshPhysicalMaterial({
  map:blankTexture,roughnessMap:blankRough,normalMap:blankNormal,
  normalScale:new THREE.Vector2(.13,.13),roughness:.98,metalness:0,
  specularIntensity:.28,side:THREE.BackSide
 });
 backMaterial.onBeforeCompile=shader=>{
  shader.vertexShader=shader.vertexShader.replace('#include <begin_vertex>',
   '#include <begin_vertex>\ntransformed -= normal * 0.007;');
  // Thin newsprint transmits a faint, mirror-oriented impression of the front ink.
  // Apply this in linear space on the REAL reverse-facing mesh, not as a fake
  // screen-space white triangle or an extra overlay.
  shader.fragmentShader=shader.fragmentShader.replace('#include <map_fragment>',
   '#ifdef USE_MAP\n  vec4 backInk = texture2D(map, vMapUv);\n  diffuseColor.rgb *= mix(vec3(1.0), backInk.rgb, 0.11);\n#endif');
 };
 backMaterial.customProgramCacheKey=()=> 'paper-reverse-thickness-transmitted-ink-v1';
 model=new PaperFoldSheet({width:layout.W,height:layout.H});
 restoreSessionCreases(model);
 front=new THREE.Mesh(model.renderGeometry,material);
 back=new THREE.Mesh(model.renderGeometry,backMaterial);
 front.castShadow=true;back.castShadow=false;front.receiveShadow=true;back.receiveShadow=true;
 ground=new THREE.Mesh(new THREE.PlaneGeometry(layout.W,layout.H),
  new THREE.ShadowMaterial({opacity:.165}));
 ground.position.z=-.045;ground.receiveShadow=true;
 scene.add(ground,front,back);
 active=true;contextLost=false;
 // The document layout was already measured before creating the renderer.
 // Explicitly initialize the real drawing buffer: otherwise updateDocument()
 // sees an unchanged viewport and leaves WebGL at its 300x150 default,
 // which gets bilinearly enlarged to a blurry full-screen physical sheet.
 renderer.setPixelRatio(Math.min(devicePixelRatio||1,2));
 renderer.setSize(layout.viewportWidth,layout.viewportHeight,false);
 updateDocument();
 resizeObserver=new ResizeObserver(()=>{
  if(!active||drag)return;
  const next=measure();
  if(Math.abs(next.width-layout.width)>.5||Math.abs(next.height-layout.height)>.5){
   updateDocument();queueCapture(100);
  }
 });
 resizeObserver.observe(document.body);
 canvas.addEventListener('webglcontextlost',event=>{
  event.preventDefault();contextLost=true;lastError='WebGL context lost';dispose(true);
 });
 renderer.shadowMap.needsUpdate=true;
 renderNow();
 window.dispatchEvent(new Event('wt:paper3d-ready'));
 queueCapture(100);
}
function dispose(notify=false){
 if(queued)clearTimeout(queued);queued=0;
 if(frame)cancelAnimationFrame(frame);if(renderFrame)cancelAnimationFrame(renderFrame);
 frame=renderFrame=0;lastTime=0;active=false;drag=null;folding=false;
 revision++;readyRevision=-1;
 root.classList.remove('wt-physical-print','wt-paper-folding','wt-paper-ui');
 resizeObserver?.disconnect();resizeObserver=null;
 releaseTexture(printTexture);releaseTexture(blankTexture);releaseTexture(normalTexture);
 releaseTexture(roughTexture);releaseTexture(blankNormal);releaseTexture(blankRough);
 front?.material.dispose();back?.material.dispose();
 ground?.geometry.dispose();ground?.material.dispose();ground=null;
 model?.foldedGeometry.dispose();model?.restingGeometry.dispose();
 renderer?.dispose();if(renderer&&!contextLost)renderer.forceContextLoss();
 canvas?.remove();
 canvas=renderer=scene=camera=front=back=model=material=backMaterial=null;
 blankTexture=printTexture=normalTexture=roughTexture=blankNormal=blankRough=null;
 originalNormalCanvas=null;inkSubstrateCanvas=null;lastPrintDark=null;
 materialShapeRevision=-1;
 printTask=null;textureSize=null;effectiveInkScale=0;
 if(notify)window.dispatchEvent(new Event('wt:paper3d-failed'));
}
async function sync(){
 if(root.dataset.activityPalette&&root.dataset.activityPalette!=='newsprint')
  forgetSessionCreases();
 if(!enabled()){
  if(active||canvas)dispose();
  return;
 }
 if(!active){
  if(launching)return;
  launching=true;contextLost=false;lastError=null;
  try{
   const scan=await loadLocalImage('./assets/newsprint-height.jpg');
   if(enabled()&&!active)init3D(scan);
  }catch(error){
   lastError='WebGL unavailable: '+String(error);
   dispose(true);
  }finally{launching=false;}
 }else{
  updateDocument();
 }
}

document.addEventListener('pointerdown',begin,true);
document.addEventListener('pointermove',event=>{
 if(drag){move(event);return;}
 // Normal pointer hover never changes the visual ink authority.
 if(keyboardNavigation){keyboardNavigation=false;syncNativeOverlay();}
 hoverControl=active&&isControl(event);
 // Resting paper has a stationary physical light: ordinary pointer hover,
 // especially across buttons, must not trigger expensive shadow re-renders.
},true);
document.addEventListener('pointerup',end,true);
document.addEventListener('pointercancel',end,true);
document.addEventListener('pointerleave',()=>{hoverControl=false;syncNativeOverlay();});
document.addEventListener('focusin',event=>{focusControl=isControl(event);syncNativeOverlay();});
document.addEventListener('focusout',()=>{
 setTimeout(()=>{focusControl=!!document.activeElement?.closest?.('button,a,input,textarea,select,summary,[role=tab],[role=button],[contenteditable],dialog');syncNativeOverlay();},0);
});
document.addEventListener('selectionchange',syncNativeOverlay);
window.addEventListener('blur',()=>{
 if(drag&&model){drag=null;model.release();rememberSessionCreases();wake(1200);}
});
document.addEventListener('keydown',event=>{
 if(event.key==='Tab'){keyboardNavigation=true;syncNativeOverlay();}
 if(event.key==='Escape'&&folding&&!drag){
  model?.reset({clearMemories:false});finishFold();wake(250);
 }
});
window.addEventListener('scroll',()=>{
 if(!active||!layout)return;
 layout.scrollY=window.scrollY;updateCamera();invalidate();
},{passive:true});
window.addEventListener('resize',()=>{
 if(active){updateDocument();queueCapture(90);}
 sync();
},{passive:true});
window.addEventListener('wt:profile-render',()=>{
 if(!enabled()){sync();return;}
 if(active){updateDocument();queueCapture(35);}else sync();
});
document.addEventListener('toggle',event=>{
 if(active&&event.target?.tagName==='DETAILS'){updateDocument();queueCapture(45);}
},true);
fine.addEventListener('change',sync);
reduced.addEventListener('change',sync);
small.addEventListener('change',sync);
document.addEventListener('visibilitychange',()=>{
 if(document.hidden){
  if(frame)cancelAnimationFrame(frame);if(renderFrame)cancelAnimationFrame(renderFrame);
  frame=renderFrame=0;lastTime=0;
 }else if(active)invalidate();
});
window.__WT_PAPER3D__={
 getDiagnostics:()=>({
  ready:active,physicalMeshResident:!!model&&!!renderer,
  materialMaps:!!normalTexture&&!!roughTexture,
  printedInkResident:!!printTexture&&material?.map===printTexture,
  inkVisibleAtRest:root.classList.contains('wt-physical-print')&&!!printTexture&&model?.phase==='resting',
  presenting:printTexture?'UV-printed resident D4 mesh':'D4 paper awaiting optical print',
  active,folding,snapshotReady:readyRevision===revision&&readyRevision>=0,
  captureRevision:revision,readyRevision,captureMs:Math.round(lastCaptureMs),
  materialMs:Math.round(lastMaterialMs),lastInkUpdateMs:Math.round(lastInkUpdateMs),
  fastInkUpdates,fullMaterialBuilds,effectiveInkScale,textureSize,frameCount,
  geometryVertices:model?.restingGeometry?.attributes?.position?.count||0,
  sheetCssWidth:layout?.width||0,sheetCssHeight:layout?.height||0,
  viewportWidth:layout?.viewportWidth||0,viewportHeight:layout?.viewportHeight||0,
  cameraScrollY:layout?.scrollY||0,flapPoints:flapPoints(),contextLost,
  extraReportSections:document.querySelectorAll('#wt-physical-paper').length,
  error:lastError,...(model?.getDiagnostics()||{})
 }),
 refresh:sync
};
sync();
