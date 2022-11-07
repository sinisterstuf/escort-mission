// Copyright 2021 Siôn le Roux.  All rights reserved.
// Use of this source code is subject to an MIT-style
// licence which can be found in the LICENSE file.

package main

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/solarlune/resolv"
)

// dogSpeed is the distance the dog moves per update cycle
const dogSpeed float64 = 0.3

// states of the dog
// It would be great to map them to the frameTag.Name from JSON
const (
	dogWalking  int = 0
	dogSniffing     = 1
	dogSitting      = 2
)

// Dog is player's companion
type Dog struct {
	Object *resolv.Object
	Angle  float64
	Speed  float64
	Frame  int
	State  int
	Sprite *SpriteSheet
}

// Update updates the state of the dog
func (d *Dog) Update(g *Game) {

	d.MoveForward()

	d.animate(g)
	d.Object.Update()
}

func (d *Dog) animate(g *Game) {
	// Update only in every 5th cycle
	if g.Tick%5 != 0 {
		return
	}

	ft := d.Sprite.Meta.FrameTags[d.State]

	if ft.From == ft.To {
		d.Frame = ft.From
	} else {
		// Contiuously increase the Frame counter between From and To
		d.Frame = (d.Frame-ft.From+1)%(ft.To-ft.From+1) + ft.From
	}
}


// MoveForward moves the dog forward without turning
func (d *Dog) MoveForward() {
	d.move(
		math.Cos(d.Angle)*dogSpeed,
		math.Sin(d.Angle)*dogSpeed,
	)
}

// MoveUp moves the dog upwards
func (d *Dog) MoveUp() {
	d.move(0, -dogSpeed)
}

// MoveDown moves the dog downwards
func (d *Dog) MoveDown() {
	d.move(0, dogSpeed)
}

// MoveLeft moves the dog left
func (d *Dog) MoveLeft() {
	d.move(-dogSpeed, 0)
}

// MoveRight moves the dog right
func (d *Dog) MoveRight() {
	d.move(dogSpeed, 0)
}

// Move the Dog by the given vector if it is possible to do so
func (d *Dog) move(dx, dy float64) {
	if collision := d.Object.Check(dx, dy, tagMob, tagWall); collision == nil {
		d.Object.X += dx
		d.Object.Y += dy
	}
}

// Draw draws the Dog to the screen
func (d *Dog) Draw(g *Game) {
	s := d.Sprite
	frame := s.Sprite[d.Frame]
	op := &ebiten.DrawImageOptions{}

	op.GeoM.Translate(
		float64(-frame.Position.W/2),
		float64(-frame.Position.H/2),
	)

	op.GeoM.Rotate(d.Angle + math.Pi/2)

	g.Camera.Surface.DrawImage(
		s.Image.SubImage(image.Rect(
			frame.Position.X,
			frame.Position.Y,
			frame.Position.X+frame.Position.W,
			frame.Position.Y+frame.Position.H,
		)).(*ebiten.Image),
		g.Camera.GetTranslation(
			op,
			float64(d.Object.X)+float64(frame.Position.W/2),
			float64(d.Object.Y)+float64(frame.Position.H/2)))

}