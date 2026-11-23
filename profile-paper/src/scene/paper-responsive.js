import * as THREE from 'three';
import { PaperMemory } from './paper-memory.js';

const PI = Math.PI;
const clamp = (value, low, high) => Math.max(low, Math.min(high, value));
const smooth = value => { const t = clamp(value, 0, 1); return t*t*(3-2*t); };

// D3: responsive local sheet bend, not an elastic cloth lattice.
// The sheet remains planar outside a finite-width bending band.
// Edge bend regions taper in their tangent direction instead of moving the full edge.
export class PaperResponsive extends PaperMemory {
  constructor(options = {}) {
    super(options);
    this.maxVertices = 7000;
    this.maxIndices = 25000;
    this.positions = new Float32Array(this.maxVertices*3);
    this.uvs = new Float32Array(this.maxVertices*2);
    this.normals = new Float32Array(this.maxVertices*3);
    this.indices = new Uint16Array(this.maxIndices);
    this.foldedGeometry.dispose();
    this.foldedGeometry = new THREE.BufferGeometry();
    this.foldedGeometry.setAttribute('position', new THREE.BufferAttribute(this.positions,3).setUsage(THREE.DynamicDrawUsage));
    this.foldedGeometry.setAttribute('uv', new THREE.BufferAttribute(this.uvs,2).setUsage(THREE.DynamicDrawUsage));
    this.foldedGeometry.setAttribute('normal', new THREE.BufferAttribute(this.normals,3).setUsage(THREE.DynamicDrawUsage));
    this.foldedGeometry.setIndex(new THREE.BufferAttribute(this.indices,1).setUsage(THREE.DynamicDrawUsage));
    this.bandWidth = .26;
    this.lastReleaseAngle = 0;
    this.renderGeometry = this.restingGeometry;
  }

  computeHinge(grab, dx, dy) {
    if (grab.type === 'corner') {
      const ix = Math.max(0, -dx*grab.sx), iy = Math.max(0, -dy*grab.sy);
      const travel = Math.hypot(ix, iy);
      const wx = .56 + ix/(travel+.12), wy = .56 + iy/(travel+.12);
      const n = Math.hypot(wx, wy), nx = grab.sx*wx/n, ny = grab.sy*wy/n;
      const px = (grab.u-.5)*this.width, py = (grab.v-.5)*this.height;
      const inset = Math.hypot(px-grab.x, py-grab.y);
      const depth = clamp(.32+.36*travel+.28*inset, .32, 2.16);
      const start = {x:grab.x-grab.sx*depth/Math.abs(nx), y:grab.y};
      const end = {x:grab.x, y:grab.y-grab.sy*depth/Math.abs(ny)};
      const span = Math.hypot(end.x-start.x, end.y-start.y);
      return {nx,ny,depth,origin:{x:grab.x,y:grab.y},start,end,
        kind:'corner',length:span,span,alongShift:0};
    }
    const vertical = grab.type === 'vertical-edge';
    const sign = vertical ? grab.sx : grab.sy;
    const inward = Math.max(0, -(vertical?dx:dy)*sign);
    const sideways = vertical ? dy : dx;
    const tilt = clamp(sideways/(inward+.8)*.17,-.24,.24);
    const n = 1/Math.hypot(1,tilt);
    const nx = vertical ? sign*n : tilt*n;
    const ny = vertical ? tilt*n : sign*n;
    const depth = clamp(.22+.28*inward+.07*Math.abs(sideways),.22,1.32);
    const speed = this.speedFiltered || 0;
    const span = clamp(.58+.72*inward+.72*Math.abs(sideways)+.08*Math.min(6,speed),.58,4.6);
    const alongShift = clamp(sideways*.21,-.62,.62);
    const origin = {x:grab.x,y:grab.y};
    const cx = origin.x-nx*depth-ny*alongShift;
    const cy = origin.y-ny*depth+nx*alongShift;
    const start = {x:cx+ny*span/2,y:cy-nx*span/2};
    const end = {x:cx-ny*span/2,y:cy+nx*span/2};
    return {nx,ny,depth,origin,start,end,kind:'edge',length:span,span,alongShift};
  }

