// Copyright 2016 The TCell Authors
// Copyright 2017 Daniel Selifonov
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package termbox is a compatibility layer to allow tcells to emulate
// the github.com/nsf/termbox package.

// This version does additional encapsulation of QuasiScreens rather than
// real TTY backed terminals.
package quasibox

import (
	"errors"

	"github.com/thyth/tcell"
	"io"
)

type Quasibox struct {
	screen tcell.Screen
	outMode OutputMode
}

// Init initializes the screen for use, with the specified backing in/out, terminal type, and size.
func Init(in io.ReadCloser, out io.WriteCloser, terminfo string, w, h int) (*Quasibox, error) {
	outMode := OutputNormal
	if s, e := tcell.NewQuasiScreen(in, out, terminfo, w, h); e != nil {
		return nil, e
	} else if e = s.Init(); e != nil {
		return nil, e
	} else {
		return &Quasibox{
			screen: s,
			outMode: outMode,
		}, nil
	}
}

// InitLocal initializes a local TTY screen for use.
func InitLocal() (*Quasibox, error) {
	outMode := OutputNormal
	if s, e := tcell.NewScreen(); e != nil {
		return nil, e
	} else if e = s.Init(); e != nil {
		return nil, e
	} else {
		return &Quasibox{
			screen: s,
			outMode: outMode,
		}, nil
	}
}

// Close cleans up the terminal, restoring terminal modes, etc.
func (qb *Quasibox) Close() {
	qb.screen.Fini()
}

// Flush updates the screen.
func (qb *Quasibox) Flush() error {
	qb.screen.Show()
	return nil
}

// SetCursor displays the terminal cursor at the given location.
func (qb *Quasibox) SetCursor(x, y int) {
	qb.screen.ShowCursor(x, y)
}

// HideCursor hides the terminal cursor.
func (qb *Quasibox) HideCursor() {
	qb.SetCursor(-1, -1)
}

// Size returns the screen size as width, height in character cells.
func (qb *Quasibox) Size() (int, int) {
	return qb.screen.Size()
}

// SetSize controls resizing of a quasiscreen. This will be a no-op on tscreens.
func (qb *Quasibox) SetSize(w, h int) {
	qb.screen.Resize(0, 0, w, h)
}

// Attribute affects the presentation of characters, such as color, boldness,
// and so forth.
type Attribute uint16

// Colors first.  The order here is significant.
const (
	ColorDefault Attribute = iota
	ColorBlack
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
)

// Other attributes.
const (
	AttrBold Attribute = 1 << (9 + iota)
	AttrUnderline
	AttrReverse
)

func (qb *Quasibox) fixColor(c tcell.Color) tcell.Color {
	if c == tcell.ColorDefault {
		return c
	}
	switch qb.outMode {
	case OutputNormal:
		c %= tcell.Color(16)
	case Output256:
		c %= tcell.Color(256)
	case Output216:
		c %= tcell.Color(216)
		c += tcell.Color(16)
	case OutputGrayscale:
		c %= tcell.Color(24)
		c += tcell.Color(232)
	default:
		c = tcell.ColorDefault
	}
	return c
}

func (qb *Quasibox) mkStyle(fg, bg Attribute) tcell.Style {
	st := tcell.StyleDefault

	f := tcell.Color(int(fg)&0x1ff) - 1
	b := tcell.Color(int(bg)&0x1ff) - 1

	f = qb.fixColor(f)
	b = qb.fixColor(b)
	st = st.Foreground(f).Background(b)
	if (fg|bg)&AttrBold != 0 {
		st = st.Bold(true)
	}
	if (fg|bg)&AttrUnderline != 0 {
		st = st.Underline(true)
	}
	if (fg|bg)&AttrReverse != 0 {
		st = st.Reverse(true)
	}
	return st
}

// Clear clears the screen with the given attributes.
func (qb *Quasibox) Clear(fg, bg Attribute) {
	st := qb.mkStyle(fg, bg)
	w, h := qb.screen.Size()
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			qb.screen.SetContent(col, row, ' ', nil, st)
		}
	}
}

// InputMode is mostly unused.
type InputMode int

// Input modes; mostly here for compatibility.
const (
	InputCurrent InputMode = 1 << iota
	InputEsc
	InputAlt
	InputMouse
)

