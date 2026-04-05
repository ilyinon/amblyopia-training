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

//go:embed beep.wav
var beepData []byte

//go:embed font.ttf
var fontData []byte

type Level struct {
	cellSize     int
	switchPeriod time.Duration
	targetTime   time.Duration
	jitter       bool
}

var levels = []Level{
	{60, 250 * time.Millisecond, 5 * time.Second, false},
	{50, 230 * time.Millisecond, 5 * time.Second, false},
	{40, 200 * time.Millisecond, 5 * time.Second, false},
	{30, 180 * time.Millisecond, 4 * time.Second, false},
	{20, 150 * time.Millisecond, 4 * time.Second, false},
	{15, 120 * time.Millisecond, 4 * time.Second, false},
	{12, 100 * time.Millisecond, 3 * time.Second, true},
	{10, 90 * time.Millisecond, 3 * time.Second, true},
	{8, 80 * time.Millisecond, 2 * time.Second, true},
	{8, 70 * time.Millisecond, 2 * time.Second, true},
}

type Game struct {
	level     int
	hits      int
	levelHits int
	misses    int

	cellSize     int
	switchPeriod time.Duration
	targetLimit  time.Duration
	jitter       bool

	targetX int
	targetY int

	targetStart time.Time

	sessionStart time.Time
	sessionLimit time.Duration

	jitterLast  time.Time
	jitterDelay time.Duration

	flash      bool
	lastSwitch time.Time

	whiteCell *ebiten.Image
	blackCell *ebiten.Image

	audioCtx *audio.Context
	beep     *audio.Player

	fontFace font.Face

	done bool
}

func (g *Game) setLevel(level int) {
	g.level = level
	g.levelHits = 0
	g.misses = 0

	cfg := levels[level-1]

	g.cellSize = cfg.cellSize
	g.switchPeriod = cfg.switchPeriod
	g.targetLimit = cfg.targetTime
	g.jitter = cfg.jitter

	if g.jitter {
		g.jitterDelay = 150 * time.Millisecond
	} else {
		g.jitterDelay = 0
	}

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
}

func (g *Game) playBeep() {
	g.beep.Rewind()
	g.beep.Play()
}

func (g *Game) score() int {
	score := g.hits*10 - g.misses*5
	if score < 0 {
		score = 0
	}

	total := g.hits + g.misses
	if total > 0 {
		acc := float64(g.hits) / float64(total)

		if acc > 0.8 {
			score += 50
		} else if acc > 0.6 {
			score += 20
		}
	}

	return score
}

func (g *Game) Update() error {
	// общий таймер сессии
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

	// мягкий jitter
	if g.jitter && time.Since(g.jitterLast) > g.jitterDelay {
		g.jitterLast = time.Now()

		g.targetX += rand.Intn(3) - 1
		g.targetY += rand.Intn(3) - 1

		if g.targetX < 1 {
			g.targetX = 1
		}
		if g.targetY < 1 {
			g.targetY = 1
		}
	}

	// таймер цели
	if time.Since(g.targetStart) > g.targetLimit {
		g.misses++
		g.randomTarget()

		if g.misses >= 3 {
			g.setLevel(g.level)
		}
	}

	mx, my := ebiten.CursorPosition()
	tx := g.targetX * g.cellSize
	ty := g.targetY * g.cellSize

	if mx >= tx && mx <= tx+g.cellSize &&
		my >= ty && my <= ty+g.cellSize {

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
			score, g.hits, g.misses, acc,
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
}

func isTargetCell(g *Game, cx, cy int) bool {
	if cx == g.targetX && cy == g.targetY {
		return true
	}
	if cx == g.targetX && (cy == g.targetY-1 || cy == g.targetY+1) {
		return true
	}
	if cy == g.targetY && (cx == g.targetX-1 || cx == g.targetX+1) {
		return true
	}
	return false
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// звук
	ctx := audio.NewContext(sampleRate)
	stream, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(beepData))
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
		sessionLimit: 3 * time.Minute,
		sessionStart: time.Now(),
	}

	game.setLevel(1)

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetFullscreen(true)
	ebiten.SetWindowTitle("Amblyopia Trainer")

	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
