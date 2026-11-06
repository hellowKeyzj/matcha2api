export interface ConstellationData {
  count: number
  vertices: Float32Array
  indices: Uint16Array
  shapes: Float32Array[]
  colors: Float32Array[]
  scales: Float32Array[]
}

export function decodeConstellation(buffer: ArrayBuffer): ConstellationData {
  const header = new DataView(buffer)
  if (header.getUint32(0, true) !== 0x31534e43) throw new Error('Invalid constellation geometry')
  const count = header.getUint32(4, true)
  const shapeCount = header.getUint32(8, true)
  const vertexCount = header.getUint32(12, true)
  const indexCount = header.getUint32(16, true)
  if (buffer.byteLength !== 20 + vertexCount * 12 + indexCount * 2 + shapeCount * count * 10) {
    throw new Error('Incomplete constellation geometry')
  }
  let offset = 20
  const vertices = new Float32Array(buffer, offset, vertexCount * 3)
  offset += vertices.byteLength
  const indices = new Uint16Array(buffer, offset, indexCount)
  offset += indices.byteLength
  const shapes: Float32Array[] = []
  const colors: Float32Array[] = []
  const scales: Float32Array[] = []
  for (let shape = 0; shape < shapeCount; shape++) {
    const positions = new Uint16Array(buffer, offset, count * 3)
    offset += positions.byteLength
    shapes.push(Float32Array.from(positions, value => value / 65535 * 2 - 1))
    const rgb = new Uint8Array(buffer, offset, count * 3)
    offset += rgb.byteLength
    colors.push(Float32Array.from(rgb, value => value / 255))
    scales.push(Float32Array.from(new Uint8Array(buffer, offset, count), value => value / 255))
    offset += count
  }
  return { count, vertices, indices, shapes, colors, scales }
}

const clamp = (value: number) => Math.max(0, Math.min(1, value))
const ramp = (value: number, start: number, end: number) => clamp((value - start) / (end - start))
const smooth = (value: number) => value * value * (3 - 2 * value)

export function constellationPose(progress: number | undefined, time: number, mobile: boolean, reduced: boolean) {
  // Scroll owns the narrative. Standalone backgrounds use the same poses as a loop.
  const p = progress === undefined ? (time % 36) / 8 : Math.min(progress, 3)
  const scatter = smooth(ramp(p, 1.1, 2))
  const gather = smooth(ramp(p, 2.55, 3))
  const next = smooth(ramp(p, 3.35, 4))
  const reset = smooth(ramp(p, 4, 4.5))
  return {
    morph: (gather + next) * (1 - reset),
    explode: (scatter - gather + next) * (1 - reset),
    rotationY: (-Math.PI / 2 * smooth(ramp(p, 0, 1)) + Math.PI / 2 * gather + Math.PI / 4 * next) * (1 - reset),
    rotationZ: (-0.489 * gather + 0.6 * next) * (1 - reset),
    offsetX: progress === undefined || mobile ? 0 : 3 - 6.5 * smooth(ramp(p, 0, 1)) + 3.5 * smooth(ramp(p, 1.25, 1.7)) + 3 * gather,
    offsetY: mobile || progress === undefined ? 0 : -0.9 + 0.4 * gather,
    factor: mobile ? 2.5 : 4.35,
    entrance: reduced ? 1 : smooth(clamp(time / 3)),
  }
}

export type ConstellationPose = ReturnType<typeof constellationPose>
export interface ConstellationRenderer {
  resize(width: number, height: number, ratio: number, centerX?: number): void
  draw(time: number, pose: ConstellationPose, light: boolean, pointer: { x: number; y: number }): void
  dispose(): void
}

