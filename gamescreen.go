package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	beziercp "github.com/brothertoad/bezier"
	camera "github.com/melonfunction/ebiten-camera"
	"github.com/solarlune/ldtkgo"
	"github.com/solarlune/resolv"
)

const sampleRate int = 44100 // assuming "normal" sample rate
var context *audio.Context

func init() {
	context = audio.NewContext(sampleRate)
}

const (
	tagPlayer     = "player"
	tagMob        = "mob"
	tagWall       = "wall"
	tagDog        = "dog"
	tagEnd        = "end"
	tagCheckpoint = "check"
)

type GameScreen struct {
	Width         int
	Height        int
	Tick          int
	TileRenderer  *TileRenderer
	LDTKProject   *ldtkgo.Project
	Sounds        Sounds
	Voices        Voices
	Level         int
	Background    *ebiten.Image
	Foreground    *ebiten.Image
	Camera        *camera.Camera
	Sprites       map[SpriteType]*SpriteSheet
	ZombieSprites []*SpriteSheet
	Player        *Player
	Dog           *Dog
	SpawnPoints   SpawnPoints
	Zombies       Zombies
	Space         *resolv.Space
	LevelMap      LevelMap
	Checkpoint    int
	HUD           *HUD
}

// NewGame fills up the main Game data with assets, entities, pre-generated
// tiles and other things that take longer to load and would make the game pause
// before starting if we did it before the first Update loop
func NewGameScreen(game *Game) {
	g := &GameScreen{
		Width:  game.Width,
		Height: game.Height,
	}

	g.Camera = camera.NewCamera(g.Width, g.Height, 0, 0, 0, 1)

	var renderer *TileRenderer
	ldtkProject := loadMaps("assets/maps/maps.ldtk")
	renderer = NewTileRenderer(&EmbedLoader{"assets/maps"})

	g.TileRenderer = renderer
	g.LDTKProject = ldtkProject

	level := g.LDTKProject.Levels[g.Level]

	bg := ebiten.NewImage(level.Width, level.Height)
	bg.Fill(level.BGColor)
	fg := ebiten.NewImage(level.Width, level.Height)

	// Render map
	g.TileRenderer.Render(level)
	for _, layer := range g.TileRenderer.RenderedLayers {
		log.Println("Pre-drawing layer:", layer.Layer.Identifier)
		if layer.Layer.Identifier == "Treetops" {
			fg.DrawImage(layer.Image, &ebiten.DrawImageOptions{})
		} else {
			bg.DrawImage(layer.Image, &ebiten.DrawImageOptions{})
		}
	}
	g.Background = bg
	g.Foreground = fg

	// Create space for collision detection
	g.Space = resolv.NewSpace(level.Width, level.Height, 16, 16)

	// Create level map for A* path planning
	g.LevelMap = CreateMap(level.Width, level.Height)

	// Add wall tiles to space for collision detection
	for _, layer := range level.Layers {
		if layer.Type == ldtkgo.LayerTypeIntGrid && layer.Identifier == "Desert" {
			for _, intData := range layer.IntGrid {
				object := resolv.NewObject(
					float64(intData.Position[0]+layer.OffsetX),
					float64(intData.Position[1]+layer.OffsetY),
					float64(layer.GridSize),
					float64(layer.GridSize),
					tagWall,
				)
				object.SetShape(resolv.NewRectangle(
					float64(intData.Position[0]+layer.OffsetX),
					float64(intData.Position[1]+layer.OffsetY),
					float64(layer.GridSize),
					float64(layer.GridSize),
				))
				g.Space.Add(object)

				g.LevelMap.SetObstacle(intData.Position[0]/layer.GridSize, intData.Position[1]/layer.GridSize)
			}
		}
	}

	// Music
	g.Sounds = make([][]*audio.Player, 7)
	g.Sounds[soundMusicBackground] = make([]*audio.Player, 1)
	g.Sounds[soundMusicBackground][0] = NewMusicPlayer(loadSoundFile("assets/music/BackgroundMusic.ogg", sampleRate), context)
	g.Sounds[soundGunShot] = make([]*audio.Player, 1)
	g.Sounds[soundGunShot][0] = NewSoundPlayer(loadSoundFile("assets/sfx/Gunshot.ogg", sampleRate), context)
	g.Sounds[soundGunReload] = make([]*audio.Player, 1)
	g.Sounds[soundGunReload][0] = NewSoundPlayer(loadSoundFile("assets/sfx/Reload.ogg", sampleRate), context)
	g.Sounds[soundDogBark] = make([]*audio.Player, 5)
	for index := 0; index < 5; index++ {
		g.Sounds[soundDogBark][index] = NewSoundPlayer(
			loadSoundFile("assets/sfx/Dog-sound-"+strconv.Itoa(index+1)+".ogg", sampleRate),
			context,
		)
	}
	g.Sounds[soundPlayerDies] = make([]*audio.Player, 1)
	g.Sounds[soundPlayerDies][0] = NewSoundPlayer(loadSoundFile("assets/sfx/PlayerDies.ogg", sampleRate), context)
	g.Sounds[soundHit1] = make([]*audio.Player, 1)
	g.Sounds[soundHit1][0] = NewSoundPlayer(loadSoundFile("assets/sfx/Hit-1.ogg", sampleRate), context)
	g.Sounds[soundDryFire] = make([]*audio.Player, 1)
	g.Sounds[soundDryFire][0] = NewSoundPlayer(loadSoundFile("assets/sfx/DryFire.ogg", sampleRate), context)

	g.Sounds[soundMusicBackground][0].SetVolume(0.5)
	g.Sounds[soundMusicBackground][0].Play()

	// Voice
	g.Voices = make([][]*audio.Player, 1)
	g.Voices[voiceCheckpoint] = make([]*audio.Player, 7)
	for index := 0; index < 7; index++ {
		g.Voices[voiceCheckpoint][index] = NewSoundPlayer(
			loadSoundFile("assets/voice/Checkpoint-"+strconv.Itoa(index+1)+".ogg", sampleRate),
			context,
		)
	}

	// Load sprites
	g.Sprites = make(map[SpriteType]*SpriteSheet, 5)
	g.Sprites[spritePlayer] = loadSprite("Player")
	g.Sprites[spriteDog] = loadSprite("Dog")
	g.Sprites[spriteZombieSprinter] = loadSprite("Zombie_sprinter")
	g.Sprites[spriteZombieBig] = loadSprite("Zombie_big")
	g.Sprites[spriteZombieCrawler] = loadSprite("Zombie_crawler")
	g.ZombieSprites = make([]*SpriteSheet, zombieVariants)
	for index := 0; index < zombieVariants; index++ {
		g.ZombieSprites[index] = loadSprite("Zombie_" + strconv.Itoa(index))
	}

	// Load entities from map
	entities := level.LayerByIdentifier("Entities")

	// Add endpoint
	endpoint := entities.EntityByIdentifier("End")
	g.Space.Add(resolv.NewObject(
		float64(endpoint.Position[0]), float64(endpoint.Position[1]),
		float64(endpoint.Width), float64(endpoint.Height),
		tagEnd,
	))

	// Add player to the game
	playerPosition := entities.EntityByIdentifier("Player").Position
	g.Player = NewPlayer(playerPosition, g.Sprites[spritePlayer])
	g.Space.Add(g.Player.Object)

	for _, e := range entities.Entities {
		if strings.HasPrefix(e.Identifier, "Checkpoint") {
			eid, err := strconv.Atoi(e.Identifier[11:])
			if err != nil {
				log.Printf("Cannot load checkpoint: %s", e.Identifier)
				continue
			}
			log.Println(e.Identifier, e.Position)
			img := loadEntityImage(e.Identifier)
			w, h := img.Size()
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(e.Position[0]), float64(e.Position[1]))
			g.Background.DrawImage(img, op)
			obj := resolv.NewObject(
				float64(e.Position[0]), float64(e.Position[1]),
				float64(w), float64(h),
				tagCheckpoint,
			)
			obj.Data = eid
			g.Space.Add(obj)
		}
	}

	// Load the dog's path
	dogEntity := entities.EntityByIdentifier("Dog")
	pathArray := dogEntity.PropertyByIdentifier("Path").AsArray()
	// Start with the dog's current position
	pathPoints := []beziercp.PointF{{X: float64(dogEntity.Position[0]), Y: float64(dogEntity.Position[1])}}
	for _, pathCoord := range pathArray {
		pathPoints = append(pathPoints, beziercp.PointF{
			X: (pathCoord.(map[string]any)["cx"].(float64) + 0.5) * float64(entities.GridSize),
			Y: (pathCoord.(map[string]any)["cy"].(float64) + 0.5) * float64(entities.GridSize),
		})
	}

	dogPath := GetBezierPath(pathPoints, 4)

	// Add dog to the game
	object := resolv.NewObject(
		float64(dogEntity.Position[0]), float64(dogEntity.Position[1]),
		16, 16,
		tagDog,
	)
	object.SetShape(resolv.NewRectangle(
		0, 0,
		15, 8,
	))
	object.Shape.(*resolv.ConvexPolygon).RecenterPoints()
	g.Dog = &Dog{
		Object:   object,
		Angle:    0,
		Sprite:   g.Sprites[spriteDog],
		MainPath: &Path{Points: dogPath, NextPoint: 0},
	}
	g.Dog.Init()
	g.Space.Add(g.Dog.Object)

	// Add spawnpoints to the game
	for _, e := range entities.Entities {
		if e.Identifier == "Zombie" || e.Identifier == "Zombie_sprinter" || e.Identifier == "Zombie_big" {
			ztype := zombieNormal
			if e.Identifier == "Zombie_sprinter" {
				ztype = zombieSprinter
			} else if e.Identifier == "Zombie_big" {
				ztype = zombieBig
			}
			initialCount := e.PropertyByIdentifier("Initial").AsInt()
			continuous := e.PropertyByIdentifier("Continuous").AsBool()
			g.SpawnPoints = append(g.SpawnPoints, &SpawnPoint{
				Position:     Coord{X: float64(e.Position[0]), Y: float64(e.Position[1])},
				InitialCount: initialCount,
				Continuous:   continuous,
				ZombieType:   ztype,
			})
		}
	}

	g.HUD = NewHUD()

	game.Screens[gameRunning] = g
	game.State = gameRunning
}

