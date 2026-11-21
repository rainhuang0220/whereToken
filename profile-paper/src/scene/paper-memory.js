import * as THREE from 'three';

const PI = Math.PI;
const clamp = (x, a, b) => Math.max(a, Math.min(b, x));
const smooth = (x) => { const t = clamp(x, 0, 1); return t*t*(3-2*t); };
const near = (a,b,epsilon=1e-7) => Math.abs(a-b) < epsilon;

/**
 * D2 — hybrid developable paper, not a cloth solver.
 *
 * Geometry is clipped into a stationary planar body, an analytically integrated
 * finite-radius bend band, and a rigid flap. Along the bend normal the local
 * arc length is preserved: derivatives are (cos(theta), sin(theta)).
 * The rendered print keeps the original UVs, including through the bend.
 *
 * Motion is a critically damped one-degree-of-freedom hinge. A yield criterion
 * writes weak plastic crease curvature into the resting paper mesh. A separate
 * snap-through threshold can leave a fold latched until the flap is reopened.
 *
 * This is an interactive phenomenological paper model, not a calibrated FEM.
 */
export class PaperMemory {
  constructor({width=7.8,height=11.7}={}) {
    this.width=width;this.height=height;
    this.rectangle=[
      {x:-width/2,y:-height/2},{x:width/2,y:-height/2},
      {x:width/2,y:height/2},{x:-width/2,y:height/2}
    ];
    this.maxVertices=320;this.maxIndices=960;
    this.positions=new Float32Array(this.maxVertices*3);
    this.uvs=new Float32Array(this.maxVertices*2);
    this.normals=new Float32Array(this.maxVertices*3);
    this.indices=new Uint16Array(this.maxIndices);
    this.foldedGeometry=new THREE.BufferGeometry();
    this.foldedGeometry.setAttribute('position',
      new THREE.BufferAttribute(this.positions,3).setUsage(THREE.DynamicDrawUsage));
    this.foldedGeometry.setAttribute('uv',
      new THREE.BufferAttribute(this.uvs,2).setUsage(THREE.DynamicDrawUsage));
    this.foldedGeometry.setAttribute('normal',
      new THREE.BufferAttribute(this.normals,3).setUsage(THREE.DynamicDrawUsage));
    this.foldedGeometry.setIndex(new THREE.BufferAttribute(this.indices,1)
      .setUsage(THREE.DynamicDrawUsage));
    this.restingGeometry=new THREE.PlaneGeometry(width,height,156,234);
    this.basePositions=new Float32Array(this.restingGeometry.attributes.position.array);
    this.renderGeometry=this.restingGeometry;
    this.memories=[];
    this.creaseLimit=9;
    this.contactRadius=.56;
    this.bandWidth=.20;
    this.frame=0;
    this.reset({clearMemories:true});
  }

  reset({clearMemories=false}={}) {
    if(clearMemories)this.memories=[];
    this.fold=null;this.hinge=null;
    this.theta=0;this.target=0;this.velocity=0;
    this.phase='resting';
    this.dragDistance=0;this.speedPeak=0;this.speedFiltered=0;this.peakAngle=0;this.peakTarget=0;
    this.dragReferenceAngle=0;
    this.bandWidth=.20;
    this.dragStartAt=0;this.lastAt=0;this.lastDx=0;this.lastDy=0;
    this.elapsed=0;this.transition='recover';this.bentArea=0;this.activeVertices=0;
    this.refreshRestingGeometry();
    this.renderGeometry=this.restingGeometry;
  }

  // The same input coordinates are used by Raycaster UVs and original print ink.
  getCandidate(u,v) {
    const left=u,right=1-u,bottom=v,top=1-v;
    const nearX=Math.min(left,right),nearY=Math.min(top,bottom);
    if(nearX<.19&&nearY<.17){
      const sx=u>.5?1:-1,sy=v>.5?1:-1;
      return {type:'corner',sx,sy,x:sx*this.width/2,y:sy*this.height/2,u,v};
    }
    if(nearX<.115) {
      const sx=u>.5?1:-1;
      return {type:'vertical-edge',sx,sy:0,x:sx*this.width/2,y:(v-.5)*this.height,u,v};
    }
    if(nearY<.115){
      const sy=v>.5?1:-1;
      return {type:'horizontal-edge',sx:0,sy,x:(u-.5)*this.width,y:sy*this.height/2,u,v};
    }
    return null;
  }

