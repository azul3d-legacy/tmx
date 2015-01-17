// Copyright 2014 The Azul3D Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tmx implements routines for rendering tmx maps.
//
// It loads a 2D tmx map into a few meshes stored in a *gfx.Object and applies
// textures such that it would render properly.
//
// At present the package only supports orthogonal tile map rendering, and has
// some issues with proper ordering of perspective (I.e. non-uniformly sized)
// tiles (see for instance tiled-qt/examples/perspective_walls.tmx).
package tmx

import (
	"image"
	"image/draw"
	"io/ioutil"
	"os"
	"path/filepath"

	"azul3d.org/gfx.v2-unstable"
	"azul3d.org/lmath.v1"
)

var (
	glslVert = []byte(`
#version 120

attribute vec3 Vertex;
attribute vec2 TexCoord0;

uniform mat4 MVP;

varying vec2 tc0;

void main()
{
	tc0 = TexCoord0;
	gl_Position = MVP * vec4(Vertex, 1.0);
}
`)

	glslFrag = []byte(`
#version 120

varying vec2 tc0;

uniform sampler2D Texture0;
uniform bool BinaryAlpha;

void main()
{
	gl_FragColor = texture2D(Texture0, tc0);
	if(BinaryAlpha && gl_FragColor.a < 0.5) {
		discard;
	}
}
`)
)

var (
	cw90, cwn90, horizFlip, vertFlip lmath.Mat4
	Shader                           *gfx.Shader
)

func init() {
	Shader = &gfx.Shader{
		Name: "tmx.Shader",
		GLSL: &gfx.GLSLSources{
			Vertex:   glslVert,
			Fragment: glslFrag,
		},
	}

	// Setup rotations
	cw90 = lmath.Mat4FromAxisAngle(
		lmath.Vec3{0, 1, 0},
		lmath.Radians(90),
		lmath.CoordSysZUpRight,
	)

	cwn90 = lmath.Mat4FromAxisAngle(
		lmath.Vec3{0, 1, 0},
		lmath.Radians(-90),
		lmath.CoordSysZUpRight,
	)

	// Setup horizontal flip
	horizFlip = lmath.Mat4FromAxisAngle(
		lmath.Vec3{0, 0, 1},
		lmath.Radians(180),
		lmath.CoordSysZUpRight,
	)

	// Setup vertical flip
	vertFlip = lmath.Mat4FromAxisAngle(
		lmath.Vec3{1, 0, 0},
		lmath.Radians(180),
		lmath.CoordSysZUpRight,
	)
}

func appendCard(m *gfx.Mesh, l, r, b, t, depth float32, rect, tex image.Rectangle) {
	addv := func(x, y float32) {
		m.Vertices = append(m.Vertices, gfx.Vec3{x, depth, y})
	}

	addt := func(u, v float32) {
		if len(m.TexCoords) == 0 {
			m.TexCoords = make([]gfx.TexCoordSet, 1)
		}
		m.TexCoords[0].Slice = append(m.TexCoords[0].Slice, gfx.TexCoord{u, v})
	}
	w := float32(tex.Dx())
	h := float32(tex.Dy())
	halfTexUnitX := 1.0 / w
	halfTexUnitY := 1.0 / w
	u0 := (float32(rect.Min.X) / w) + halfTexUnitX
	u1 := (float32(rect.Max.X) / w) - halfTexUnitX
	v0 := (float32(rect.Min.Y) / h) + halfTexUnitY
	v1 := (float32(rect.Max.Y) / h) - halfTexUnitY

	// Left triangle.
	addv(l, t)
	addv(l, b)
	addv(r, b)

	// Right triangle.
	addv(l, t)
	addv(r, b)
	addv(r, t)

	// Left triangle.
	addt(u0, v0)
	addt(u0, v1)
	addt(u1, v1)

	// Right triangle.
	addt(u0, v0)
	addt(u1, v1)
	addt(u1, v0)
}

// Config represents a tmx mesh configuration
type Config struct {
	// The value which is used to offset each layer on the Y axis.
	LayerOffset float64

	// The value to offset each individual tile from one another on the Y axis.
	TileOffset float64
}

