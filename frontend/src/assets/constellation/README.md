# Constellation geometry

Reference: https://dala.craftedbygc.com/ (the animation linked by Refero).

`constellation.bin` contains the reference's brain, lightbulb and dispersed-field
particle positions, per-particle colors/scales, and small hollow pyramid mesh.
It is data only: no third-party scripts, tracking, fonts, text or logo are bundled.

Source resources: `/images/pos-33.exr`, `/images/cd-33.png`,
`/images/sc-33.png`, `/models/py-lod7.glb`. The EXR's three 100x100 quadrants
are converted from PXR24 RGB into unsigned 16-bit coordinates. The fourth
quadrant (the Dala logo) is deliberately excluded.

Binary layout, little endian:

- 20-byte header: `CNS1`, particle count, shape count, vertex count, index count.
- Mesh positions: float32 XYZ; triangle indices: uint16.
- For each shape: uint16 XYZ, uint8 RGB, uint8 scale, in particle order.

The renderer and lifecycle are implemented locally. Reference-specific brand
artwork remains attributable to its original creator; public availability does
not imply an open-source license.

No license granting redistribution of this extracted artwork has been verified
in this repository. This file records provenance, not permission. Confirm the
required rights or replace the geometry with original or licensed artwork
before redistribution.
