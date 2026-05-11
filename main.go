package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	screenWidth  = 800
	screenHeight = 600
	sampleRate   = 44100
)

//go:embed assets/beep.wav
var beepData []byte

//go:embed assets/font.ttf
var fontData []byte

type Level struct {
	cellSize     int
	switchPeriod time.Duration
	targetTime   time.Duration
}

var levels = []Level{
	{30, 150 * time.Millisecond, 4 * time.Second},
	{20, 150 * time.Millisecond, 4 * time.Second},
	{15, 150 * time.Millisecond, 4 * time.Second},
	{12, 150 * time.Millisecond, 4 * time.Second},
	{10, 150 * time.Millisecond, 4 * time.Second},
	{8, 150 * time.Millisecond, 4 * time.Second},
	{6, 150 * time.Millisecond, 4 * time.Second},
	{4, 150 * time.Millisecond, 4 * time.Second},
}

type Game struct {
	level     int
	hits      int
	levelHits int
	misses    int

	cellSize     int
	switchPeriod time.Duration
	targetLimit  time.Duration

	targetX int
	targetY int

	targetStart time.Time

	sessionStart time.Time
	sessionLimit time.Duration

	flash      bool
	lastSwitch time.Time

	whiteCell *ebiten.Image
	blackCell *ebiten.Image

	audioCtx *audio.Context
	beep     *audio.Player

	fontFace font.Face

	done    bool
	hovered bool
}

func (g *Game) setLevel(level int) {
	g.level = level
	g.levelHits = 0

	cfg := levels[level-1]

	g.cellSize = cfg.cellSize
	g.switchPeriod = cfg.switchPeriod
	g.targetLimit = cfg.targetTime

	g.initCells()
	g.randomTarget()
}

func (g *Game) initCells() {
	g.whiteCell = ebiten.NewImage(g.cellSize, g.cellSize)
	g.whiteCell.Fill(color.White)

	g.blackCell = ebiten.NewImage(g.cellSize, g.cellSize)
	g.blackCell.Fill(color.Black)
}

func (g *Game) randomTarget() {
	cols := screenWidth / g.cellSize
	rows := screenHeight / g.cellSize

	g.targetX = rand.Intn(cols-2) + 1
	g.targetY = rand.Intn(rows-2) + 1

	g.targetStart = time.Now()
	g.hovered = false
}

func (g *Game) playBeep() {
	g.beep.Rewind()
	g.beep.Play()
}

func (g *Game) score() int {
	total := g.hits + g.misses
	if total == 0 {
		return 0
	}

	acc := float64(g.hits) / float64(total)

	score := g.hits*10 - g.misses*10
	score = int(float64(score) * (0.5 + acc))

	if score < 0 {
		score = 0
	}

	return score
}

func (g *Game) Update() error {

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	if time.Since(g.sessionStart) > g.sessionLimit {
		g.done = true
		return nil
	}

	if g.done {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
			len(inpututil.AppendJustPressedKeys(nil)) > 0 {
			return ebiten.Termination
		}
		return nil
	}

	// мигание
	if time.Since(g.lastSwitch) > g.switchPeriod {
		g.flash = !g.flash
		g.lastSwitch = time.Now()
	}

	// таймер цели
	if time.Since(g.targetStart) > g.targetLimit {
		g.misses++
		g.randomTarget()

		if g.misses >= 3 {
			g.setLevel(g.level)
		}
	}

	// попадание по наведению
	mx, my := ebiten.CursorPosition()

	cx := mx / g.cellSize
	cy := my / g.cellSize

	onTarget := isTargetCell(g, cx, cy)

	if onTarget && !g.hovered {

		g.playBeep()

		g.hits++
		g.levelHits++

		if g.levelHits >= 5 {

			if g.level == len(levels) {
				g.done = true
				return nil
			}

			g.setLevel(g.level + 1)

		} else {
			g.randomTarget()
		}
	}

	g.hovered = onTarget

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.done {
		screen.Fill(color.Black)

		score := g.score()
		total := g.hits + g.misses

		acc := 0.0
		if total > 0 {
			acc = float64(g.hits) / float64(total) * 100
		}

		msg := fmt.Sprintf(
			"Ты молодец!\n\nОчки: %d\nПопадания: %d\nПромахи: %d\nТочность: %.0f%%\n\nНажми любую клавишу",
			score,
			g.hits,
			g.misses,
			acc,
		)

		text.Draw(screen, msg, g.fontFace, 150, 250, color.White)
		return
	}

	for x := 0; x < screenWidth; x += g.cellSize {
		for y := 0; y < screenHeight; y += g.cellSize {

			white := ((x/g.cellSize)+(y/g.cellSize))%2 == 0

			if g.flash {
				white = !white
			}

			if isTargetCell(g, x/g.cellSize, y/g.cellSize) {
				white = !white
			}

			img := g.blackCell
			if white {
				img = g.whiteCell
			}

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))

			screen.DrawImage(img, op)
		}
	}

	drawCursor(screen, g)
}

func drawCursor(screen *ebiten.Image, g *Game) {
	mx, my := ebiten.CursorPosition()

	size := g.cellSize

	cx := mx / size
	cy := my / size

	for i := -1; i <= 1; i++ {

		drawCursorCell(screen, g, cx+i, cy, true)
		drawCursorCell(screen, g, cx, cy+i, true)
	}
}

func drawCursorCell(screen *ebiten.Image, g *Game, cx, cy int, invert bool) {
	size := g.cellSize

	white := ((cx + cy) % 2) == 0

	if g.flash {
		white = !white
	}

	if invert {
		white = !white
	}

	img := g.blackCell
	if white {
		img = g.whiteCell
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(cx*size), float64(cy*size))

	screen.DrawImage(img, op)
}

func isTargetCell(g *Game, cx, cy int) bool {

	// центр 2x2
	if cx >= g.targetX && cx <= g.targetX+1 &&
		cy >= g.targetY && cy <= g.targetY+1 {
		return true
	}

	// горизонталь
	if cy == g.targetY || cy == g.targetY+1 {
		if abs(cx-g.targetX) <= 2 {
			return true
		}
	}

	// вертикаль
	if cx == g.targetX || cx == g.targetX+1 {
		if abs(cy-g.targetY) <= 2 {
			return true
		}
	}

	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// звук
	ctx := audio.NewContext(sampleRate)

	stream, err := wav.DecodeWithSampleRate(
		sampleRate,
		bytes.NewReader(beepData),
	)
	if err != nil {
		log.Fatal(err)
	}

	player, err := ctx.NewPlayer(stream)
	if err != nil {
		log.Fatal(err)
	}

	// шрифт
	tt, err := opentype.Parse(fontData)
	if err != nil {
		log.Fatal(err)
	}

	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	game := &Game{
		audioCtx:     ctx,
		beep:         player,
		fontFace:     face,
		sessionLimit: 1 * time.Minute,
		sessionStart: time.Now(),
	}

	game.setLevel(1)

	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetFullscreen(true)
	ebiten.SetWindowTitle("Amblyopia Trainer")

	if err := ebiten.RunGame(game); err != nil &&
		err != ebiten.Termination {
		log.Fatal(err)
	}
}