func (g *GameScreen) Update() (GameState, error) {
	g.Tick++

	// Pressing R reloads the ammo
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		switch g.Player.State {
		case playerShooting, playerReload:
		default:
			g.Player.Reload(g)
		}
	}

	// Gun shooting handler
	if clicked() {
		Shoot(g)
	}

	// Update player
	g.Player.Update(g)

	// Update dog
	g.Dog.Update(g)

	// Update zombies
	g.Zombies.Update(g)

	// Update spawn points
	g.SpawnPoints.Update(g)

	// Collision detection and response between zombie and player
	if collision := g.Player.Object.Check(0, 0, tagMob); collision != nil {
		if g.Player.Object.Overlaps(collision.Objects[0]) {
			if g.Player.Object.Shape.Intersection(0, 0, collision.Objects[0].Shape) != nil {
				g.Sounds[soundMusicBackground][0].Pause()
				g.Sounds[soundMusicBackground][0].Rewind()
				g.Sounds[soundPlayerDies][0].Rewind()
				g.Sounds[soundPlayerDies][0].Play()
				return gameOver, nil // return early, no point in continuing, you are dead
			}
		}
	}

	// Do something special when you find a Checkpoint entity
	if collision := g.Player.Object.Check(0, 0, tagCheckpoint); collision != nil {
		if o := collision.Objects[0]; g.Player.Object.Overlaps(o) {
			if g.Checkpoint < o.Data.(int) {
				g.Checkpoint = o.Data.(int)
				g.Voices[voiceCheckpoint][g.Checkpoint-1].Rewind()
				g.Voices[voiceCheckpoint][g.Checkpoint-1].Play()
			}

		}
	}

	// End game when you reach the End entity
	if collision := g.Player.Object.Check(0, 0, tagEnd); collision != nil {
		if g.Player.Object.Overlaps(collision.Objects[0]) {
			return gameWon, nil
		}
	}

	// Collision detection and response between zombie and dog
	if collision := g.Dog.Object.Check(0, 0, tagMob); collision != nil {
		if g.Dog.Object.Overlaps(collision.Objects[0]) {
			g.Dog.Mode = dogDead
		}
	}

	// Game over if the dog dies
	if g.Dog.Mode == dogDead {
		g.Sounds[soundMusicBackground][0].Pause()
		g.Sounds[soundMusicBackground][0].Rewind()
		return gameOver, nil
	}

	// Position camera and clamp in to the Map dimensions
	level := g.LDTKProject.Levels[g.Level]
	g.Camera.SetPosition(
		math.Min(math.Max(g.Player.Object.X, float64(g.Width)/2), float64(level.Width)-float64(g.Width)/2),
		math.Min(math.Max(g.Player.Object.Y, float64(g.Height)/2), float64(level.Height)-float64(g.Height)/2))

	return gameRunning, nil
}

