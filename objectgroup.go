// Copyright 2014 Lightpoke. All rights reserved.
// This source code is subject to the terms and
// conditions defined in the "License.txt" file.

package tmx

import (
	"fmt"
	"image/color"
)

// NOTE: x, y, width and height attributes are apparently meaningless:
//  https://github.com/bjorn/tiled/wiki/TMX-Map-Format#objectgroup
type xmlObjectgroup struct {
	Name       string        `xml:"name,attr"`
	Color      string        `xml:"color,attr"`
	Opacity    float64       `xml:"opacity,attr"`
	Visible    int           `xml:"visible,attr"`
	Properties xmlProperties `xml:"properties"`
	Object     []xmlObject   `xml:"object"`
}

func (x xmlObjectgroup) toObjectGroup() *ObjectGroup {
	objects := make([]*Object, len(x.Object))
	for i, o := range x.Object {
		objects[i] = o.toObject()
	}
	return &ObjectGroup{
		Name:       x.Name,
		Color:      hexToRGBA(x.Color),
		Opacity:    x.Opacity,
		Visible:    x.Visible != 0,
		Properties: x.Properties.toMap(),
		Objects:    objects,
	}
}

// ObjectGroup represents a group of objects.
type ObjectGroup struct {
	// The name of this object group.
	Name string

	// Color of this object group.
	Color color.RGBA

	// Value between 0 and 1 representing the opacity of the object group.
	Opacity float64

	// Boolean value representing whether or not the object group is visible.
	Visible bool

	// Map of properties for this object group.
	Properties map[string]string

	// List of objects in this object group.
	Objects []*Object
}

// String returns a string representation of this object group, like:
//  ObjectGroup(Name="the name", 500 objects)
func (o *ObjectGroup) String() string {
	return fmt.Sprintf("ObjectGroup(Name=%q, %d objects)", o.Name, len(o.Objects))
}