  // Hinges are never predefined in the middle of the document.
  computeHinge(fold,dx,dy) {
    if(fold.type==='corner') {
      const amount=Math.hypot(Math.max(0,-dx*fold.sx),Math.max(0,-dy*fold.sy));
      const wx=.60+Math.max(0,-dx*fold.sx)/Math.max(amount,.0001);
      const wy=.60+Math.max(0,-dy*fold.sy)/Math.max(amount,.0001);
      const norm=Math.hypot(wx,wy);
      const nx=fold.sx*wx/norm,ny=fold.sy*wy/norm;
      const depth=clamp(.93+amount*.11,.93,1.24);
      const start={x:fold.x-fold.sx*depth/Math.abs(nx),y:fold.y};
      const end={x:fold.x,y:fold.y-fold.sy*depth/Math.abs(ny)};
      return {nx,ny,depth,origin:{x:fold.x,y:fold.y},start,end,
        kind:'corner',length:Math.hypot(end.x-start.x,end.y-start.y)};
    }
    if(fold.type==='vertical-edge') {
      const nx=fold.sx,ny=0;
      const depth=clamp(.45+Math.max(0,-dx*fold.sx)*.14,.45,.76);
      return {nx,ny,depth,origin:{x:fold.x,y:fold.y},
        start:{x:fold.x-fold.sx*depth,y:-this.height/2},
        end:{x:fold.x-fold.sx*depth,y:this.height/2},
        kind:'edge',length:this.height};
    }
    const nx=0,ny=fold.sy;
    const depth=clamp(.43+Math.max(0,-dy*fold.sy)*.15,.43,.75);
    return {nx,ny,depth,origin:{x:fold.x,y:fold.y},
      start:{x:-this.width/2,y:fold.y-fold.sy*depth},
      end:{x:this.width/2,y:fold.y-fold.sy*depth},
      kind:'edge',length:this.width};
  }

  q(point,hinge=this.hinge) {
    if(!hinge)return 0;
    return (point.x-hinge.origin.x)*hinge.nx+
      (point.y-hinge.origin.y)*hinge.ny+hinge.depth;
  }

  grabUV(u,v,now=performance.now()) {
    if(this.phase==='dragging')return false;
    if(this.phase==='settling') {
      if(this.transition==='latch'&&this.theta>1.35) {
        // A fold is graspable while it is finishing its last few degrees:
        // do not force the reader to await a frame-rate-dependent timeout.
        this.phase='latched';
      } else if(this.transition==='recover'&&this.theta<.22){
        this.phase='resting';this.fold=null;this.hinge=null;
        this.theta=0;this.target=0;this.bentArea=0;
        this.renderGeometry=this.restingGeometry;
      } else return false;
    }
    // A previously folded flap remains graspable in material coordinates,
    // even when its geometric location has moved under the folded paper.
    if(this.phase==='latched'&&this.fold&&this.hinge) {
      const x=(u-.5)*this.width,y=(v-.5)*this.height;
      if(this.q({x,y})>this.bandWidth*.30){
        this.phase='dragging';
        this.dragReferenceAngle=this.theta;
        this.dragStartAt=now;this.lastAt=now;
        this.lastDx=this.lastDy=0;
        this.dragDistance=0;this.speedPeak=0;this.speedFiltered=0;
        this.peakAngle=this.theta;this.peakTarget=this.theta;
        return true;
      }
    }
    const candidate=this.getCandidate(u,v);
    if(!candidate)return false;
    this.fold=candidate;
    this.hinge=this.computeHinge(candidate,0,0);
    this.theta=0;this.target=0;this.velocity=0;this.elapsed=0;
    this.dragReferenceAngle=0;this.phase='dragging';
    this.dragStartAt=now;this.lastAt=now;this.lastDx=this.lastDy=0;
    this.dragDistance=0;this.speedPeak=0;this.speedFiltered=0;
    this.peakAngle=0;this.peakTarget=0;
    this.drawFolded();
    return true;
  }