func (g *GameScreen) Draw(screen *ebiten.Image) {
	g.Camera.Surface.Clear()

	// Ground, walls and other lowest-level stuff needs to be drawn first
	g.Camera.Surface.DrawImage(
		g.Background,
		g.Camera.GetTranslation(&ebiten.DrawImageOptions{}, 0, 0),
	)

	// Dog
	g.Dog.Draw(g)

	// Player
	g.Player.Draw(g)

	// Zombies
	g.Zombies.Draw(g)

	// Tree tops etc. high-up stuff need to be drawn above the entities
	g.Camera.Surface.DrawImage(
		g.Foreground,
		g.Camera.GetTranslation(&ebiten.DrawImageOptions{}, 0, 0),
	)

	g.Camera.Blit(screen)

	g.HUD.Draw(g.Player.Ammo, screen)

	ebitenutil.DebugPrint(screen, fmt.Sprintf(
		"FPS: %.2f\n"+
			"Checkpoint: %d\n",
		ebiten.ActualFPS(),
		g.Checkpoint,
	))
}

func debugPosition(g *GameScreen, screen *ebiten.Image, o *resolv.Object) {
	verts := o.Shape.(*resolv.ConvexPolygon).Transformed()
	for i := 0; i < len(verts); i++ {
		vert := verts[i]
		next := verts[0]
		if i < len(verts)-1 {
			next = verts[i+1]
		}
		vX, vY := g.Camera.GetScreenCoords(vert.X(), vert.Y())
		nX, nY := g.Camera.GetScreenCoords(next.X(), next.Y())
		ebitenutil.DrawLine(screen, vX, vY, nX, nY, color.White)
	}
}

