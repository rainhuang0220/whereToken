import { Bounds, Context, DocumentCloner, CanvasRenderer, createContainer, parseProfileNodes } from 'html2canvas';

const task = () => new Promise(resolve => setTimeout(resolve, 0));

function whileCurrent(work, signal) {
  signal.throwIfAborted();
  let onAbort;
  const interrupted = new Promise((resolve, reject) => {
    onAbort = () => reject(signal.reason);
    signal.addEventListener('abort', onAbort, { once: true });
  });
  return Promise.race([work, interrupted]).finally(() => signal.removeEventListener('abort', onAbort));
}

function waitForFrame(iframe, signal) {
  return new Promise((resolve, reject) => {
    const close = () => {
      iframe.removeEventListener('load', load);
      signal.removeEventListener('abort', abort);
    };
    const load = () => {
      if (!iframe.contentDocument?.body?.childNodes.length) return;
      close();
      resolve(iframe);
    };
    const abort = () => {
      close();
      reject(signal.reason);
    };
    signal.addEventListener('abort', abort, { once: true });
    iframe.addEventListener('load', load);
  });
}

export async function captureProfile(layout, signal) {
  const started = performance.now();
  let deadline = started + 7;
  const checkpoint = async () => {
    signal.throwIfAborted();
    if (performance.now() < deadline) return;
    await (globalThis.scheduler?.yield?.() ?? task());
    signal.throwIfAborted();
    deadline = performance.now() + 7;
  };
  const { width, height, viewportWidth, viewportHeight, scale } = layout;
  const context = new Context({ logging: false, allowTaint: false, imageTimeout: 2000 },
    new Bounds(0, 0, viewportWidth, viewportHeight));
  const cloner = new DocumentCloner(context, document.body, {
    checkpoint,
    waitForFrame: iframe => waitForFrame(iframe, signal),
    ignoreElements: node => node.id === 'wt-paper-stock' || node.id === 'wt-paper-fold' ||
      node.id === 'tip' || node.tagName === 'DIALOG',
    onclone: doc => {
      doc.documentElement.classList.remove('wt-physical-print', 'wt-paper-folding');
      doc.documentElement.classList.add('wt-paper-capture');
      // Match the live content box while preserving the iframe's media-query viewport.
      doc.documentElement.style.width = width + 'px';
    },
  });
  const times = {};
  let mark = started;
  function measure(phase) {
    const end = performance.now();
    times[phase] = Math.round(end - mark);
    performance.clearMeasures('Newsprint capture ' + phase);
    performance.measure('Newsprint capture ' + phase, { start: mark, end });
    mark = end;
  }
  try {
    cloner.documentElement = await cloner.cloneNode(document.documentElement, false);
    await checkpoint();
    measure('clone');
    await whileCurrent(cloner.toIFrame(document, context.windowBounds), signal);
    await checkpoint();
    measure('layout');
    const body = cloner.clonedReferenceElement;
    // .x/.y remain document-local even when the live page is scrolled.
    const tree = createContainer(context, body);
    tree.flags |= 4; // html2canvas CREATES_REAL_STACKING_CONTEXT
    await parseProfileNodes(context, body, tree, tree, checkpoint);
    measure('parse');

    const canvas = new OffscreenCanvas(Math.floor(width * scale), Math.floor(height * scale));
    const renderer = new CanvasRenderer(context, { canvas, width, height, scale, x: 0, y: 0 });
    for (const method of ['renderStack', 'renderNode', 'renderTextNode']) {
      const render = renderer[method].bind(renderer);
      renderer[method] = async (...args) => {
        await checkpoint();
        return render(...args);
      };
    }
    await whileCurrent(renderer.render(tree), signal);
    await checkpoint();
    measure('paint');
    const bitmap = canvas.transferToImageBitmap();
    measure('transfer');
    return { bitmap, captureMs: Math.round(performance.now() - started), times };
  } finally {
    cloner.frame?.remove();
  }
}