  dragDelta(dx,dy,now=performance.now()) {
    if(this.phase!=='dragging'||!this.fold)return false;
    const elapsedSec=Math.max(.012,(now-this.lastAt)/1000);
    const ddx=dx-this.lastDx,ddy=dy-this.lastDy;
    const speed=Math.hypot(ddx,ddy)/elapsedSec;
    if(Number.isFinite(speed)) {
      // Timestamp jitter should not turn a slow mouse motion into a flick.
      this.speedFiltered=this.speedFiltered==null ? speed*.38 :
        this.speedFiltered*.66+speed*.34;
      this.speedPeak=Math.max(this.speedPeak,Math.min(9,this.speedFiltered));
    }
    this.lastDx=dx;this.lastDy=dy;this.lastAt=now;
    const f=this.fold;
    const normalProjection=f.type==='corner'
      ? -.68*(f.sx*dx+f.sy*dy)
      : f.type==='vertical-edge' ? -f.sx*dx : -f.sy*dy;
    this.dragDistance=normalProjection;
    if(this.dragReferenceAngle<.2) {
      // During new folds the hinge follows the trajectory; during unfolding
      // it remains on the existing material scar.
      this.hinge=this.computeHinge(f,dx,dy);
    }
    const ratio=f.type==='corner'?1.65:1.70;
    this.target=clamp(this.dragReferenceAngle+
      (normalProjection>0?Math.max(0,normalProjection-.035):normalProjection)*ratio,
      0,PI-.095);
    this.peakTarget=Math.max(this.peakTarget,this.target);
    return true;
  }

  release(now=performance.now()) {
    if(this.phase!=='dragging')return;
    const wasUnfolding=this.dragReferenceAngle>1;
    // Hysteresis: an ordinary short or gentle pull relaxes; a completed flip
    // or fast final turn latches. No arbitrary "hold" animation is played.
    const fastFlip=this.speedPeak>2.3 && this.target>2.20;
    const completed=this.target>2.75;
    const alreadyLatched=wasUnfolding&&this.target>1.95;
    const latches=completed||fastFlip||alreadyLatched;
    if(this.peakAngle>.75||this.peakTarget>.75){
      this.recordMemory(latches);
    }
    this.transition=latches?'latch':'recover';
    this.target=latches?PI-.095:0;
    this.velocity=0;
    this.phase='settling';
    this.elapsed=0;
  }

  recordMemory(latched=false) {
    if(!this.hinge)return;
    const h=this.hinge;
    const strain=Math.max(this.peakAngle,this.peakTarget);
    const strength=clamp((strain-.65)/(PI-.65),.08,1);
    const previous=this.memories.find(m=>
      Math.hypot(m.hinge.start.x-h.start.x,m.hinge.start.y-h.start.y)<.20 &&
      Math.abs(m.hinge.nx*h.nx+m.hinge.ny*h.ny)>.97
    );
    const copy={
      hinge:{...h,start:{...h.start},end:{...h.end},origin:{...h.origin}},
      strength:latched?Math.max(strength,.70):strength,
      band:this.bandWidth,
      times:(previous?.times||0)+1
    };
    if(previous)this.memories.splice(this.memories.indexOf(previous),1);
    this.memories.push(copy);
    if(this.memories.length>this.creaseLimit)this.memories.shift();
    this.refreshRestingGeometry();
  }

  step(dt=1/60) {
    if(this.phase==='resting'||this.phase==='latched')return;
    const seconds=clamp(dt,1/240,.16);
    const omega=this.phase==='dragging'?19.5:
      this.transition==='latch'?12.5:15;
    const displacement=this.theta-this.target;
    const decay=Math.exp(-omega*seconds);
    const v=(this.velocity+omega*displacement)*seconds;
    this.theta=clamp(this.target+(displacement+v)*decay,0,PI-.085);
    this.velocity=(this.velocity-omega*v)*decay;
    this.elapsed+=seconds;
    this.peakAngle=Math.max(this.peakAngle,this.theta);
    this.drawFolded();
    if(this.phase==='settling'&&Math.abs(this.theta-this.target)<.003){
      if(this.transition==='latch'){
        this.theta=PI-.095;
        this.phase='latched';
        this.drawFolded();
      }else{
        this.fold=null;this.hinge=null;this.theta=0;this.target=0;
        this.phase='resting';
        this.bentArea=0;this.activeVertices=0;
        this.dragDistance=0;this.speedPeak=0;
        this.renderGeometry=this.restingGeometry;
      }
    }
  }

