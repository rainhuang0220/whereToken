(function () {
  "use strict";
  var TILE = 560;
  var light = [-0.34, -0.28, 0.9];
  var target = [-0.34, -0.28, 0.9];
  var canvas = null;
  var gl = null;
  var prog = null;
  var tex = null;
  var buf = null;
  var raf = 0;
  var last = 0;
  var active = false;
  var token = 0;
  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)");
  var fine = window.matchMedia("(pointer: fine)");

  var vert =
    "#version 300 es\n" +
    "layout(location=0) in vec2 a;\n" +
    "void main(){ gl_Position = vec4(a, 0.0, 1.0); }\n";

  var frag =
    "#version 300 es\n" +
    "precision highp float;\n" +
    "uniform sampler2D uHeight;\n" +
    "uniform vec3 uLight;\n" +
    "uniform vec2 uTile;\n" +
    "uniform vec2 uRes;\n" +
    "uniform vec4 uSheet;\n" +
    "uniform float uSheetOn;\n" +
    "out vec4 o;\n" +
    "float heightAt(vec2 uv){\n" +
    "  float fineH = texture(uHeight, uv).r;\n" +
    "  float form = textureLod(uHeight, uv, 2.2).r;\n" +
    "  float broad = textureLod(uHeight, uv, 4.4).r;\n" +
    "  return broad * 0.18 + form * 0.47 + fineH * 0.35;\n" +
    "}\n" +
    "void main(){\n" +
    "  vec2 px = vec2(gl_FragCoord.x, uRes.y - gl_FragCoord.y);\n" +
    "  if (uSheetOn > 0.5) {\n" +
    "    if (px.x < uSheet.x || px.y < uSheet.y || px.x > uSheet.x + uSheet.z || px.y > uSheet.y + uSheet.w) {\n" +
    "      o = vec4(0.078, 0.071, 0.063, 1.0);\n" +
    "      return;\n" +
    "    }\n" +
    "  }\n" +
    "  vec2 uv = px / uTile;\n" +
    "  vec2 texel = vec2(1.0) / vec2(textureSize(uHeight, 0));\n" +
    "  float hL = heightAt(uv - vec2(texel.x, 0.0));\n" +
    "  float hR = heightAt(uv + vec2(texel.x, 0.0));\n" +
    "  float hD = heightAt(uv - vec2(0.0, texel.y));\n" +
    "  float hU = heightAt(uv + vec2(0.0, texel.y));\n" +
    "  float hC = heightAt(uv);\n" +
    "  float scale = 28.0;\n" +
    "  vec3 n = normalize(vec3(-(hR - hL) * scale, -(hU - hD) * scale, 1.0));\n" +
    "  vec3 L = normalize(uLight);\n" +
    "  vec3 V = vec3(0.0, 0.0, 1.0);\n" +
    "  float ndotl = clamp(dot(n, L), 0.04, 1.0);\n" +
    "  float ndotv = clamp(dot(n, V), 0.04, 1.0);\n" +
    "  float rough = clamp(0.78 + 0.14 * (texture(uHeight, uv).r - textureLod(uHeight, uv, 2.2).r), 0.7, 0.95);\n" +
    "  float sigma = 0.25 + 0.65 * rough;\n" +
    "  float s2 = sigma * sigma;\n" +
    "  float A = 1.0 - 0.5 * s2 / (s2 + 0.33);\n" +
    "  float B = 0.45 * s2 / (s2 + 0.09);\n" +
    "  float thetaI = acos(ndotl);\n" +
    "  float thetaR = acos(ndotv);\n" +
    "  float alpha = max(thetaI, thetaR);\n" +
    "  float beta = min(min(thetaI, thetaR), 0.85);\n" +
    "  vec3 lv = L - n * dot(n, L);\n" +
    "  vec3 vv = V - n * dot(n, V);\n" +
    "  float cosPhi = 0.0;\n" +
    "  if (dot(lv, lv) > 1e-5 && dot(vv, vv) > 1e-5) cosPhi = clamp(dot(normalize(lv), normalize(vv)), -1.0, 1.0);\n" +
    "  float diff = ndotl * (A + B * max(0.0, cosPhi) * sin(alpha) * tan(beta));\n" +
    "  vec3 H = normalize(L + V);\n" +
    "  float ndoth = clamp(dot(n, H), 0.0, 1.0);\n" +
    "  float a = rough * rough;\n" +
    "  float a2 = a * a;\n" +
    "  float d = ndoth * ndoth * (a2 - 1.0) + 1.0;\n" +
    "  float D = a2 / (3.14159265 * d * d);\n" +
    "  float spec = ndotl * D * 0.012;\n" +
    "  float form = textureLod(uHeight, uv, 2.2).r;\n" +
    "  vec3 albedo = vec3(0.955, 0.945, 0.915) * mix(0.94, 1.03, form);\n" +
    "  vec2 page = px / uRes * 2.0 - 1.0;\n" +
    "  vec2 lxy = L.xy;\n" +
    "  float llen = length(lxy);\n" +
    "  vec2 ldir = llen > 1e-4 ? lxy / llen : vec2(-0.77, -0.64);\n" +
    "  float side = clamp(dot(ldir, page) * 0.62 + 0.50, 0.0, 1.0);\n" +
    "  float lift = mix(0.88, 1.14, side);\n" +
    "  vec3 tint = mix(vec3(0.986, 0.988, 1.0), vec3(1.0, 0.993, 0.978), side);\n" +
    "  vec3 color = albedo * (0.58 + 0.46 * diff) * lift * tint + vec3(1.0, 0.98, 0.94) * spec;\n" +
    "  color = pow(max(color, 0.0), vec3(1.0 / 2.2));\n" +
    "  o = vec4(color, 1.0);\n" +
    "}\n";

  function tracking() {
    return fine.matches && !reduce.matches;
  }

  function sheet() {
    var theme = document.documentElement.getAttribute("data-theme");
    var dark = theme === "dark" || (theme !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    if (!dark) return null;
    var w = Math.min(1120, window.innerWidth);
    return { x: (window.innerWidth - w) / 2, y: 0, w: w, h: window.innerHeight };
  }

  function compile(type, src) {
    var shader = gl.createShader(type);
    gl.shaderSource(shader, src);
    gl.compileShader(shader);
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
      gl.deleteShader(shader);
      return null;
    }
    return shader;
  }

  function boot(image) {
    canvas = document.createElement("canvas");
    canvas.id = "wt-paper";
    canvas.setAttribute("aria-hidden", "true");
    gl = canvas.getContext("webgl2", {
      alpha: false,
      antialias: false,
      depth: false,
      stencil: false,
      powerPreference: "low-power",
    });
    if (!gl) return false;
    var vs = compile(gl.VERTEX_SHADER, vert);
    var fs = compile(gl.FRAGMENT_SHADER, frag);
    if (!vs || !fs) return false;
    prog = gl.createProgram();
    gl.attachShader(prog, vs);
    gl.attachShader(prog, fs);
    gl.linkProgram(prog);
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) return false;
    buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
    tex = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, 1);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.generateMipmap(gl.TEXTURE_2D);
    document.body.prepend(canvas);
    document.documentElement.classList.add("wt-paper-live");
    return true;
  }

  function resize() {
    if (!canvas || !gl) return;
    var dpr = Math.min(window.devicePixelRatio || 1, fine.matches ? 2 : 1.5);
    var w = Math.max(1, Math.round(window.innerWidth * dpr));
    var h = Math.max(1, Math.round(window.innerHeight * dpr));
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
    }
  }

  function draw() {
    if (!gl || !prog) return;
    resize();
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.useProgram(prog);
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.uniform1i(gl.getUniformLocation(prog, "uHeight"), 0);
    gl.uniform3f(gl.getUniformLocation(prog, "uLight"), light[0], light[1], light[2]);
    var dpr = canvas.width / Math.max(1, window.innerWidth);
    gl.uniform2f(gl.getUniformLocation(prog, "uTile"), TILE * dpr, TILE * dpr * 0.623);
    gl.uniform2f(gl.getUniformLocation(prog, "uRes"), canvas.width, canvas.height);
    var box = sheet();
    if (box) {
      gl.uniform1f(gl.getUniformLocation(prog, "uSheetOn"), 1);
      gl.uniform4f(gl.getUniformLocation(prog, "uSheet"), box.x * dpr, box.y * dpr, box.w * dpr, box.h * dpr);
    } else {
      gl.uniform1f(gl.getUniformLocation(prog, "uSheetOn"), 0);
      gl.uniform4f(gl.getUniformLocation(prog, "uSheet"), 0, 0, canvas.width, canvas.height);
    }
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
  }

  function tick(now) {
    raf = 0;
    if (!active) return;
    var dt = last ? Math.min(0.05, (now - last) / 1000) : 0.016;
    last = now;
    var k = tracking() ? 1 - Math.exp(-dt / 0.45) : 1;
    light[0] += (target[0] - light[0]) * k;
    light[1] += (target[1] - light[1]) * k;
    light[2] += (target[2] - light[2]) * k;
    draw();
    if (Math.hypot(target[0] - light[0], target[1] - light[1]) > 0.004) kick();
  }

  function kick() {
    if (!raf && active) raf = requestAnimationFrame(tick);
  }

  function onPointer(event) {
    if (!tracking()) return;
    var nx = event.clientX / Math.max(1, window.innerWidth) * 2 - 1;
    var ny = event.clientY / Math.max(1, window.innerHeight) * 2 - 1;
    target = [nx * 0.62, ny * 0.42, 0.78];
    kick();
  }

  function onResize() {
    draw();
  }

  function stop() {
    token += 1;
    active = false;
    if (raf) cancelAnimationFrame(raf);
    raf = 0;
    window.removeEventListener("pointermove", onPointer);
    window.removeEventListener("resize", onResize);
    document.removeEventListener("visibilitychange", onVisible);
    if (gl) {
      var lose = gl.getExtension("WEBGL_lose_context");
      if (lose) lose.loseContext();
    }
    if (canvas && canvas.parentNode) canvas.parentNode.removeChild(canvas);
    canvas = null;
    gl = null;
    prog = null;
    tex = null;
    document.documentElement.classList.remove("wt-paper-live");
  }

  function onVisible() {
    if (document.hidden) {
      if (raf) cancelAnimationFrame(raf);
      raf = 0;
      return;
    }
    draw();
  }

  function start() {
    if (active || !wantPaper()) return;
    var mine = ++token;
    var img = new Image();
    img.onload = function () {
      if (mine !== token || !wantPaper()) return;
      if (!boot(img)) {
        stop();
        return;
      }
      active = true;
      target = [-0.34, -0.28, 0.9];
      light = target.slice();
      window.addEventListener("pointermove", onPointer, { passive: true });
      window.addEventListener("resize", onResize);
      document.addEventListener("visibilitychange", onVisible);
      draw();
    };
    img.src = "./assets/newsprint-height.jpg";
  }

  function wantPaper() {
    return document.documentElement.getAttribute("data-activity-palette") === "newsprint";
  }

  function sync() {
    if (wantPaper()) {
      if (!active) start();
      else draw();
    } else if (active) {
      stop();
    }
  }

  new MutationObserver(sync).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-activity-palette", "data-theme"],
  });
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", sync);
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", sync);
  } else {
    sync();
  }
})();