  tangentAt(point, hinge = this.hinge) {
    const cx = hinge.origin.x-hinge.nx*hinge.depth;
    const cy = hinge.origin.y-hinge.ny*hinge.depth;
    return -(point.x-cx)*hinge.ny+(point.y-cy)*hinge.nx-hinge.alongShift;
  }

  localWindow(x,y,hinge=this.hinge) {
    if (!hinge) return 0;
    if (hinge.kind==='corner') return 1;
    const half=hinge.span/2, fade=clamp(half*.33,.14,.50);
    const t=this.tangentAt({x,y},hinge);
    return smooth((t+half)/fade)*smooth((half-t)/fade);
  }

  dragDelta(dx,dy,now=performance.now()) {
    // PaperMemory updates timestamp and measured motion. Discard its older
    // fixed-gain target/peak so it cannot create fake plastic yield for a
    // gentle fold in this newer, lower-gain interaction model.
    const realPeak=this.peakTarget;
    if (!super.dragDelta(dx,dy,now)) return false;
    const g=this.fold;
    const inward=g.type==='corner' ? -.707*(g.sx*dx+g.sy*dy)
      : g.type==='vertical-edge' ? -g.sx*dx : -g.sy*dy;
    const tangential=g.type==='corner' ? Math.abs(-g.sy*dx+g.sx*dy)*.40
      : Math.abs(g.type==='vertical-edge'?dy:dx);
    const speed=Math.min(6,this.speedFiltered||0);
    const impulse=Math.min(.16,speed*.028)*smooth(Math.max(0,inward)/.7);
    const angle=1.13*inward+.13*tangential*smooth(Math.max(0,inward)/.6)+impulse;
    this.target=clamp(this.dragReferenceAngle+angle,0,this.maxFoldAngle??PI-.09);
    this.peakTarget=Math.max(realPeak,this.target);
    return true;
  }

  release() {
    if (this.phase!=='dragging') return;
    const peak=Math.max(this.theta,this.target),speed=this.speedFiltered||0;
    const reopening=this.dragReferenceAngle>1;
    const latched=reopening ? this.target>1.91 : peak>2.70||(peak>2.06&&speed>3.0);
    this.lastReleaseAngle=peak;
    if (this.peakAngle>.74||this.peakTarget>.74) this.recordMemory(latched);
    this.transition=latched?'latch':'recover';
    this.target=latched?clamp(peak+Math.min(.11,speed*.02),1.92,this.maxFoldAngle??PI-.09):0;
    this.velocity=0;this.phase='settling';this.elapsed=0;
  }

  step(dt=1/60) {
    if (this.phase==='resting'||this.phase==='latched') return;
    const seconds=clamp(dt,1/240,.16);
    const omega=this.phase==='dragging'?18.8:this.transition==='latch'?12:15;
    const error=this.theta-this.target,decay=Math.exp(-omega*seconds);
    const temp=(this.velocity+omega*error)*seconds;
    this.theta=clamp(this.target+(error+temp)*decay,0,this.maxFoldAngle??PI-.09);
    this.velocity=(this.velocity-omega*temp)*decay;
    this.peakAngle=Math.max(this.peakAngle,this.theta);
    this.drawFolded();
    if(this.phase==='settling'&&Math.abs(this.theta-this.target)<.003){
      if(this.transition==='latch'){
        this.theta=this.target;this.phase='latched';this.drawFolded();
      }else{
        this.fold=null;this.hinge=null;this.phase='resting';
        this.theta=0;this.target=0;this.bentArea=0;this.activeVertices=0;
        this.renderGeometry=this.restingGeometry;
      }
    }
  }

  cornerWindow(x,y,h) {
    const ex=h.end.x-h.start.x,ey=h.end.y-h.start.y,len=Math.max(.001,h.length);
    const t=((x-h.start.x)*ex+(y-h.start.y)*ey)/len;
    return smooth((t+.14)/.22)*smooth((len+.14-t)/.22);
  }