// Load loads the given tmx map, m, and returns a slice of *gfx.Object with the
// proper meshes and textures attached to them.
//
// If the configuration, c, is non-nil then it is used in place of the default
// configuration.
//
// The tsImages map should be a map of tileset image filenames and their
// associated loaded RGBA images. Tiles who reference tilesets who are not
// found in the map will be omited (not rendered) in the returned objects.
func Load(m *Map, c *Config, tsImages map[string]*image.RGBA) (layers map[string]map[string]*gfx.Object) {
	if c == nil {
		c = &Config{
			LayerOffset: 0.001,
			TileOffset:  0.000001,
		}
	}

	// A map of layer names to a slice of objects each containing one texture
	// and mesh.
	layers = make(map[string]map[string]*gfx.Object, len(m.Layers))
	var layerOffset float64

	for _, layer := range m.Layers {
		// A slice of objects which contain a single texture and mesh.
		texObjects := make(map[string]*gfx.Object)
		var tileOffset float64

		for x := 0; x < m.Width; x++ {
			for y := 0; y < m.Height; y++ {
				gid, hasTile := layer.Tiles[Coord{x, y}]
				if !hasTile {
					continue
				}

				tileset := m.FindTileset(gid)

				// Load the tileset texture if needed
				tsImage := filepath.Base(tileset.Image.Source)
				rgba, haveTilesetImage := tsImages[tsImage]
				if !haveTilesetImage {
					// We weren't given a RGBA image for the tileset, so we
					// will just omit this tile.
					continue
				}

				// Create a textured mesh object, if needed.
				obj, ok := texObjects[tsImage]
				if !ok {
					// Create texture.
					t := gfx.NewTexture()
					t.Source = rgba
					t.Bounds = rgba.Bounds()
					t.WrapU = gfx.Clamp
					t.WrapV = gfx.Clamp
					t.MinFilter = gfx.LinearMipmapLinear
					t.MagFilter = gfx.Linear

					// And the object.
					obj = gfx.NewObject()
					obj.Shader = Shader
					obj.Meshes = []*gfx.Mesh{gfx.NewMesh()}
					obj.Textures = []*gfx.Texture{t}

					// Disable face culling because of the flipped cards.
					obj.State = gfx.NewState()
					obj.State.FaceCulling = gfx.NoFaceCulling
					obj.State.AlphaMode = gfx.AlphaToCoverage
					texObjects[tsImage] = obj
				}
				r := m.TilesetRect(tileset, rgba.Bounds().Dx(), rgba.Bounds().Dy(), true, gid)

				halfWidth := float32(tileset.Width) / 2.0
				halfHeight := float32(tileset.Height) / 2.0
				cardStart := len(obj.Meshes[0].Vertices)
				appendCard(
					obj.Meshes[0],
					-halfWidth,
					halfWidth,
					-halfHeight,
					halfHeight,
					0, r, rgba.Bounds(),
				)
				cardEnd := len(obj.Meshes[0].Vertices)

				// apply necessary flips
				flip := lmath.Mat4Identity
				diagFlipped := (gid & FLIPPED_DIAGONALLY_FLAG) > 0
				horizFlipped := (gid & FLIPPED_HORIZONTALLY_FLAG) > 0
				vertFlipped := (gid & FLIPPED_VERTICALLY_FLAG) > 0
				if diagFlipped {
					if horizFlipped && vertFlipped {
						flip = cw90.Mul(flip)
						flip = horizFlip.Mul(flip)
					} else if horizFlipped {
						flip = cw90.Mul(flip)
					} else if vertFlipped {
						flip = cwn90.Mul(flip)
					} else {
						flip = horizFlip.Mul(flip)
						flip = cw90.Mul(flip)
					}
				} else {
					if horizFlipped {
						flip = horizFlip.Mul(flip)
					}
					if vertFlipped {
						flip = vertFlip.Mul(flip)
					}
				}

				// Move the card,
				move := lmath.Mat4FromTranslation(lmath.Vec3{
					float64(x*m.TileWidth) + float64(halfWidth),
					layerOffset + tileOffset,
					float64((m.Height-y)*m.TileHeight) - float64(halfHeight),
				})
				tileOffset -= c.TileOffset

				trans := flip.Mul(move)

				// Apply transformation.
				verts := obj.Meshes[0].Vertices
				for i, v := range verts[cardStart:cardEnd] {
					vt := v.Vec3().TransformMat4(trans)
					verts[cardStart+i] = gfx.Vec3{float32(vt.X), float32(vt.Y), float32(vt.Z)}
				}
			}
		}

		// Add the slice to the map of layers.
		layers[layer.Name] = texObjects

		// Decrease layer offset
		layerOffset -= c.LayerOffset
	}

	/*
			m := new(gfx.Mesh)
			o := &gfx.Object{
				State: gfx.DefaultState,
				Meshes: []*gfx.Mesh{m},
				Textures: texturesSlice,
			}

		for _, layer := range m.Layers {
			// Collect all the little tile meshes
			for tex, tsImageNode := range texturedNodes {
				_, collected := geom.Collect(tsImageNode)
				collected.SetParent(baseNode)
				tsImageNode.Detatch()
				//collected := tsImageNode

				// Add the tileset texture to the collected set of tile meshes
				tex.SetWrapModeU(texture.Clamp)
				tex.SetWrapModeV(texture.Clamp)
				texture.Set(collected, texture.DefaultLayer, tex)

				// Decrease Y by layer offset
				x, _, z := collected.Pos()
				collected.SetPos(x, float64(layerOffset), z)
			}

		}
	*/
	return layers
}

// LoadFile works just like Load except it loads all associated dependencies
// (external tsx tileset files, tileset texture images) for you.
//
// Advanced clients who wish to have more control over file IO will use Load()
// directly instead of using this function.
func LoadFile(path string, c *Config) (*Map, map[string]map[string]*gfx.Object, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}

	m, err := Parse(data)
	if err != nil {
		return nil, nil, err
	}

	relativeDir := filepath.Dir(path)

	// External tilesets in the map must be loaded seperately
	for _, ts := range m.Tilesets {
		if len(ts.Source) > 0 {
			// Open tsx file
			f, err := os.Open(filepath.Join(relativeDir, filepath.Base(ts.Source)))
			if err != nil {
				return nil, nil, err
			}

			// Read file data
			data, err := ioutil.ReadAll(f)
			if err != nil {
				return nil, nil, err
			}

			// Load the tileset
			err = ts.Load(data)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// We must also load the images of the tileset
	tsImages := make(map[string]*image.RGBA)
	for _, ts := range m.Tilesets {
		// Name of the tileset image file
		tsImage := filepath.Base(ts.Image.Source)

		// Open tileset image
		f, err := os.Open(filepath.Join(relativeDir, tsImage))
		if err != nil {
			return nil, nil, err
		}

		// Decode the image
		src, _, err := image.Decode(f)
		if err != nil {
			return nil, nil, err
		}

		// If need be, convert to RGBA
		rgba, ok := src.(*image.RGBA)
		if !ok {
			b := src.Bounds()
			rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
			draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
		}

		// Put into the tileset images map
		tsImages[tsImage] = rgba
	}

	return m, Load(m, c, tsImages), nil
}