// Clicked is shorthand for when the left mouse button has just been clicked
func clicked() bool {
	return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

// Shoot sets shooting states and also die states for any zombies in range
func Shoot(g *GameScreen) {
	interruptReload := func() {
		g.Sounds[soundGunReload][0].Pause()
		g.Sounds[soundDryFire][0].Rewind()
		g.Sounds[soundDryFire][0].Play()
		g.Player.State = playerDryFire
	}

	switch g.Player.State {
	case playerShooting, playerReady, playerUnready:
		return // no-op
	case playerReload:
		interruptReload()
		return
	default:
		if g.Player.Ammo < 1 {
			interruptReload()
			return
		}

		g.Sounds[soundGunShot][0].Rewind()
		g.Sounds[soundGunShot][0].Play()

		g.Player.Ammo--
		g.Player.State = playerShooting
		rangeOfFire := g.Player.Range
		sX, sY := g.Space.WorldToSpace(
			g.Player.Object.X-math.Cos(g.Player.Angle-math.Pi)*rangeOfFire,
			g.Player.Object.Y-math.Sin(g.Player.Angle-math.Pi)*rangeOfFire,
		)
		pX, pY := g.Space.WorldToSpace(
			g.Player.Object.X+g.Player.Object.W/2,
			g.Player.Object.Y+g.Player.Object.H/2,
		)
		cells := g.Space.CellsInLine(pX, pY, sX, sY)
		for _, c := range cells {
			for _, o := range c.Objects {
				if o.HasTags(tagMob) {
					log.Println("HIT!")
					g.Sounds[soundHit1][0].Rewind()
					g.Sounds[soundHit1][0].Play()
					o.Data.(*Zombie).Hit()
					return // stop at the first zombie
				}
			}
		}
	}
}

// CalcObjectDistance calculates the distance between two Objects
func CalcObjectDistance(obj1, obj2 *resolv.Object) (float64, float64, float64) {
	return CalcDistance(obj1.X, obj1.Y, obj2.X, obj2.Y), obj1.X - obj2.X, obj1.Y - obj2.Y
}

// CalcDistance calculates the distance between two coordinates
func CalcDistance(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt(math.Pow(x1-x2, 2) + math.Pow(y1-y2, 2))
}

// NormalizeVector normalizes the vector
func NormalizeVector(vector Coord) Coord {
	magnitude := CalcDistance(vector.X, vector.Y, 0, 0)
	return Coord{X: vector.X / magnitude, Y: vector.Y / magnitude}
}
