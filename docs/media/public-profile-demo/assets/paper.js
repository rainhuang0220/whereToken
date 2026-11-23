// The CSS paper is usable immediately. Only fine, motion-enabled desktops
// request the worker-backed physical sheet.
(() => {
  const root = document.documentElement;
  const fine = matchMedia('(pointer:fine)');
  const reduced = matchMedia('(prefers-reduced-motion:reduce)');
  const small = matchMedia('(max-width:700px)');
  let requested = false;
  function load() {
    if (requested || root.dataset.activityPalette !== 'newsprint' ||
      !fine.matches || reduced.matches || small.matches) return;
    requested = true;
    const script = document.createElement('script');
    script.src = './assets/paper-3d.js?v=0d0fc55e1da05f82ab4c8698b4adc28cdb8d08654aecc2d99754f70efbdf810c';
    document.head.append(script);
  }
  new MutationObserver(load).observe(root, { attributes: true, attributeFilter: ['data-activity-palette'] });
  for (const query of [fine, reduced, small]) query.addEventListener('change', load);
  load();
})();
