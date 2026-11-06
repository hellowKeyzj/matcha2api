import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createConstellationRenderer, decodeConstellation, constellationPose } from '../constellationScene'

afterEach(() => vi.restoreAllMocks())

function sourceData() {
  const file = readFileSync(resolve('src/assets/constellation/constellation.bin'))
  return decodeConstellation(file.buffer.slice(file.byteOffset, file.byteOffset + file.byteLength))
}

function webgl() {
  const gl = {
    TRIANGLES: 4, POINTS: 0, UNSIGNED_SHORT: 5123,
    getProgramParameter: vi.fn(() => true),
    getUniformLocation: vi.fn((_program: unknown, name: string) => name),
    ...Object.fromEntries([
      'createProgram', 'createShader', 'shaderSource', 'compileShader', 'attachShader',
      'linkProgram', 'createVertexArray', 'bindVertexArray', 'createBuffer', 'bindBuffer',
      'bufferData', 'enableVertexAttribArray', 'vertexAttribPointer', 'vertexAttribDivisor',
      'enable', 'blendFuncSeparate', 'viewport', 'clear', 'useProgram', 'uniform1f',
      'uniform2f', 'uniform4f', 'drawElementsInstanced', 'drawArraysInstanced',
      'deleteShader', 'deleteBuffer', 'deleteVertexArray', 'deleteProgram',
    ].map(name => [name, vi.fn(() => ({}))])),
  }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(gl as unknown as WebGL2RenderingContext)
  return gl
}

function canvas2d() {
  const ctx = {
    setTransform: vi.fn(), clearRect: vi.fn(), beginPath: vi.fn(),
    moveTo: vi.fn(), lineTo: vi.fn(), closePath: vi.fn(), stroke: vi.fn(),
    globalAlpha: 1, lineWidth: 1, strokeStyle: '',
  }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(type =>
    type === '2d' ? ctx as unknown as CanvasRenderingContext2D : null)
  return ctx
}