  // Negative Gaussian valley with subtle asymmetric raised shoulders.
  // The material is deformed: there is no painted dark crease line.
  residualHeight(x,y) {
    let result=0;
    for(const m of this.memories){
      const h=m.hinge,q=this.q({x,y},h);
      if(Math.abs(q)>.39)continue;
      const local=h.kind==='edge'?this.localWindow(x,y,h):this.cornerWindow(x,y,h);
      if(local<=0)continue;
      const sigma=.071+.019*m.strength;
      const depression=Math.exp(-((q/sigma)**2));
      const shoulder1=Math.exp(-(((q+1.7*sigma)/(.93*sigma))**2));
      const shoulder2=Math.exp(-(((q-2.0*sigma)/(.90*sigma))**2));
      const amount=.010+.020*m.strength;
      result+=local*amount*(-depression+.21*shoulder1+.19*shoulder2);
    }
    return clamp(result,-.038,.013);
  }

  refreshRestingGeometry() {
    const attr=this.restingGeometry?.attributes?.position;
    if(!attr)return;
    const points=attr.array;
    for(let i=0;i<points.length;i+=3)
      points[i+2]=.021+this.residualHeight(this.basePositions[i],this.basePositions[i+1]);
    attr.needsUpdate=true;
    this.restingGeometry.computeVertexNormals();
    this.restingGeometry.computeBoundingSphere();
    this.restingGeometry.computeBoundingBox();
  }

  bendShape(t) {
    const x=clamp(t,0,1);
    return x-Math.sin(2*PI*x)/(2*PI);
  }

  mapVertex(x,y) {
    const hinge=this.hinge,q=this.q({x,y},hinge);
    const half=this.bandWidth/2;
    const envelope=this.localWindow(x,y,hinge);
    if(q<=-half||envelope<1e-5||this.theta<1e-6)
      return {x,y,z:.021+this.residualHeight(x,y),normalAngle:0};
    const angle=this.theta*envelope;
    const distance=clamp(q+half,0,this.bandWidth);
    const parts=16,step=distance/parts;
    let arcX=0,arcZ=0;
    for(let i=0;i<parts;i++){
      const fraction=(i+.5)/parts*distance/this.bandWidth;
      const phi=angle*this.bendShape(fraction);
      arcX+=Math.cos(phi)*step;
      arcZ+=Math.sin(phi)*step;
    }
    if(q>half) {
      const tail=q-half;
      arcX+=tail*Math.cos(angle);
      arcZ+=tail*Math.sin(angle);
    }
    const shift=arcX-(q+half);
    return {x:x+shift*hinge.nx,y:y+shift*hinge.ny,z:.021+arcZ,
      normalAngle:angle*this.bendShape(distance/this.bandWidth)};
  }

  clipTangent(polygon,threshold,above) {
    const result=[];
    for(let i=0;i<polygon.length;i++){
      const a=polygon[i],b=polygon[(i+1)%polygon.length];
      const da=this.tangentAt(a)-threshold,db=this.tangentAt(b)-threshold;
      const insideA=above?da>=-1e-7:da<=1e-7;
      const insideB=above?db>=-1e-7:db<=1e-7;
      if(insideA!==insideB){
        const t=da/(da-db);
        result.push({x:a.x+(b.x-a.x)*t,y:a.y+(b.y-a.y)*t});
      }
      if(insideB)result.push(b);
    }
    const unique=[];
    for(const point of result)
      if(!unique.some(other=>Math.hypot(other.x-point.x,other.y-point.y)<1e-7))
        unique.push(point);
    return unique;
  }

