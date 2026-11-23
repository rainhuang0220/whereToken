import { readFile } from 'node:fs/promises';

// Use the locked html2canvas parser and painter with cooperative DOM traversal.
// Exact anchors make a dependency upgrade fail at build time if it needs review.
export const incrementalCapture = {
  name: 'incremental-profile-capture',
  setup(build) {
    build.onLoad({ filter: /html2canvas\.esm\.js$/ }, async ({ path }) => {
      let source = await readFile(path, 'utf8');
      const replace = (from, to) => {
        if (source.split(from).length !== 2) throw new Error('Review html2canvas capture anchor: ' + from);
        source = source.replace(from, to);
      };
      replace("this.documentElement = this.cloneNode(element.ownerDocument.documentElement, false);",
        "this.documentElement = null;");
      replace("var iframe = createIFrameContainer(ownerDocument, windowSize);",
        "var iframe = this.frame = createIFrameContainer(ownerDocument, windowSize);");
      replace("var iframeLoad = iframeLoader(iframe)", "var iframeLoad = this.options.waitForFrame(iframe)");
      replace("prototype.appendChildNode = function (clone, child, copyStyles)",
        "prototype.appendChildNode = async function (clone, child, copyStyles)");
      replace("clone.appendChild(this.cloneNode(child, copyStyles));",
        "clone.appendChild(await this.cloneNode(child, copyStyles));");
      replace("prototype.cloneChildNodes = function (node, clone, copyStyles)",
        "prototype.cloneChildNodes = async function (node, clone, copyStyles)");
      replace("assignedNodes.forEach(function (assignedNode) { return _this.appendChildNode(clone, assignedNode, copyStyles); });",
        "for (const assignedNode of assignedNodes) await this.appendChildNode(clone, assignedNode, copyStyles);");
      replace("this.appendChildNode(clone, child, copyStyles);",
        "await this.appendChildNode(clone, child, copyStyles);");
      replace("createPseudoHideStyles(clone);", "/* Pseudo suppression comes from profile.css under CSP. */");
      replace("prototype.cloneNode = function (node, copyStyles) {",
        "prototype.cloneNode = async function (node, copyStyles) { await this.options.checkpoint();");
      replace("this.cloneChildNodes(node, clone, copyStyles);",
        "await this.cloneChildNodes(node, clone, copyStyles);");

      const start = source.indexOf('var parseNodeTree = function');
      const end = source.indexOf('var createContainer = function');
      if (start < 0 || end < start) throw new Error('Review html2canvas node traversal');
      const traversal = source.slice(start, end)
        .replaceAll('parseNodeTree', 'parseProfileNodes')
        .replace('= function (context, node, parent, root)', '= async function (context, node, parent, root, checkpoint)')
        .replace('nextNode = childNode.nextSibling;', 'await checkpoint();\n        nextNode = childNode.nextSibling;')
        .replace('childNode.assignedNodes().forEach(function (childNode) { return parseProfileNodes(context, childNode, parent, root); });',
          'for (const assigned of childNode.assignedNodes()) await parseProfileNodes(context, assigned, parent, root, checkpoint);')
        .replace('parseProfileNodes(context, childNode.shadowRoot, container, root);',
          'await parseProfileNodes(context, childNode.shadowRoot, container, root, checkpoint);')
        .replace('parseProfileNodes(context, childNode, container, root);',
          'await parseProfileNodes(context, childNode, container, root, checkpoint);');
      return { contents: source + '\n' + traversal +
        '\nexport { Bounds, Context, DocumentCloner, CanvasRenderer, createContainer, parseProfileNodes };',
        loader: 'js' };
    });
  },
};