const vertexShader = `#version 300 es
precision highp float;
layout(location=0) in vec3 vertex;
layout(location=1) in vec3 brain;
layout(location=2) in vec3 bulb;
layout(location=3) in vec3 field;
layout(location=4) in vec3 brainColor;
layout(location=5) in vec3 bulbColor;
layout(location=6) in vec3 fieldColor;
layout(location=7) in vec3 sizes;
layout(location=8) in vec3 randoms;
layout(location=9) in vec3 orders;
uniform float time;
uniform vec2 resolution;
uniform vec2 pointer;
uniform vec4 pose;
uniform vec4 placement;
uniform float light;
uniform float volume;
uniform float pixelHeight;
uniform float centerX;
out vec4 color;
flat out float bodyPass;

mat3 rotateY(float angle) {
  float c = cos(angle), s = sin(angle);
  return mat3(c, 0., -s, 0., 1., 0., s, 0., c);
}
mat3 rotateZ(float angle) {
  float c = cos(angle), s = sin(angle);
  return mat3(c, s, 0., -s, c, 0., 0., 0., 1.);
}
void main() {
  float morph = pose.x;
  float explode = pose.y;
  float first = clamp(morph * 6. - orders.y * 5., 0., 1.);
  float second = clamp(max(morph - 1., 0.) * 6. - orders.z * 5., 0., 1.);
  vec3 target = mix(mix(brain, bulb, first), field, second);
  vec3 rgb = mix(mix(brainColor, bulbColor, first), fieldColor, second);
  float size = mix(mix(sizes.x, sizes.y, first), sizes.z, second);
  float reveal = smoothstep(0., 1., placement.w * 1.5 - orders.x * .5);
  float radius = placement.z + (1. - reveal) * (randoms.x * 6. - 3.);
  target *= radius;
  float scatter = clamp(explode * 2.5 - (1. - orders.x) * 1.5, 0., 1.);
  target *= 1. + scatter * randoms.y * 5.;
  bool ambient = gl_InstanceID % 40 == 0;
  if (ambient) {
    target = vec3(field.xy * resolution * .55, randoms.z * 7. - 1.);
    target.xy += vec2(sin(time * .08 + randoms.x * 6.28), cos(time * .09 + randoms.y * 6.28)) * .12;
    size = .2 + randoms.x * .35;
  }
  vec2 scenePointer = pointer - vec2(centerX * 2. - 1., 0.);
  mat3 rotation = rotateZ(pose.w) * rotateY(pose.z + scenePointer.x * .06);
  target = rotation * target + vec3(placement.xy, 0.);
  vec2 mouse = scenePointer * resolution * .5;
  float influence = (1. - smoothstep(0., 1.7, distance(target.xy, mouse))) * (1. - explode);
  target.xy += vec2(sin(time * randoms.y), cos(time * randoms.y)) * influence * .22;
  target.y += sin(time * .25 + randoms.x * 6.28) * .018;
  float spin = time * .28 + randoms.x * 6.28;
  mat3 tumble = rotateY(spin) * rotateZ(spin * .7);
  vec3 position = target + tumble * vertex * .155 * (size + influence * .18);
  bodyPass = volume > 0. ? 1. : 0.;
  if (bodyPass > .5) position = target;
  float depth = max(1., 10. - position.z);
  float focal = 2.145;
  gl_Position = vec4(position.x * focal / (resolution.x / resolution.y), position.y * focal,
    depth * 1.0067 - .2007, depth);
  // Compose the artwork off-center without clipping the full-page viewport.
  gl_Position.x += (centerX * 2. - 1.) * depth;
  float alpha = smoothstep(-4.5, 4., target.z) * reveal * (ambient ? .28 : .68);
  vec3 darkColor = rgb * 1.3;
  vec3 lightColor = rgb * .62;
  color = vec4(mix(darkColor, lightColor, light), alpha * mix(1., .85, light));
  if (bodyPass > .5) {
    // Soft, overlapping surfels fill the real silhouette and follow its folds.
    // Reusing particle transforms keeps the body attached throughout the morph.
    float front = smoothstep(-placement.z, placement.z, target.z);
    float highlight = clamp((target.y - placement.y) / placement.z * .2 + front * .7, 0., 1.);
    vec3 body = mix(vec3(.56, .53, .48), vec3(.961, .949, .925), highlight);
    gl_PointSize = clamp(.22 * focal * pixelHeight / depth, 2., 72.);
    color = vec4(body, (ambient ? 0. : .12) * reveal * volume);
  }
}`

const fragmentShader = `#version 300 es
precision mediump float;
in vec4 color;
flat in float bodyPass;
out vec4 outputColor;
void main() {
  float opacity = 1.;
  if (bodyPass > .5) {
    float radius = length(gl_PointCoord * 2. - 1.);
    if (radius > 1.) discard;
    opacity = exp(-radius * radius * 4.) * (1. - smoothstep(.7, 1., radius));
  }
  outputColor = vec4(color.rgb, color.a * opacity);
}
`

