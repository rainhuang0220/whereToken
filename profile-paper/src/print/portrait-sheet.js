import { projectSnapshot } from './profile-data.js';

const INK = '#1b1e19';
const FAINT = '#51554c';
const RED = '#873e31';
const W = 1200;
const H = 1800;

function label(ctx, text, x, y, size = 17, color = FAINT, weight = 500, align = 'left') {
  ctx.textAlign = align;
  ctx.fillStyle = color;
  ctx.font = `${weight} ${size}px -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", sans-serif`;
  ctx.fillText(String(text), x, y);
  ctx.textAlign = 'left';
}

function serif(ctx, text, x, y, size, weight = 650, color = INK) {
  ctx.font = `${weight} ${size}px Georgia, "Times New Roman", "Songti SC", serif`;
  ctx.fillStyle = color;
  ctx.fillText(String(text), x, y);
}

function rule(ctx, y, thick = 1.2, color = INK) {
  ctx.strokeStyle = color;
  ctx.lineWidth = thick;
  ctx.beginPath();
  ctx.moveTo(73, y);
  ctx.lineTo(1127, y);
  ctx.stroke();
}

function hatch(ctx, x, y, size) {
  ctx.strokeStyle = 'rgba(25,27,22,.38)';
  ctx.lineWidth = .7;
  for (let t = -size; t < size * 2; t += 6) {
    ctx.beginPath();
    ctx.moveTo(x + Math.max(0, t), y + Math.min(size, size - t));
    ctx.lineTo(x + Math.min(size, t + size), y + Math.max(0, -t));
    ctx.stroke();
  }
}

