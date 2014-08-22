// Copyright 2014 Lightpoke. All rights reserved.
// This source code is subject to the terms and
// conditions defined in the "License.txt" file.

package tmx

import (
	"fmt"
	"strconv"
	"strings"
)

// Ellipse represents an ellipse object, found in the Object.Value field.
type Ellipse struct {
	// The position and size of the ellipse.
	//
	// These values are identical to the parent object's fields of the same
	// name, they are only provided for convenience.
	X, Y, Width, Height int
}

// Point represents a single point.
type Point struct {
	X, Y int
}

// Polygon represents a polygon object, found in the Object.Value field.
type Polygon struct {
	// The position/origin of the polygon.
	//
	// These values are identical to the parent object's fields of the same
	// name, they are only provided for convenience.
	X, Y int

	// Points making up the polygon, in pixels.
	Points []Point
}

// Polyline represents a polyline object, found in the Object.Value field.
type Polyline struct {
	// The position/origin of the polyline.
	//
	// These values are identical to the parent object's fields of the same
	// name, they are only provided for convenience.
	X, Y int

	// Points making up the polyline, in pixels.
	Points []Point
}

// Polygon and Polyset are identical, we share definitions here.
type xmlPolyset struct {
	Data string `xml:"points,attr"`
}

func (x xmlPolyset) toPoints() (points []Point) {
	pairs := strings.Split(x.Data, " ")
	points = make([]Point, 0, len(pairs))
	for _, pair := range pairs {
		xy := strings.Split(strings.TrimSpace(pair), ",")
		if len(xy) == 2 {
			x, _ := strconv.Atoi(strings.TrimSpace(xy[0]))
			y, _ := strconv.Atoi(strings.TrimSpace(xy[1]))
			points = append(points, Point{
				X: x,
				Y: y,
			})
		}
	}
	return points
}

type xmlObject struct {
	Name       string        `xml:"name,attr"`
	Type       string        `xml:"type,attr"`
	X          int           `xml:"x,attr"`
	Y          int           `xml:"y,attr"`
	Width      int           `xml:"width,attr"`
	Height     int           `xml:"height,attr"`
	Rotation   float64       `xml:"rotation,attr"`
	Gid        int           `xml:"gid,attr"`
	Visible    int           `xml:"visible,attr"`
	Properties xmlProperties `xml:"properties"`
	Ellipse    *string       `xml:"ellipse"`
	Polygon    xmlPolyset    `xml:"polygon"`
	Polyline   xmlPolyset    `xml:"polyline"`

	// FIXME: alledgedly, object tags can have images under them, but it's not
	// clear what that would mean. There also is no way to create one in
	// Tiled-Qt that I can find anywhere. My guess: it's not supposed to be
	// there at all and none actually have them.
}

func (x xmlObject) toValue() interface{} {
	if x.Ellipse != nil {
		return &Ellipse{
			X:      x.X,
			Y:      x.Y,
			Width:  x.Width,
			Height: x.Height,
		}
	}
	if len(x.Polygon.Data) > 0 {
		return &Polygon{
			X:      x.X,
			Y:      x.Y,
			Points: x.Polygon.toPoints(),
		}
	}
	if len(x.Polyline.Data) > 0 {
		return &Polyline{
			X:      x.X,
			Y:      x.Y,
			Points: x.Polyline.toPoints(),
		}
	}
	return nil
}

func (x xmlObject) toObject() *Object {
	return &Object{
		Name:       x.Name,
		Type:       x.Type,
		X:          x.X,
		Y:          x.Y,
		Width:      x.Width,
		Height:     x.Height,
		Rotation:   x.Rotation,
		Gid:        uint32(x.Gid),
		Visible:    x.Visible != 0,
		Properties: x.Properties.toMap(),
		Value:      x.toValue(),
	}
}

// Object represents a single object, which are generally used to add custom
// information to tile maps, like collision information, spawn points, etc.
type Object struct {
	// The name of this object.
	Name string

	// The type of this object, which is an arbitrary string.
	Type string

	// The X and Y coordinates, as well as the width and height of this object
	// in pixels.
	X, Y, Width, Height int

	// The rotation of this object in degrees clockwise.
	Rotation float64

	// Reference to a tile (optional). If it is non-zero then this object is
	// represented by the image of the tile with this global ID. Currently that
	// means width and height are ignored for such objects. The image alignment
	// currently depends on the map orientation:
	//  Orthogonal - Aligned to the bottom-left
	//  Isometric - Aligned to the bottom-center
	Gid uint32

	// Boolean value representing whether or not the object group is visible.
	Visible bool

	// Map of properties for this object group.
	Properties map[string]string

	// Value represents the underlying object value (which is sometimes nil).
	// You can use a type switch to determine it's value:
	//  switch v := obj.Value.(type) {
	//  case *tmx.Ellipse: handleEllipse(obj, v)
	//  case *tmx.Polygon: handlePolygon(obj, v)
	//  case *tmx.Polyline: handlePolyline(obj, v)
	//  case *tmx.Image: handleImage(obj, v)
	//  }
	Value interface{}
}

// String returns a string representation of this object, like:
//  Object(Name="the name", X=%d, Y=%d, Width=%d, Height=%d)
func (o *Object) String() string {
	return fmt.Sprintf("Object(Name=%q, X=%d, Y=%d, Width=%d, Height=%d)", o.Name, o.X, o.Y, o.Width, o.Height)
}