export function createConstellationRenderer(canvas: HTMLCanvasElement, data: ConstellationData): ConstellationRenderer | null {
  const gl = canvas.getContext('webgl2', { alpha: true, antialias: true, powerPreference: 'low-power', depth: false })
  if (!gl) return createCanvasRenderer(canvas, data)
  const shaders: WebGLShader[] = []
  const buffers: WebGLBuffer[] = []
  const program = gl.createProgram()!
  for (const [type, source] of [[gl.VERTEX_SHADER, vertexShader], [gl.FRAGMENT_SHADER, fragmentShader]] as const) {
    const shader = gl.createShader(type)!
    shaders.push(shader)
    gl.shaderSource(shader, source)
    gl.compileShader(shader)
    gl.attachShader(program, shader)
  }
  gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    const message = gl.getProgramInfoLog(program)
    shaders.forEach(shader => gl.deleteShader(shader))
    gl.deleteProgram(program)
    throw new Error(`Constellation shader: ${message}`)
  }
  const vao = gl.createVertexArray()!
  gl.bindVertexArray(vao)
  const attribute = (location: number, values: Float32Array, divisor = 1) => {
    const buffer = gl.createBuffer()!
    buffers.push(buffer)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, values, gl.STATIC_DRAW)
    gl.enableVertexAttribArray(location)
    gl.vertexAttribPointer(location, 3, gl.FLOAT, false, 0, 0)
    gl.vertexAttribDivisor(location, divisor)
  }
  attribute(0, data.vertices, 0)
  data.shapes.forEach((shape, index) => attribute(1 + index, shape))
  data.colors.forEach((colors, index) => attribute(4 + index, colors))
  const sizes = new Float32Array(data.count * 3)
  const randoms = new Float32Array(data.count * 3)
  let seed = 37
  const random = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296 }
  for (let i = 0; i < data.count; i++) {
    for (let shape = 0; shape < 3; shape++) sizes[i * 3 + shape] = data.scales[shape][i]
    randoms[i * 3] = random()
    randoms[i * 3 + 1] = .2 + random() * .8
    randoms[i * 3 + 2] = random()
  }
  attribute(7, sizes)
  attribute(8, randoms)
  const orders = new Float32Array(data.count * 3)
  for (let axis = 0; axis < 3; axis++) {
    const shape = data.shapes[axis]
    const sorted = Array.from({ length: data.count }, (_, index) => index)
      .sort((a, b) => axis === 0 ? shape[b * 3 + 1] - shape[a * 3 + 1] : shape[a * 3] - shape[b * 3])
    sorted.forEach((index, order) => { orders[index * 3 + axis] = order / data.count })
  }
  attribute(9, orders)
  const indexBuffer = gl.createBuffer()!
  buffers.push(indexBuffer)
  gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, indexBuffer)
  gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, data.indices, gl.STATIC_DRAW)
  const uniforms = Object.fromEntries(['time', 'resolution', 'pointer', 'pose', 'placement', 'light', 'volume', 'pixelHeight', 'centerX'].map(name => [name, gl.getUniformLocation(program, name)]))
  gl.enable(gl.BLEND)
  gl.blendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
  let width = 0
  let height = 0
  let center = .5
  return {
    resize(w, h, ratio, centerX = .5) {
      width = w
      height = h
      center = centerX
      canvas.width = Math.round(w * ratio)
      canvas.height = Math.round(h * ratio)
      gl.viewport(0, 0, canvas.width, canvas.height)
    },
    draw(time, pose, light, pointer) {
      gl.clear(gl.COLOR_BUFFER_BIT)
      gl.useProgram(program)
      gl.bindVertexArray(vao)
      gl.uniform1f(uniforms.time, time)
      gl.uniform1f(uniforms.centerX, center)
      // World-space span at the reference camera's z=10, focal length=35mm.
      const spanY = 20 / 2.145
      gl.uniform2f(uniforms.resolution, spanY * width / height, spanY)
      gl.uniform2f(uniforms.pointer, pointer.x, pointer.y)
      gl.uniform4f(uniforms.pose, pose.morph, pose.explode, pose.rotationY, pose.rotationZ)
      gl.uniform4f(uniforms.placement, pose.offsetX, pose.offsetY, pose.factor, pose.entrance)
      gl.uniform1f(uniforms.light, light ? 1 : 0)
      const count = width < 600 ? 7000 : data.count
      const volume = light ? (1 - clamp(pose.explode)) ** 2 : 0
      if (volume > 0) {
        gl.uniform1f(uniforms.pixelHeight, canvas.height)
        gl.uniform1f(uniforms.volume, volume)
        gl.drawArraysInstanced(gl.POINTS, 0, 1, count)
      }
      gl.uniform1f(uniforms.volume, 0)
      gl.drawElementsInstanced(gl.TRIANGLES, data.indices.length, gl.UNSIGNED_SHORT, 0, count)
    },
    dispose() {
      buffers.forEach(buffer => gl.deleteBuffer(buffer))
      shaders.forEach(shader => gl.deleteShader(shader))
      gl.deleteVertexArray(vao)
      gl.deleteProgram(program)
    },
  }
}