  // Permanent plastic bend is a small *geometry normal change*, not an ink
  // stripe overlaid on top of the texture. Its material-space center remains
  // stable after the flap has been unfolded and when the sheet is viewed obliquely.
  residualHeight(x,y) {
    let height=0;
    for(const memory of this.memories){
      const h=memory.hinge;
      const q=this.q({x,y},h);
      if(Math.abs(q)>.32)continue;
      const ex=h.end.x-h.start.x,ey=h.end.y-h.start.y;
      const len=h.length;
      const along=((x-h.start.x)*ex+(y-h.start.y)*ey)/Math.max(len,1e-5);
      const envelope=smooth((along+.12)/.18)*smooth((len+.12-along)/.18);
      if(envelope<=0)continue;
      const sigma=.066+memory.strength*.027;
      const bump=Math.exp(-((q/sigma)**2));
      const shoulders=Math.exp(-(((Math.abs(q)-2.1*sigma)/(sigma*.87))**2));
      height+=(.013+.018*memory.strength)*envelope*(bump-.21*shoulders);
    }
    return Math.max(-.012,Math.min(.075,height));
  }

  refreshRestingGeometry() {
    const p=this.restingGeometry.attributes.position.array;
    for(let i=0;i<p.length;i+=3){
      p[i+2]=.012+this.residualHeight(this.basePositions[i],this.basePositions[i+1]);
    }
    this.restingGeometry.attributes.position.needsUpdate=true;
    this.restingGeometry.computeVertexNormals();
    this.restingGeometry.computeBoundingBox();
    this.restingGeometry.computeBoundingSphere();
  }

  clip(points, threshold, keepAbove) {
    const output=[];
    for(let i=0;i<points.length;i++){
      const a=points[i],b=points[(i+1)%points.length];
      const da=this.q(a)-threshold,db=this.q(b)-threshold;
      const ina=keepAbove?da>=-1e-7:da<=1e-7;
      const inb=keepAbove?db>=-1e-7:db<=1e-7;
      if(ina!==inb){
        const t=da/(da-db);
        output.push({x:a.x+(b.x-a.x)*t,y:a.y+(b.y-a.y)*t});
      }
      if(inb)output.push(b);
    }
    const clean=[];
    for(const p of output){
      if(!clean.some(q=>Math.hypot(q.x-p.x,q.y-p.y)<1e-6))clean.push(p);
    }
    return clean;
  }

  // θ(u) with zero curvature at each side of the strip.
  angleAt(u) {
    const t=clamp(u,0,1);
    return this.theta*(t-Math.sin(2*PI*t)/(2*PI));
  }

  makeIntegrator() {
    const segments=96,w=this.bandWidth;
    const X=new Float64Array(segments+1);
    const Z=new Float64Array(segments+1);
    const step=w/segments;
    for(let j=1;j<=segments;j++){
      const a=this.angleAt((j-.5)/segments);
      X[j]=X[j-1]+step*Math.cos(a);
      Z[j]=Z[j-1]+step*Math.sin(a);
    }
    return {X,Z,segments,w};
  }

  mapVertex(x,y,integrator){
    const half=this.bandWidth/2,q=this.q({x,y});
    if(q<=-half) return {x,y,z:.012,theta:0};
    const dist=clamp(q+half,0,this.bandWidth);
    const t=dist/this.bandWidth*integrator.segments;
    const i=Math.min(integrator.segments-1,Math.floor(t));
    const f=clamp(t-i,0,1);
    let xArc=integrator.X[i]*(1-f)+integrator.X[i+1]*f;
    let zArc=integrator.Z[i]*(1-f)+integrator.Z[i+1]*f;
    if(q>half){
      xArc+=(q-half)*Math.cos(this.theta);
      zArc+=(q-half)*Math.sin(this.theta);
    }
    const shifted=-half+xArc-q;
    return {
      x:x+shifted*this.hinge.nx,
      y:y+shifted*this.hinge.ny,
      z:.012+zArc,
      theta:this.angleAt(dist/this.bandWidth)
    };
  }

