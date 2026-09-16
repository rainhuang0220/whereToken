package main

import "fmt"

func fiberPrimary() []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="131" height="127" viewBox="0 0 131 127">
  <filter id="fiber-a" x="0" y="0" width="100%" height="100%" color-interpolation-filters="sRGB">
    <feTurbulence type="fractalNoise" baseFrequency="0.082 0.39" numOctaves="2" seed="7341" stitchTiles="stitch" result="height"/>
    <feGaussianBlur in="height" stdDeviation="0.20 0.46" result="soft-height"/>
    <feDiffuseLighting in="soft-height" surfaceScale="0.58" diffuseConstant="0.72" lighting-color="#808080" result="lit">
      <feDistantLight azimuth="315" elevation="58"/>
    </feDiffuseLighting>
    <feComponentTransfer in="lit">
      <feFuncR type="linear" slope="0.27" intercept="0.365"/>
      <feFuncG type="linear" slope="0.27" intercept="0.365"/>
      <feFuncB type="linear" slope="0.27" intercept="0.365"/>
      <feFuncA type="linear" slope="0.58"/>
    </feComponentTransfer>
  </filter>
  <rect width="131" height="127" fill="#808080" filter="url(#fiber-a)"/>
</svg>
`)
}

func fiberSecondary() []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="173" height="149" viewBox="0 0 173 149">
  <filter id="fiber-b" x="0" y="0" width="100%" height="100%" color-interpolation-filters="sRGB">
    <feTurbulence type="fractalNoise" baseFrequency="0.34 0.068" numOctaves="2" seed="9021" stitchTiles="stitch" result="height"/>
    <feGaussianBlur in="height" stdDeviation="0.42 0.16" result="soft-height"/>
    <feDiffuseLighting in="soft-height" surfaceScale="0.42" diffuseConstant="0.70" lighting-color="#808080" result="lit">
      <feDistantLight azimuth="318" elevation="60"/>
    </feDiffuseLighting>
    <feComponentTransfer in="lit">
      <feFuncR type="linear" slope="0.18" intercept="0.41"/>
      <feFuncG type="linear" slope="0.18" intercept="0.41"/>
      <feFuncB type="linear" slope="0.18" intercept="0.41"/>
      <feFuncA type="linear" slope="0.34"/>
    </feComponentTransfer>
  </filter>
  <rect width="173" height="149" fill="#808080" filter="url(#fiber-b)"/>
</svg>
`)
}

func swatchHTML() []byte {
	body := fmt.Sprintf(`<!doctype html>
<html lang="en"><meta charset="utf-8"><title>Newsprint material swatch</title>
<style>
*{box-sizing:border-box}html,body{margin:0;width:1200px;height:900px;overflow:hidden}
body{
  padding:72px;
  background-color:#f2f0e9;
  background-image:url("../../internal/profilewebembed/static/assets/newsprint-fiber.svg"),url("../../internal/profilewebembed/static/assets/newsprint-fiber-b.svg"),url("../../internal/profilewebembed/static/assets/newsprint-surface.png");
  background-size:131px 127px,173px 149px,%dpx %dpx;
  background-repeat:repeat,repeat,repeat;
  background-position:0 0,41px 23px,0 0;
  background-blend-mode:soft-light,soft-light,soft-light;
  color:#171717;
  font:14px/1.5 Georgia,serif;
}
.label{margin:0 0 28px;font:600 13px/1.2 ui-monospace,monospace;letter-spacing:.12em;text-transform:uppercase}
.blocks{display:flex;gap:28px;align-items:flex-end;max-width:520px}
.block{width:240px;height:150px;background-image:url("../../internal/profilewebembed/static/assets/newsprint-fiber.svg"),url("../../internal/profilewebembed/static/assets/newsprint-fiber-b.svg");background-size:28px 27px,37px 31px;background-blend-mode:soft-light,soft-light}
.gray{background-color:#b6b5b0}
.black{background-color:#4a4946}
.rules{margin-top:36px;max-width:420px}
.one{border-top:1px solid #171717}
.two{margin-top:22px;border-top:2px solid #171717}
.copy{max-width:420px;margin-top:28px;font-size:15px}
.copy strong{font-size:20px}
</style>
<body>
<p class="label">Newsprint material · heterogeneous sheet · folds 0</p>
<div class="blocks"><div class="block gray"></div><div class="block black"></div></div>
<div class="rules"><div class="one"></div><div class="two"></div></div>
<p class="copy"><strong>whereToken local token accounting</strong><br>Matte ink sits on a continuous, lightly cockled sheet. Type remains crisp while the substrate carries the material character.</p>
</body></html>
`, displayWidth, displayHeight)
	return []byte(body)
}
