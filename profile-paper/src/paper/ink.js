const clamp01 = (value) => Math.min(1, Math.max(0, value));

export function srgbToLinear(value) {
  return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
}

export function linearToSrgb(value) {
  return value <= 0.0031308 ? value * 12.92 : 1.055 * Math.max(0, value) ** (1 / 2.4) - 0.055;
}

export function heatReflectance(intensity) {
  const value = clamp01(intensity);
  const stops = [[0, 0.86], [0.25, 0.67], [0.6, 0.43], [1, 0.2]];
  let lower = stops[0];
  let upper = stops[stops.length - 1];
  for (let index = 1; index < stops.length; index++) {
    if (value <= stops[index][0]) {
      lower = stops[index - 1];
      upper = stops[index];
      break;
    }
  }
  const amount = (value - lower[0]) / (upper[0] - lower[0] || 1);
  const lightness = lower[1] + (upper[1] - lower[1]) * amount;
  return lightness ** 3;
}

export function inkTransmittance(pigmentLinear, coverage, absorption, mottle = 0) {
  const density = Math.max(0, coverage) * Math.max(0, absorption) * Math.max(0, 1 + mottle);
  return pigmentLinear.map((pigment) => density === 0 ? 1 : Math.exp(Math.log(clamp01(pigment)) * density));
}

export function compositeInk(paperLinear, pigmentLinear, coverage, { absorption, mottle }) {
  if (coverage <= 0 || absorption <= 0) return [...paperLinear];
  const transmission = inkTransmittance(pigmentLinear, coverage, absorption, mottle);
  return paperLinear.map((reflectance, channel) => reflectance * transmission[channel]);
}