  drawFolded() {
    if(!this.hinge||this.theta<1e-6) {
      this.renderGeometry=this.restingGeometry;
      return;
    }
    this.bandWidth=clamp(.34-.18*this.theta/PI,.15,.34);
    const h=this.hinge,half=this.bandWidth/2;
    const count=20;
    const qBreaks=[-Infinity,-half];
    for(let i=1;i<count;i++)qBreaks.push(-half+this.bandWidth*i/count);
    qBreaks.push(half,Infinity);
    let sBreaks=[-Infinity,Infinity];
    if(h.kind==='edge' && !h.fullSpan){
      const edgeHalf=h.span/2,fade=clamp(edgeHalf*.33,.14,.50);
      sBreaks=[-Infinity,-edgeHalf,-edgeHalf+fade*.20,
        -edgeHalf+fade*.45,-edgeHalf+fade*.72,-edgeHalf+fade,
        -edgeHalf+fade*1.48,0,
        edgeHalf-fade*1.48,edgeHalf-fade,
        edgeHalf-fade*.72,edgeHalf-fade*.45,
        edgeHalf-fade*.20,edgeHalf,Infinity];
      sBreaks=[...new Set(sBreaks)].sort((a,b)=>a-b);
    }
    const positions=this.positions,uv=this.uvs,normals=this.normals,indices=this.indices;
    let vertices=0,indexCount=0;
    const writePolygon=polygon=>{
      if(polygon.length<3)return;
      const first=vertices;
      for(const point of polygon){
        if(vertices>=this.maxVertices)throw new Error('D3 vertex allocation exceeded');
        const vertex=this.mapVertex(point.x,point.y);
        const p=vertices*3,t=vertices*2;
        positions[p]=vertex.x;positions[p+1]=vertex.y;positions[p+2]=vertex.z;
        uv[t]=(point.x+this.width/2)/this.width;
        uv[t+1]=(point.y+this.height/2)/this.height;
        const sinus=Math.sin(vertex.normalAngle);
        normals[p]=-h.nx*sinus;
        normals[p+1]=-h.ny*sinus;
        normals[p+2]=Math.cos(vertex.normalAngle);
        vertices++;
      }
      for(let j=1;j<polygon.length-1;j++){
        if(indexCount+3>this.maxIndices)throw new Error('D3 index allocation exceeded');
        indices[indexCount++]=first;
        indices[indexCount++]=first+j;
        indices[indexCount++]=first+j+1;
      }
    };
    for(let i=0;i<qBreaks.length-1;i++){
      let region=this.rectangle;
      const min=qBreaks[i],max=qBreaks[i+1];
      if(Number.isFinite(min))region=this.clip(region,min,true);
      if(Number.isFinite(max))region=this.clip(region,max,false);
      if(region.length<3)continue;
      for(let j=0;j<sBreaks.length-1;j++){
        let area=region;
        if(Number.isFinite(sBreaks[j]))
          area=this.clipTangent(area,sBreaks[j],true);
        if(Number.isFinite(sBreaks[j+1]))
          area=this.clipTangent(area,sBreaks[j+1],false);
        writePolygon(area);
      }
    }
    this.activeVertices=vertices;
    this.bentArea=h.kind==='corner'
      ? .5*(h.depth/Math.abs(h.nx))*(h.depth/Math.abs(h.ny))/(this.width*this.height)
      : .76*h.depth*h.span/(this.width*this.height);
    this.foldedGeometry.setDrawRange(0,indexCount);
    for(const key of ['position','uv','normal'])
      this.foldedGeometry.attributes[key].needsUpdate=true;
    this.foldedGeometry.index.needsUpdate=true;
    this.foldedGeometry.computeBoundingSphere();
    this.foldedGeometry.computeBoundingBox();
    this.renderGeometry=this.foldedGeometry;
  }

  getDiagnostics() {
    const d=super.getDiagnostics();
    return {...d,
      solver:'responsive localized paper bend (not cloth)',
      creaseLength:this.hinge?+this.hinge.length.toFixed(3):0,
      foldDepth:this.hinge?+this.hinge.depth.toFixed(3):0,
      releaseAngle:+((this.lastReleaseAngle||0)*180/PI).toFixed(1),
      creaseProfile:'negative valley with asymmetric shoulders'};
  }
}
