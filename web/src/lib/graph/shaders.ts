/**
 * Node and edge shaders for the ported graph engine.
 *
 * Ported faithfully from `context/refs/react/graph-react/src/KnowledgeGraph.jsx`
 * (NODE_VS/FS 149-193, EDGE_VS/FS 195-218), which the project owns. Strings are
 * verbatim; do not re-tune the glow/rim/arrow math without a visual review.
 */

export const NODE_VS = /* glsl */ `
attribute float aSize;
attribute vec3 aColor;
attribute float aAlpha;
attribute float aBoost;
uniform float uScale;
uniform float uHalo;
uniform float uFocus;
uniform float uDepthFade;
varying vec3 vColor;
varying float vAlpha;
varying float vBoost;
void main() {
  vec4 mv = modelViewMatrix * vec4(position, 1.0);
  gl_Position = projectionMatrix * mv;
  float d = max(-mv.z, 0.001);
  gl_PointSize = clamp(aSize * uScale / d * uHalo, 7.0, 600.0);
  float fade = 1.0 - uDepthFade * 0.6 * smoothstep(uFocus * 0.9, uFocus * 1.9, d);
  vColor = aColor;
  vAlpha = aAlpha * fade;
  vBoost = aBoost;
}`;

export const NODE_FS = /* glsl */ `
precision highp float;
uniform float uGlow;
uniform float uHalo;
varying vec3 vColor;
varying float vAlpha;
varying float vBoost;
void main() {
  vec2 p = gl_PointCoord * 2.0 - 1.0;
  float r = length(p);
  if (r > 1.0) discard;
  float coreR = 1.0 / uHalo;
  float core = 1.0 - smoothstep(coreR - 0.035, coreR, r);
  float t = clamp((r - coreR * 0.5) / (1.0 - coreR * 0.5), 0.0, 1.0);
  float halo = pow(1.0 - t, 2.6) * uGlow * (0.6 + 0.9 * vBoost);
  float ring = vBoost * (1.0 - smoothstep(0.0, 0.03, abs(r - coreR * 1.5)));
  vec3 rim = mix(vColor, vec3(1.0), 0.30 * (1.0 - clamp(r / coreR, 0.0, 1.0)));
  vec3 col = mix(vColor, rim, core);
  col = mix(col, vec3(1.0), ring * 0.7);
  float a = max(core, max(halo, ring * 0.85));
  gl_FragColor = vec4(col, a * vAlpha);
}`;

export const EDGE_VS = /* glsl */ `
attribute vec3 aColor;
attribute float aAlpha;
uniform float uFocus;
uniform float uDepthFade;
varying vec3 vColor;
varying float vAlpha;
void main() {
  vec4 mv = modelViewMatrix * vec4(position, 1.0);
  gl_Position = projectionMatrix * mv;
  float d = max(-mv.z, 0.001);
  float fade = 1.0 - uDepthFade * 0.6 * smoothstep(uFocus * 0.9, uFocus * 1.9, d);
  vColor = aColor;
  vAlpha = aAlpha * fade;
}`;

export const EDGE_FS = /* glsl */ `
precision highp float;
uniform float uOpacity;
varying vec3 vColor;
varying float vAlpha;
void main() {
  gl_FragColor = vec4(vColor, vAlpha * uOpacity);
}`;
