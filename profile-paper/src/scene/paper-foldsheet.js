import { PaperResponsive } from './paper-responsive.js';

const PI=Math.PI;
const clamp=(x,a,b)=>Math.max(a,Math.min(b,x));
const smooth=(x)=>{const t=clamp(x,0,1);return t*t*(3-2*t)};

/**
 * D4: exact continuous single-crease folding of an intact rectangular sheet.
 * There are no free interior edges and NO local edge notch.
 * A corner fold can move across much of the page. An edge grip folds a
 * continuous side panel, potentially all the way to the sheet midpoint.
 */
export class PaperFoldSheet extends PaperResponsive {
  constructor(opts={}) {
    super(opts);
    // Closer to a physical, faint embossed crease. The inherited original
    // 156x234 plane has a 0.05-unit pitch, which blurs tiny creases.
    this.restingGeometry.dispose();
    this.restingGeometry=new (this.restingGeometry.constructor)(this.width,this.height,480,720);
    this.basePositions=new Float32Array(this.restingGeometry.attributes.position.array);
    this.lastReleaseAngle=0;
    this.maxFoldAngle=PI-.018;
    this.reset({clearMemories:true});
  }

  lineIntersections(nx,ny,depth,origin) {
    const q=p=>(p.x-origin.x)*nx+(p.y-origin.y)*ny+depth;
    const points=[];
    const R=this.rectangle;
    for(let i=0;i<R.length;i++){
      const a=R[i],b=R[(i+1)%R.length],da=q(a),db=q(b);
      if(Math.abs(da)<1e-7)points.push({...a});
      if(da*db<0){
        const t=da/(da-db);
        points.push({x:a.x+t*(b.x-a.x),y:a.y+t*(b.y-a.y)});
      }
    }
    const filtered=[];
    for(const p of points)
      if(!filtered.some(v=>Math.hypot(v.x-p.x,v.y-p.y)<1e-6))filtered.push(p);
    if(filtered.length<2)return null;
    let best=null,dist=-1;
    for(let i=0;i<filtered.length;i++)for(let j=i+1;j<filtered.length;j++){
      const v=Math.hypot(filtered[i].x-filtered[j].x,filtered[i].y-filtered[j].y);
      if(v>dist){dist=v;best=[filtered[i],filtered[j]]}
    }
    return {start:best[0],end:best[1],length:dist};
  }

  computeHinge(grab,dx,dy) {
    let nx,ny,depth,origin,kind;
    if(grab.type==='corner'){
      const ix=Math.max(0,-grab.sx*dx),iy=Math.max(0,-grab.sy*dy);
      const total=Math.hypot(ix,iy);
      // The crease normal follows the real drag direction, while the
      // small baseline avoids degeneracy for a purely vertical/side motion.
      const weightX=.36+ix/(total+.12),weightY=.36+iy/(total+.12);
      const magnitude=Math.hypot(weightX,weightY);
      nx=grab.sx*weightX/magnitude;
      ny=grab.sy*weightY/magnitude;
      origin={x:grab.x,y:grab.y};
      const maxProjection=Math.abs(nx)*this.width+Math.abs(ny)*this.height;
      depth=clamp(.20+.87*total,.20,maxProjection*.535);
      kind='corner';
    } else {
      const vertical=grab.type==='vertical-edge';
      const sign=vertical?grab.sx:grab.sy;
      const inward=Math.max(0,-sign*(vertical?dx:dy));
      const sideways=vertical?dy:dx;
      // A side crease goes across the ENTIRE sheet. Minor rotation from
      // tangential motion is permitted but never causes an internal flap edge.
      const spanOther=vertical?this.height:this.width;
      const sheetAcross=vertical?this.width:this.height;
      const rawTilt=clamp(sideways*.045,-.065,.065);
      const allowance=Math.max(0,.92*inward)/(spanOther/2+1e-6);
      const tilt=clamp(rawTilt,-allowance,allowance);
      const n=1/Math.hypot(1,tilt);
      nx=vertical?sign*n:tilt*n;
      ny=vertical?tilt*n:sign*n;
      origin={x:grab.x,y:grab.y};
      depth=clamp(.10+.91*inward,.10,sheetAcross*.535);
      kind='edge';
    }
    const intersection=this.lineIntersections(nx,ny,depth,origin);
    if(!intersection){
      // For legal nonzero grab, stay just inside the boundary when the
      // crease nearly coincides with a corner of the original rectangle.
      depth=Math.max(.07,depth-.04);
    }
    const cut=intersection||this.lineIntersections(nx,ny,depth,origin);
    if(!cut)throw new Error('Crease must intersect original sheet boundary twice');
    return {nx,ny,depth,origin,kind,fullSpan:true,
      start:cut.start,end:cut.end,length:cut.length,span:cut.length,
      alongShift:0};
  }

