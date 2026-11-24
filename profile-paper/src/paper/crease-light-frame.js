import { CSS_PIXELS_PER_UNIT, projectCreases } from '../scene/crease-memory.js';
import { stockFrame } from './paper-frame-keys.js';

const canonical = value => Math.round(value * 1e7) / 1e7;
export const LIGHT_VERTEX_PITCH = 2.4;
// Includes the residual solver's 18.5px radius and mesh interpolation.
const PADDING = 24;

function overlaps(a, b) {
  return a.x < b.right && b.x < a.right && a.y < b.bottom && b.y < a.bottom;
}

export function creaseLightFrames(layout, saved, revision) {
  const { width: w, height: h } = layout;
  const memories = projectCreases(saved, w / CSS_PIXELS_PER_UNIT, h / CSS_PIXELS_PER_UNIT);
  if (!memories.length) return [];
  const { key: material, width, height } = stockFrame(layout);
  // Suppress only floating-point noise from translating a world-space line.
  const css = point => [
    canonical(w / 2 + point.x * CSS_PIXELS_PER_UNIT),
    canonical(h / 2 - point.y * CSS_PIXELS_PER_UNIT),
  ];
  const groups = [];
  for (const [index, { hinge, strength }] of memories.entries()) {
    const start = css(hinge.start), end = css(hinge.end);
    const group = {
      x: Math.max(0, Math.floor((Math.min(start[0], end[0]) - PADDING) * width / w)),
      y: Math.max(0, Math.floor((Math.min(start[1], end[1]) - PADDING) * height / h)),
      right: Math.min(width, Math.ceil((Math.max(start[0], end[0]) + PADDING) * width / w)),
      bottom: Math.min(height, Math.ceil((Math.max(start[1], end[1]) + PADDING) * height / h)),
      members: [{ index, shape: [css(hinge.origin), start, end, hinge.nx, hinge.ny, hinge.depth, strength,
        hinge.direction, hinge.fiberStart, hinge.fiberLength] }],
    };
    // Merged boxes must be disjoint. Overlapping creases use one draw so
    // lighting is evaluated on their combined height field at each pixel.
    for (let match = groups.findIndex(other => overlaps(group, other)); match >= 0;
      match = groups.findIndex(other => overlaps(group, other))) {
      const [other] = groups.splice(match, 1);
      group.x = Math.min(group.x, other.x);
      group.y = Math.min(group.y, other.y);
      group.right = Math.max(group.right, other.right);
      group.bottom = Math.max(group.bottom, other.bottom);
      group.members.push(...other.members);
    }
    groups.push(group);
  }
  return groups.map(({ x, y, right, bottom, members }) => {
    const bounds = { x, y, w: right - x, h: bottom - y, width, height };
    const shapes = members.sort((a, b) => a.index - b.index).map(member => member.shape);
    return { key: JSON.stringify([material, revision, x, y, bounds.w, bounds.h, shapes]), bounds };
  });
}