// WebGL can be disabled by browser policy. Use the same silhouettes, not a blank hero.
function createCanvasRenderer(canvas: HTMLCanvasElement, data: ConstellationData): ConstellationRenderer | null {
  const ctx = canvas.getContext('2d')
  if (!ctx) return null
  let width = 0
  let height = 0
  let center = .5
  return {
    resize(w, h, ratio, centerX = .5) {
      width = w
      height = h
      center = centerX
      canvas.width = Math.round(w * ratio)
      canvas.height = Math.round(h * ratio)
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0)
    },
    draw(time, pose, light, pointer) {
      ctx.clearRect(0, 0, width, height)
      const a = Math.min(1, pose.morph)
      const b = Math.max(0, pose.morph - 1)
      const cos = Math.cos(pose.rotationY + (pointer.x - (center * 2 - 1)) * .06)
      const sin = Math.sin(pose.rotationY + (pointer.x - (center * 2 - 1)) * .06)
      const cosZ = Math.cos(pose.rotationZ)
      const sinZ = Math.sin(pose.rotationZ)
      const scale = height * .10725
      ctx.lineWidth = .65
      for (let i = 0; i < data.count; i += width < 600 ? 4 : 2) {
        const point = [0, 0, 0]
        const color = [0, 0, 0]
        for (let axis = 0; axis < 3; axis++) {
          const index = i * 3 + axis
          point[axis] = ((data.shapes[0][index] * (1 - a) + data.shapes[1][index] * a) * (1 - b) + data.shapes[2][index] * b) * pose.factor
          color[axis] = ((data.colors[0][index] * (1 - a) + data.colors[1][index] * a) * (1 - b) + data.colors[2][index] * b) * (light ? 158 : 255)
          point[axis] = point[axis] * (1 - pose.explode) + data.shapes[2][index] * (axis === 0 ? width / scale * .65 : height / scale * .65) * pose.explode
        }
        const depth = point[2] * cos - point[0] * sin
        const perspective = 10 / Math.max(1, 10 - depth)
        const rotatedX = point[0] * cos + point[2] * sin
        const x = width * center + (rotatedX * cosZ - point[1] * sinZ + pose.offsetX) * scale * perspective
        const y = height / 2 - (rotatedX * sinZ + point[1] * cosZ + pose.offsetY) * scale * perspective
        const size = (.7 + data.scales[0][i] * 3) * perspective
        ctx.strokeStyle = `rgb(${color.join(',')})`
        ctx.globalAlpha = (.25 + (depth + pose.factor) / pose.factor * .25) * pose.entrance
        ctx.beginPath()
        for (let corner = 0; corner < 3; corner++) {
          const angle = time * .28 + i + corner * Math.PI * 2 / 3
          const px = x + Math.cos(angle) * size
          const py = y + Math.sin(angle) * size
          if (corner === 0) ctx.moveTo(px, py)
          else ctx.lineTo(px, py)
        }
        ctx.closePath()
        ctx.stroke()
      }
      ctx.globalAlpha = 1
    },
    dispose() { ctx.clearRect(0, 0, width, height) },
  }
}