export function createPortraitPrint(snapshot, language = 'zh', selection = {}) {
  const data = projectSnapshot(snapshot, selection);
  const zh = language === 'zh';
  const canvas = document.createElement('canvas');
  canvas.width = W; canvas.height = H;
  const c = canvas.getContext('2d', { alpha: false });
  c.fillStyle = '#fff'; c.fillRect(0, 0, W, H);
  c.textBaseline = 'alphabetic';

  label(c, zh ? '个人算力日志  /  编程智能体用量报告' : 'PERSONAL COMPUTE LEDGER / CODING AGENT REPORT', 73, 69, 15, INK, 650);
  label(c, (data.provenance === 'synthetic_demo' ? 'SYNTHETIC DEMO' : 'PUBLIC SNAPSHOT') + ' · ' + data.asOf, 1127, 69, 13, FAINT, 550, 'right');
  rule(c, 87, 1.5);
  serif(c, 'whereToken', 70, 206, 111, 700);
  label(c, zh ? '时间、用量与工作的印刷记录' : 'A PRINTED RECORD OF WORK, TIME & USAGE', 78, 244, 17, FAINT, 550);
  rule(c, 270, 2.5);

  const metricName = data.metric === 'requests' ? (zh ? '请求总数' : 'REQUESTS') : (zh ? 'Token 用量' : 'TOKENS');
  const rangeName = {all:'ALL',today:'TODAY','7d':'7 DAYS','30d':'30 DAYS','53w':'53 WEEKS'}[data.range] || 'ALL';
  label(c, `${metricName} · ${rangeName}${data.selection ? ' · ' + data.selection.slice(0,24) : ''}`, 74, 323, 17, INK, 650);
  serif(c, data.total.display, 70, 430, 103, 600);
  label(c, data.metric === 'requests' ? 'REQUESTS' : 'TOKENS', 1057, 429, 17, FAINT, 650, 'right');
  label(c, `${data.requests.display} ${zh ? '请求' : 'requests'}`, 76, 487, 21, INK);
  label(c, `${data.activeDays.display} ${zh ? '活跃天数' : 'active days'}`, 450, 487, 21, INK);
  label(c, `${data.streak.display} ${zh ? '当前连续天数' : 'day streak'}`, 850, 487, 21, INK);
  rule(c, 526);

  label(c, '01', 77, 589, 19, RED, 700);
  serif(c, zh ? '年度活动' : 'Activity', 130, 591, 39);
  label(c, zh ? '53 周 · 每格代表一天 · 最新日期在右侧' : '53 weeks · one day per cell · latest on the right', 76, 631, 16);

  const step = 18.6, square = 13.6, sx = 105, sy = 714;
  const dates = data.activity.dates;
  for (let col = 0; col < 53; col++) {
    const date = dates[col * 7];
    const prev = dates[(col - 1) * 7];
    if (date && (!prev || date.slice(0, 7) !== prev.slice(0, 7))) {
      label(c, date.slice(5, 7), sx + col * step, 681, 12, FAINT, 650);
    }
  }
  for (let row = 1; row <= 5; row += 2) label(c, String(row), 76, sy + row * step + 11, 12);
  for (let i = 0; i < dates.length; i++) {
    const col = Math.floor(i / 7), row = i % 7;
    const x = sx + col * step, y = sy + row * step;
    const status = data.activity.states[i];
    const intensity = data.activity.intensities[i];
    let channel = Math.round(220 - Math.pow(intensity, .76) * 188);
    if (status === 'unknown') channel = 170;
    if (status === 'future') channel = 238;
    c.fillStyle = `rgb(${channel},${channel},${Math.max(0, channel - 2)})`;
    c.fillRect(x, y, square, square);
    if (status === 'unknown') hatch(c, x, y, square);
  }
  rule(c, 878);
  label(c, data.metric === 'requests' ? 'REQUEST ACTIVITY / 53 WEEKS' : 'TOKEN ACTIVITY / 53 WEEKS', 76, 916, 15, INK, 650);
  const vals = data.activity.values;
  const max = Math.max(1, ...vals.filter((v, i) => v != null && data.activity.states[i] !== 'unknown' && data.activity.states[i] !== 'future'));
  c.strokeStyle = INK; c.lineWidth = 2.5; c.beginPath();
  let started = false;
  vals.forEach((v, i) => {
    if (v == null || data.activity.states[i] === 'unknown' || data.activity.states[i] === 'future') { started = false; return; }
    const x = 78 + i / Math.max(1, vals.length - 1) * 1044;
    const y = 1070 - (v / max) * 112;
    if (!started) { c.moveTo(x, y); started = true; } else c.lineTo(x, y);
  });
  c.stroke();
  rule(c, 1116);

  label(c, '02', 77, 1181, 19, RED, 700);
  serif(c, zh ? '来源构成' : 'Breakdown', 130, 1183, 39);
  label(c, zh ? '按编程智能体归类' : 'Usage by coding agent', 78, 1218, 16);
  const rows = data.breakdown.slice(0, 6);
  const maximum = Math.max(1, ...rows.map(r => Number(r.total.value) || 0));
  rows.forEach((row, i) => {
    const y = 1272 + i * 69;
    label(c, String(i + 1).padStart(2, '0'), 76, y, 14, RED, 650);
    serif(c, row.label, 128, y, 29, 550);
    label(c, row.total.display, 1123, y, 21, INK, 650, 'right');
    c.fillStyle = '#d2d1c8'; c.fillRect(129, y + 11, 992, 4);
    c.fillStyle = i === 0 ? RED : INK;
    c.fillRect(129, y + 11, Math.max(2, 992 * (Number(row.total.value) || 0) / maximum), 4);
  });
  rule(c, 1718, 2);
  const provenance = data.provenance === 'synthetic_demo'
    ? (zh ? '合成演示数据 · 非真实账户数据' : 'SYNTHETIC DEMO · NOT LIVE ACCOUNT DATA')
    : (zh ? '公开脱敏数据 · 非实时同步' : 'SANITIZED PUBLIC DATA · NOT A LIVE FEED');
  label(c, provenance, 77, 1750, 13, INK, 650);
  label(c, 'PAPER / PIGMENT / REAL MESH', 1125, 1750, 12, FAINT, 650, 'right');
  return canvas;
}
