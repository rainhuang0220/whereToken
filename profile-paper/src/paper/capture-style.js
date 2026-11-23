// Only reuse optical output when the document's capture styles are stable.
// A stylesheet with inaccessible rules or an asynchronous font needs a capture.
export function captureStyleKey(document) {
  if (document.fonts?.size) return null;
  const { documentElement: root, body } = document;
  try {
    const sheets = Array.from(document.styleSheets, sheet => [
      sheet.href, sheet.disabled, sheet.media.mediaText,
      Array.from(sheet.cssRules, rule => rule.cssText),
    ]);
    return JSON.stringify([root.lang, root.dataset.theme, root.className,
      root.getAttribute('style'), body.className, body.getAttribute('style'), sheets]);
  } catch {
    return null;
  }
}