describe('constellation scene', () => {
  it('uses the three source silhouettes, not an approximation of a sphere', () => {
    const data = sourceData()
    expect(data.count).toBe(10000)
    expect(data.shapes).toHaveLength(3)
    expect(data.vertices).toHaveLength(60)
    expect(data.indices).toHaveLength(144)
    expect(data.shapes[0]).not.toEqual(data.shapes[1])
    expect(data.shapes[1]).not.toEqual(data.shapes[2])
    expect([...data.vertices].every(Number.isFinite)).toBe(true)
  })

  it('keeps scroll-driven brain, scattered field and lightbulb poses continuous', () => {
    const start = constellationPose(0, 0, false, true)
    const brain = constellationPose(1, 0, false, true)
    const field = constellationPose(2, 0, false, true)
    const bulb = constellationPose(3, 0, false, true)
    expect(start.offsetX).toBeGreaterThan(0)
    expect(brain.offsetX).toBeLessThan(0)
    expect(field.explode).toBe(1)
    expect(bulb.morph).toBe(1)
    expect(bulb.explode).toBe(0)
    for (const boundary of [1, 2, 3]) {
      const a = constellationPose(boundary - 0.00001, 0, false, true)
      const b = constellationPose(boundary, 0, false, true)
      expect(Math.abs(a.offsetX - b.offsetX)).toBeLessThan(0.001)
      expect(Math.abs(a.morph - b.morph)).toBeLessThan(0.001)
      expect(Math.abs(a.explode - b.explode)).toBeLessThan(0.001)
    }
  })

  it('cycles the same shapes when there is no scroll narrative', () => {
    const poses = [0, 16, 24, 32].map(time => constellationPose(undefined, time, false, false))
    expect(poses.some(pose => pose.explode > 0.9)).toBe(true)
    expect(poses.some(pose => pose.morph === 1)).toBe(true)
    expect(poses.every(pose => pose.offsetX === 0)).toBe(true)
    expect(constellationPose(undefined, 0, true, true).factor).toBeLessThan(constellationPose(undefined, 0, false, true).factor)
  })

  it('adds the soft body only in light mode without changing the dark particle draw', () => {
    const gl = webgl()
    const renderer = createConstellationRenderer(document.createElement('canvas'), sourceData())!
    renderer.resize(1280, 720, 1.5)
    const pose = constellationPose(0, 4, false, false)
    renderer.draw(4, pose, false, { x: 0, y: 0 })
    expect(gl.drawArraysInstanced).not.toHaveBeenCalled()
    expect(gl.drawElementsInstanced).toHaveBeenLastCalledWith(gl.TRIANGLES, 144, gl.UNSIGNED_SHORT, 0, 10000)

    renderer.draw(4, pose, true, { x: 0, y: 0 })
    expect(gl.drawArraysInstanced).toHaveBeenCalledWith(gl.POINTS, 0, 1, 10000)
    expect(gl.uniform1f).toHaveBeenCalledWith('volume', 1)
    expect(gl.uniform1f).toHaveBeenCalledWith('pixelHeight', 1080)
    expect(gl.uniform1f).toHaveBeenLastCalledWith('volume', 0)
    expect(gl.drawArraysInstanced.mock.invocationCallOrder[0]).toBeLessThan(gl.drawElementsInstanced.mock.invocationCallOrder[1])
    renderer.dispose()
  })

  it('keeps the render viewport full width while aligning the scene in screen space', () => {
    const gl = webgl()
    const canvas = document.createElement('canvas')
    const renderer = createConstellationRenderer(canvas, sourceData())!
    renderer.resize(1169, 987, 1, .34)
    const pose = constellationPose(undefined, 4, false, false)
    renderer.draw(4, pose, true, { x: 0, y: 0 })
    expect(canvas.width).toBe(1169)
    expect(gl.viewport).toHaveBeenLastCalledWith(0, 0, 1169, 987)
    expect(gl.uniform1f).toHaveBeenCalledWith('centerX', .34)
    renderer.resize(390, 844, 1)
    renderer.draw(4, pose, false, { x: 0, y: 0 })
    expect(gl.uniform1f).toHaveBeenCalledWith('centerX', .5)
    renderer.dispose()
  })

  it('dissolves the light body during scatter and restores it for the lightbulb', () => {
    const gl = webgl()
    const renderer = createConstellationRenderer(document.createElement('canvas'), sourceData())!
    renderer.resize(390, 550, 1)
    const pose = constellationPose(0, 4, true, false)
    renderer.draw(4, { ...pose, explode: .5 }, true, { x: 0, y: 0 })
    expect(gl.uniform1f).toHaveBeenCalledWith('volume', .25)
    expect(gl.drawArraysInstanced).toHaveBeenCalledWith(gl.POINTS, 0, 1, 7000)
    gl.drawArraysInstanced.mockClear()
    renderer.draw(4, constellationPose(2, 4, true, false), true, { x: 0, y: 0 })
    expect(gl.drawArraysInstanced).not.toHaveBeenCalled()
    renderer.draw(4, constellationPose(3, 4, true, false), true, { x: 0, y: 0 })
    expect(gl.drawArraysInstanced).toHaveBeenCalledOnce()
    expect(gl.uniform1f).toHaveBeenCalledWith('volume', 1)
    renderer.dispose()
  })

  it('falls back to canvas without losing the silhouette rotation or alignment', () => {
    const ctx = canvas2d()
    const data = {
      count: 1,
      vertices: new Float32Array(), indices: new Uint16Array(),
      shapes: Array.from({ length: 3 }, () => new Float32Array([1, 0, 0])),
      colors: Array.from({ length: 3 }, () => new Float32Array([1, .5, 0])),
      scales: Array.from({ length: 3 }, () => new Float32Array([0])),
    }
    const canvas = document.createElement('canvas')
    const renderer = createConstellationRenderer(canvas, data)!
    renderer.resize(1000, 500, 1.5, .34)
    const pose = { morph: 0, explode: 0, rotationY: 0, rotationZ: Math.PI / 2, offsetX: 0, offsetY: 0, factor: 1, entrance: 1 }

    renderer.draw(0, pose, false, { x: -.32, y: 0 })

    expect(canvas.width).toBe(1500)
    expect(ctx.setTransform).toHaveBeenCalledWith(1.5, 0, 0, 1.5, 0, 0)
    expect(ctx.moveTo).toHaveBeenCalledWith(expect.closeTo(340.7), expect.closeTo(250 - 500 * .10725))
    expect(ctx.stroke).toHaveBeenCalledOnce()
    expect(ctx.globalAlpha).toBe(1)
    renderer.dispose()
    expect(ctx.clearRect).toHaveBeenLastCalledWith(0, 0, 1000, 500)
  })

  it('leaves the page usable when neither graphics context is available', () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
    expect(createConstellationRenderer(document.createElement('canvas'), sourceData())).toBeNull()
  })

  it('releases GPU resources when shader linking fails', () => {
    const gl = webgl()
    gl.getProgramParameter.mockReturnValue(false)
    Object.assign(gl, { getProgramInfoLog: vi.fn(() => 'Link failed') })
    expect(() => createConstellationRenderer(document.createElement('canvas'), sourceData())).toThrow('Link failed')
    expect(gl.deleteShader).toHaveBeenCalledTimes(2)
    expect(gl.deleteProgram).toHaveBeenCalledOnce()
    expect(gl.createBuffer).not.toHaveBeenCalled()
  })
})