// SetInputMode will only enable mouse mode. Otherwise unused.
func (qb *Quasibox) SetInputMode(mode InputMode) InputMode {
	// enable mouse mode, if it's requested
	if mode & InputMouse != 0 {
		qb.screen.EnableMouse()
	}
	return mode
}

// OutputMode represents an output mode, which determines how colors
// are used.  See the termbox documentation for an explanation.
type OutputMode int

// OutputMode values.
const (
	OutputCurrent OutputMode = iota
	OutputNormal
	Output256
	Output216
	OutputGrayscale
)

// SetOutputMode is used to set the color palette used.
func (qb *Quasibox) SetOutputMode(mode OutputMode) OutputMode {
	if qb.screen.Colors() < 256 {
		mode = OutputNormal
	}
	switch mode {
	case OutputCurrent:
		return qb.outMode
	case OutputNormal, Output256, Output216, OutputGrayscale:
		qb.outMode = mode
		return mode
	default:
		return qb.outMode
	}
}

// Sync forces a resync of the screen.
func (qb *Quasibox) Sync() error {
	qb.screen.Sync()
	return nil
}

// SetCell sets the character cell at a given location to the given
// content (rune) and attributes.
func (qb *Quasibox) SetCell(x, y int, ch rune, fg, bg Attribute) {
	st := qb.mkStyle(fg, bg)
	qb.screen.SetContent(x, y, ch, nil, st)
}

// GetCellRune added to complete compatability implementation for gocui
func (qb *Quasibox) GetCellRune(x, y int) (rune) {
	mainc, _, _, _ := qb.screen.GetContent(x, y)
	return mainc
}

// EventType represents the type of event.
type EventType uint8

// Modifier represents the possible modifier keys.
type Modifier tcell.ModMask

// Key is a key press.
type Key tcell.Key

// Event represents an event like a key press, mouse action, or window resize.
type Event struct {
	Type   EventType
	Mod    Modifier
	Key    Key
	Ch     rune
	Width  int
	Height int
	Err    error
	MouseX int
	MouseY int
	N      int
}

// Event types.
const (
	EventNone EventType = iota
	EventKey
	EventResize
	EventMouse
	EventInterrupt
	EventError
	EventRaw
)

// Keys codes.
const (
	KeyF1         = Key(tcell.KeyF1)
	KeyF2         = Key(tcell.KeyF2)
	KeyF3         = Key(tcell.KeyF3)
	KeyF4         = Key(tcell.KeyF4)
	KeyF5         = Key(tcell.KeyF5)
	KeyF6         = Key(tcell.KeyF6)
	KeyF7         = Key(tcell.KeyF7)
	KeyF8         = Key(tcell.KeyF8)
	KeyF9         = Key(tcell.KeyF9)
	KeyF10        = Key(tcell.KeyF10)
	KeyF11        = Key(tcell.KeyF11)
	KeyF12        = Key(tcell.KeyF12)
	KeyInsert     = Key(tcell.KeyInsert)
	KeyDelete     = Key(tcell.KeyDelete)
	KeyHome       = Key(tcell.KeyHome)
	KeyEnd        = Key(tcell.KeyEnd)
	KeyArrowUp    = Key(tcell.KeyUp)
	KeyArrowDown  = Key(tcell.KeyDown)
	KeyArrowRight = Key(tcell.KeyRight)
	KeyArrowLeft  = Key(tcell.KeyLeft)
	KeyCtrlA      = Key(tcell.KeyCtrlA)
	KeyCtrlB      = Key(tcell.KeyCtrlB)
	KeyCtrlC      = Key(tcell.KeyCtrlC)
	KeyCtrlD      = Key(tcell.KeyCtrlD)
	KeyCtrlE      = Key(tcell.KeyCtrlE)
	KeyCtrlF      = Key(tcell.KeyCtrlF)
	KeyCtrlG      = Key(tcell.KeyCtrlG)
	KeyCtrlH      = Key(tcell.KeyCtrlH)
	KeyCtrlI      = Key(tcell.KeyCtrlI)
	KeyCtrlJ      = Key(tcell.KeyCtrlJ)
	KeyCtrlK      = Key(tcell.KeyCtrlK)
	KeyCtrlL      = Key(tcell.KeyCtrlL)
	KeyCtrlM      = Key(tcell.KeyCtrlM)
	KeyCtrlN      = Key(tcell.KeyCtrlN)
	KeyCtrlO      = Key(tcell.KeyCtrlO)
	KeyCtrlP      = Key(tcell.KeyCtrlP)
	KeyCtrlQ      = Key(tcell.KeyCtrlQ)
	KeyCtrlR      = Key(tcell.KeyCtrlR)
	KeyCtrlS      = Key(tcell.KeyCtrlS)
	KeyCtrlT      = Key(tcell.KeyCtrlT)
	KeyCtrlU      = Key(tcell.KeyCtrlU)
	KeyCtrlV      = Key(tcell.KeyCtrlV)
	KeyCtrlW      = Key(tcell.KeyCtrlW)
	KeyCtrlX      = Key(tcell.KeyCtrlX)
	KeyCtrlY      = Key(tcell.KeyCtrlY)
	KeyCtrlZ      = Key(tcell.KeyCtrlZ)
	KeyBackspace  = Key(tcell.KeyBackspace)
	KeyBackspace2 = Key(tcell.KeyBackspace2)
	KeyTab        = Key(tcell.KeyTab)
	KeyEnter      = Key(tcell.KeyEnter)
	KeyEsc        = Key(tcell.KeyEscape)
	KeyPgdn       = Key(tcell.KeyPgDn)
	KeyPgup       = Key(tcell.KeyPgUp)
	MouseLeft     = Key(tcell.Button1) // arbitrary assignments
	MouseRight    = Key(tcell.Button3)
	MouseMiddle   = Key(tcell.Button2)
	KeySpace      = Key(tcell.Key(' '))

	// missing set of termbox keybindings used by gocui
	KeyCtrlSpace = Key(tcell.KeyCtrlSpace)
	MouseRelease = Key(tcell.ButtonNone)
	MouseWheelUp = Key(tcell.WheelUp)
	MouseWheelDown = Key(tcell.WheelDown)
	KeyCtrlBackslash = Key(tcell.KeyCtrlBackslash)
	KeyCtrlLsqBracket = Key(tcell.KeyCtrlLeftSq)
	KeyCtrlRsqBracket = Key(tcell.KeyCtrlRightSq)
	KeyCtrlUnderscore = Key(tcell.KeyCtrlUnderscore)
)