  drawFolded() {
    if(!this.hinge||this.theta<.000001) {
      this.renderGeometry=this.restingGeometry;
      return;
    }
    // Tight creases develop smaller radius after material yield, without a
    // discontinuity. Even a fully latched fold keeps a finite bending band.
    this.bandWidth=clamp(.27-.155*this.theta/PI,.105,.27);
    const half=this.bandWidth/2;
    const N=18,integrator=this.makeIntegrator();
    const p=this.positions,u=this.uvs,n=this.normals,ids=this.indices;
    let vertex=0,tri=0;
    const addPolygon=(polygon)=>{
      if(polygon.length<3)return;
      const first=vertex;
      for(const point of polygon){
        const value=this.mapVertex(point.x,point.y,integrator);
        const vi=vertex*3,ui=vertex*2;
        p[vi]=value.x;p[vi+1]=value.y;p[vi+2]=value.z;
        u[ui]=(point.x+this.width/2)/this.width;
        u[ui+1]=(point.y+this.height/2)/this.height;
        n[vi]=-this.hinge.nx*Math.sin(value.theta);
        n[vi+1]=-this.hinge.ny*Math.sin(value.theta);
        n[vi+2]=Math.cos(value.theta);
        vertex++;
      }
      for(let i=1;i<polygon.length-1;i++){
        ids[tri++]=first;ids[tri++]=first+i;ids[tri++]=first+i+1;
      }
    };
    addPolygon(this.clip(this.rectangle,-half,false));
    for(let k=0;k<N;k++){
      const low=-half+this.bandWidth*k/N;
      const high=-half+this.bandWidth*(k+1)/N;
      addPolygon(this.clip(this.clip(this.rectangle,low,true),high,false));
    }
    addPolygon(this.clip(this.rectangle,half,true));
    if(vertex>this.maxVertices||tri>this.maxIndices)
      throw new Error('paper topology exceeds bounded geometry allocation');
    this.activeVertices=vertex;
    this.bentArea= this.hinge.kind==='corner'
      ? .5*(this.hinge.depth/Math.abs(this.hinge.nx))*
         (this.hinge.depth/Math.abs(this.hinge.ny))/
         (this.width*this.height)
      : this.hinge.kind==='edge'
        ? this.hinge.depth / (this.hinge.nx ? this.width : this.height)
        :0;
    this.foldedGeometry.setDrawRange(0,tri);
    for(const key of ['position','uv','normal'])
      this.foldedGeometry.attributes[key].needsUpdate=true;
    this.foldedGeometry.index.needsUpdate=true;
    this.foldedGeometry.computeBoundingSphere();
    this.foldedGeometry.computeBoundingBox();
    this.renderGeometry=this.foldedGeometry;
  }

  getDiagnostics() {
    return {
      solver:'finite-radius developable hinge + plastic crease memory',
      phase:this.phase,
      grabType:this.fold?.type??'none',
      angleDegrees:+(this.theta*180/PI).toFixed(1),
      targetDegrees:+(this.target*180/PI).toFixed(1),
      creaseWidth:+this.bandWidth.toFixed(3),
      flatFraction:+(1-this.bentArea).toFixed(4),
      activeVertices:this.activeVertices,
      inkUVsFixed:true,
      speedPeak:+this.speedPeak.toFixed(2),
      dragDistance:+this.dragDistance.toFixed(3),
      plasticCreases:this.memories.length,
      memoryStrengths:this.memories.map(m=>+m.strength.toFixed(2)),
      latch:this.phase==='latched'||(this.phase==='settling'&&this.transition==='latch'),
      direction:this.hinge?+(Math.atan2(this.hinge.ny,this.hinge.nx)*180/PI).toFixed(1):null
    };
  }
}