  // No synthetic fade-out at the center of an edge: it would create the
  // notch and impossible discontinuity the user rejected.
  localWindow(){ return 1; }
  cornerWindow(x,y,h){
    const ex=h.end.x-h.start.x,ey=h.end.y-h.start.y,len=Math.max(.001,h.length);
    const t=((x-h.start.x)*ex+(y-h.start.y)*ey)/len;
    return smooth((t+.085)/.14)*smooth((len+.085-t)/.14);
  }

  // Narrow pressure crease, slight asymmetric shoulders, and subtle
  // irregularity along its fiber direction. The valley is truly inside the
  // sheet geometry; a smooth core avoids dotted highlights on the rest grid.
  residualHeight(x,y){
    let z=0;
    for(const m of this.memories){
      const h=m.hinge,ex=h.end.x-h.start.x,ey=h.end.y-h.start.y;
      const len=Math.max(1e-4,h.length);
      const t=((x-h.start.x)*ex+(y-h.start.y)*ey)/len;
      const extent=smooth((t+.07)/.13)*smooth((len+.07-t)/.13);
      if(extent<1e-4)continue;
      const longitudinal=t/len;
      const irregular=.0025*Math.sin(t*32.1)+.0015*Math.sin(t*83.7+.7);
      const q=this.q({x,y},h)-irregular;
      const core=.024+.008*m.strength;
      if(Math.abs(q)>.18)continue;
      const valley=Math.exp(-((q/core)**2));
      const left=Math.max(0,1-Math.abs((q+.063)/.041));
      const right=Math.max(0,1-Math.abs((q-.071)/.044));
      const pressure=(.015+.017*m.strength)*
        (1+.07*Math.sin(longitudinal*19.3)+.04*Math.sin(longitudinal*58.9));
      z+=extent*pressure*(-valley+.13*left+.10*right);
    }
    return clamp(z,-.044,.012);
  }

  refreshRestingGeometry(){
    const position=this.restingGeometry?.attributes?.position;
    if(!position)return;
    const points=position.array,normals=this.restingGeometry.attributes.normal.array;
    const epsilon=.003;
    for(let i=0;i<points.length;i+=3){
      const x=this.basePositions[i],y=this.basePositions[i+1];
      points[i+2]=.012+this.residualHeight(x,y);
      const dx=(this.residualHeight(x+epsilon,y)-this.residualHeight(x-epsilon,y))/(2*epsilon);
      const dy=(this.residualHeight(x,y+epsilon)-this.residualHeight(x,y-epsilon))/(2*epsilon);
      const norm=Math.hypot(dx,dy,1);
      normals[i]=-dx/norm;normals[i+1]=-dy/norm;normals[i+2]=1/norm;
    }
    position.needsUpdate=true;this.restingGeometry.attributes.normal.needsUpdate=true;
    this.restingGeometry.computeBoundingBox();
    this.restingGeometry.computeBoundingSphere();
  }

  drawFolded(){
    super.drawFolded();
    if(this.hinge&&this.theta>1e-5){
      // Exact clipped flap polygon area. Previous UI used a triangle
      // approximation that became inaccurate for half-page corner folds.
      const poly=this.clip(this.rectangle,0,true);
      let twice=0;
      for(let i=0;i<poly.length;i++){
        const a=poly[i],b=poly[(i+1)%poly.length];
        twice+=a.x*b.y-b.x*a.y;
      }
      this.bentArea=Math.min(1,Math.abs(twice)*.5/(this.width*this.height));
    }
  }

  getDiagnostics(){
    const d=super.getDiagnostics();
    return {...d,solver:'continuous full-sheet fold / clipped line hinge',
      geometry:'single unbroken sheet; both ends of crease on original boundary',
      creaseProfile:'narrow smooth compressed valley with asymmetric shoulders'};
  }
}