// Modifiers.
const (
	ModAlt = Modifier(tcell.ModAlt)
)

func makeEvent(tev tcell.Event) Event {
	switch tev := tev.(type) {
	case *tcell.EventInterrupt:
		return Event{Type: EventInterrupt}
	case *tcell.EventResize:
		w, h := tev.Size()
		return Event{Type: EventResize, Width: w, Height: h}
	case *tcell.EventKey:
		k := tev.Key()
		ch := rune(0)
		if k == tcell.KeyRune {
			ch = tev.Rune()
			if ch == ' ' {
				k = tcell.Key(' ')
				ch = rune(0) // note: termbox clients assume no rune for space
			} else {
				k = 0 // note: termbox clients assume no key for runes
			}
		}
		mod := tev.Modifiers()
		// knock out the Ctrl modifier, since termbox clients use key values merged with Ctrl
		mod &= ^tcell.ModCtrl
		return Event{
			Type: EventKey,
			Key:  Key(k),
			Ch:   ch,
			Mod:  Modifier(mod),
		}
	case *tcell.EventMouse:
		x, y := tev.Position()
		b := tev.Buttons()
		mod := tev.Modifiers()
		return Event{
			Type: EventMouse,
			MouseX: x,
			MouseY: y,
			Key: Key(b),
			Mod: Modifier(mod),
		}
	default:
		return Event{Type: EventNone}
	}
}

// ParseEvent is not supported.
func ParseEvent(data []byte) Event {
	// Not supported
	return Event{Type: EventError, Err: errors.New("no raw events")}
}

// PollRawEvent is not supported.
func PollRawEvent(data []byte) Event {
	// Not supported
	return Event{Type: EventError, Err: errors.New("no raw events")}
}

// PollEvent blocks until an event is ready, and then returns it.
func (qb *Quasibox) PollEvent() Event {
	ev := qb.screen.PollEvent()
	return makeEvent(ev)
}

// Interrupt posts an interrupt event.
func (qb *Quasibox) Interrupt() {
	qb.screen.PostEvent(tcell.NewEventInterrupt(nil))
}

// Cell represents a single character cell on screen.
type Cell struct {
	Ch rune
	Fg Attribute
	Bg Attribute
}