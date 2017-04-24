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

package tcell

import (
	"bytes"
	"io"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/transform"
)

// NewQuasiScreen returns a Screen that does not attach to real TTY interfaces,
// but rather a generic set of io.ReaderCloser and io.WriteCloser compatible
// implementations. The terminfo description is provided as a formal argument,
// along with initial width and height values instead of reading these values
// from environment variables.
func NewQuasiScreen(in io.ReadCloser, out io.WriteCloser, terminfo string, w, h int) (Screen, error) {
	ti, e := LookupTerminfo(terminfo)
	if e != nil {
		return nil, e
	}
	q := &qScreen{
		ti: ti,
		in: in,
		out: out,

		w: w,
		h: h,
	}

	q.keyexist = make(map[Key]bool)
	q.keycodes = make(map[string]*tKeyCode)
	if len(ti.Mouse) > 0 {
		q.mouse = []byte(ti.Mouse)
	}
	q.prepareKeys()
	q.buildAcsMap()
	q.fallback = make(map[rune]string)
	for k, v := range RuneFallbacks {
		q.fallback[k] = v
	}

	return q, nil
}

// qScreen represents a screen backed by a terminfo implementation, but not a
// real operating system tty/pty. Any Reader/Writer for input/output will do.
type qScreen struct {
	ti        *Terminfo
	h         int
	w         int
	fini      bool
	cells     CellBuffer
	in        io.ReadCloser
	out       io.WriteCloser
	curstyle  Style
	style     Style
	evch      chan Event
	quit      chan struct{}
	keyexist  map[Key]bool
	keycodes  map[string]*tKeyCode
	cx        int
	cy        int
	mouse     []byte
	clear     bool
	cursorx   int
	cursory   int
	baud      int
	wasbtn    bool
	acs       map[rune]string
	charset   string
	encoder   transform.Transformer
	decoder   transform.Transformer
	fallback  map[rune]string
	colors    map[Color]Color
	palette   []Color
	truecolor bool
	escaped   bool
	buttondn  bool

	forcesize bool

	sync.Mutex
}

func (q *qScreen) Init() error {
	q.evch = make(chan Event, 10)
	q.charset = "UTF-8"

	q.charset = getCharset()
	if enc := GetEncoding(q.charset); enc != nil {
		q.encoder = enc.NewEncoder()
		q.decoder = enc.NewDecoder()
	} else {
		return ErrNoCharset
	}
	ti := q.ti

	q.cells.Resize(q.w, q.h)

	if q.ti.SetFgBgRGB != "" || q.ti.SetFgRGB != "" || q.ti.SetBgRGB != "" {
		q.truecolor = true
	}
	if !q.truecolor {
		q.colors = make(map[Color]Color)
		q.palette = make([]Color, q.Colors())
		for i := 0; i < q.Colors(); i++ {
			q.palette[i] = Color(i)
			// identity map for our builtin colors
			q.colors[Color(i)] = Color(i)
		}
	}

	q.TPuts(ti.EnterCA)
	q.TPuts(ti.HideCursor)
	q.TPuts(ti.EnableAcs)
	q.TPuts(ti.Clear)

	q.quit = make(chan struct{})

	q.Lock()
	q.cx = -1
	q.cy = -1
	q.style = StyleDefault
	q.cells.Resize(q.w, q.h)
	q.cursorx = -1
	q.cursory = -1
	q.resize()
	q.Unlock()

	go q.inputLoop()

	return nil
}

func (q *qScreen) prepareKeyMod(key Key, mod ModMask, val string) {
	if val != "" {
		// Do not overrride codes that already exist
		if _, exist := q.keycodes[val]; !exist {
			q.keyexist[key] = true
			q.keycodes[val] = &tKeyCode{key: key, mod: mod}
		}
	}
}

func (q *qScreen) prepareKey(key Key, val string) {
	q.prepareKeyMod(key, ModNone, val)
}

func (q *qScreen) prepareKeys() {
	ti := q.ti
	q.prepareKey(KeyBackspace, ti.KeyBackspace)
	q.prepareKey(KeyF1, ti.KeyF1)
	q.prepareKey(KeyF2, ti.KeyF2)
	q.prepareKey(KeyF3, ti.KeyF3)
	q.prepareKey(KeyF4, ti.KeyF4)
	q.prepareKey(KeyF5, ti.KeyF5)
	q.prepareKey(KeyF6, ti.KeyF6)
	q.prepareKey(KeyF7, ti.KeyF7)
	q.prepareKey(KeyF8, ti.KeyF8)
	q.prepareKey(KeyF9, ti.KeyF9)
	q.prepareKey(KeyF10, ti.KeyF10)
	q.prepareKey(KeyF11, ti.KeyF11)
	q.prepareKey(KeyF12, ti.KeyF12)
	q.prepareKey(KeyF13, ti.KeyF13)
	q.prepareKey(KeyF14, ti.KeyF14)
	q.prepareKey(KeyF15, ti.KeyF15)
	q.prepareKey(KeyF16, ti.KeyF16)
	q.prepareKey(KeyF17, ti.KeyF17)
	q.prepareKey(KeyF18, ti.KeyF18)
	q.prepareKey(KeyF19, ti.KeyF19)
	q.prepareKey(KeyF20, ti.KeyF20)
	q.prepareKey(KeyF21, ti.KeyF21)
	q.prepareKey(KeyF22, ti.KeyF22)
	q.prepareKey(KeyF23, ti.KeyF23)
	q.prepareKey(KeyF24, ti.KeyF24)
	q.prepareKey(KeyF25, ti.KeyF25)
	q.prepareKey(KeyF26, ti.KeyF26)
	q.prepareKey(KeyF27, ti.KeyF27)
	q.prepareKey(KeyF28, ti.KeyF28)
	q.prepareKey(KeyF29, ti.KeyF29)
	q.prepareKey(KeyF30, ti.KeyF30)
	q.prepareKey(KeyF31, ti.KeyF31)
	q.prepareKey(KeyF32, ti.KeyF32)
	q.prepareKey(KeyF33, ti.KeyF33)
	q.prepareKey(KeyF34, ti.KeyF34)
	q.prepareKey(KeyF35, ti.KeyF35)
	q.prepareKey(KeyF36, ti.KeyF36)
	q.prepareKey(KeyF37, ti.KeyF37)
	q.prepareKey(KeyF38, ti.KeyF38)
	q.prepareKey(KeyF39, ti.KeyF39)
	q.prepareKey(KeyF40, ti.KeyF40)
	q.prepareKey(KeyF41, ti.KeyF41)
	q.prepareKey(KeyF42, ti.KeyF42)
	q.prepareKey(KeyF43, ti.KeyF43)
	q.prepareKey(KeyF44, ti.KeyF44)
	q.prepareKey(KeyF45, ti.KeyF45)
	q.prepareKey(KeyF46, ti.KeyF46)
	q.prepareKey(KeyF47, ti.KeyF47)
	q.prepareKey(KeyF48, ti.KeyF48)
	q.prepareKey(KeyF49, ti.KeyF49)
	q.prepareKey(KeyF50, ti.KeyF50)
	q.prepareKey(KeyF51, ti.KeyF51)
	q.prepareKey(KeyF52, ti.KeyF52)
	q.prepareKey(KeyF53, ti.KeyF53)
	q.prepareKey(KeyF54, ti.KeyF54)
	q.prepareKey(KeyF55, ti.KeyF55)
	q.prepareKey(KeyF56, ti.KeyF56)
	q.prepareKey(KeyF57, ti.KeyF57)
	q.prepareKey(KeyF58, ti.KeyF58)
	q.prepareKey(KeyF59, ti.KeyF59)
	q.prepareKey(KeyF60, ti.KeyF60)
	q.prepareKey(KeyF61, ti.KeyF61)
	q.prepareKey(KeyF62, ti.KeyF62)
	q.prepareKey(KeyF63, ti.KeyF63)
	q.prepareKey(KeyF64, ti.KeyF64)
	q.prepareKey(KeyInsert, ti.KeyInsert)
	q.prepareKey(KeyDelete, ti.KeyDelete)
	q.prepareKey(KeyHome, ti.KeyHome)
	q.prepareKey(KeyEnd, ti.KeyEnd)
	q.prepareKey(KeyUp, ti.KeyUp)
	q.prepareKey(KeyDown, ti.KeyDown)
	q.prepareKey(KeyLeft, ti.KeyLeft)
	q.prepareKey(KeyRight, ti.KeyRight)
	q.prepareKey(KeyPgUp, ti.KeyPgUp)
	q.prepareKey(KeyPgDn, ti.KeyPgDn)
	q.prepareKey(KeyHelp, ti.KeyHelp)
	q.prepareKey(KeyPrint, ti.KeyPrint)
	q.prepareKey(KeyCancel, ti.KeyCancel)
	q.prepareKey(KeyExit, ti.KeyExit)
	q.prepareKey(KeyBacktab, ti.KeyBacktab)

	q.prepareKeyMod(KeyRight, ModShift, ti.KeyShfRight)
	q.prepareKeyMod(KeyLeft, ModShift, ti.KeyShfLeft)
	q.prepareKeyMod(KeyUp, ModShift, ti.KeyShfUp)
	q.prepareKeyMod(KeyDown, ModShift, ti.KeyShfDown)
	q.prepareKeyMod(KeyHome, ModShift, ti.KeyShfHome)
	q.prepareKeyMod(KeyEnd, ModShift, ti.KeyShfEnd)

	q.prepareKeyMod(KeyRight, ModCtrl, ti.KeyCtrlRight)
	q.prepareKeyMod(KeyLeft, ModCtrl, ti.KeyCtrlLeft)
	q.prepareKeyMod(KeyUp, ModCtrl, ti.KeyCtrlUp)
	q.prepareKeyMod(KeyDown, ModCtrl, ti.KeyCtrlDown)
	q.prepareKeyMod(KeyHome, ModCtrl, ti.KeyCtrlHome)
	q.prepareKeyMod(KeyEnd, ModCtrl, ti.KeyCtrlEnd)

	q.prepareKeyMod(KeyRight, ModAlt, ti.KeyAltRight)
	q.prepareKeyMod(KeyLeft, ModAlt, ti.KeyAltLeft)
	q.prepareKeyMod(KeyUp, ModAlt, ti.KeyAltUp)
	q.prepareKeyMod(KeyDown, ModAlt, ti.KeyAltDown)
	q.prepareKeyMod(KeyHome, ModAlt, ti.KeyAltHome)
	q.prepareKeyMod(KeyEnd, ModAlt, ti.KeyAltEnd)

	q.prepareKeyMod(KeyRight, ModAlt, ti.KeyMetaRight)
	q.prepareKeyMod(KeyLeft, ModAlt, ti.KeyMetaLeft)
	q.prepareKeyMod(KeyUp, ModAlt, ti.KeyMetaUp)
	q.prepareKeyMod(KeyDown, ModAlt, ti.KeyMetaDown)
	q.prepareKeyMod(KeyHome, ModAlt, ti.KeyMetaHome)
	q.prepareKeyMod(KeyEnd, ModAlt, ti.KeyMetaEnd)

	q.prepareKeyMod(KeyRight, ModAlt|ModShift, ti.KeyAltShfRight)
	q.prepareKeyMod(KeyLeft, ModAlt|ModShift, ti.KeyAltShfLeft)
	q.prepareKeyMod(KeyUp, ModAlt|ModShift, ti.KeyAltShfUp)
	q.prepareKeyMod(KeyDown, ModAlt|ModShift, ti.KeyAltShfDown)
	q.prepareKeyMod(KeyHome, ModAlt|ModShift, ti.KeyAltShfHome)
	q.prepareKeyMod(KeyEnd, ModAlt|ModShift, ti.KeyAltShfEnd)

	q.prepareKeyMod(KeyRight, ModAlt|ModShift, ti.KeyMetaShfRight)
	q.prepareKeyMod(KeyLeft, ModAlt|ModShift, ti.KeyMetaShfLeft)
	q.prepareKeyMod(KeyUp, ModAlt|ModShift, ti.KeyMetaShfUp)
	q.prepareKeyMod(KeyDown, ModAlt|ModShift, ti.KeyMetaShfDown)
	q.prepareKeyMod(KeyHome, ModAlt|ModShift, ti.KeyMetaShfHome)
	q.prepareKeyMod(KeyEnd, ModAlt|ModShift, ti.KeyMetaShfEnd)

	q.prepareKeyMod(KeyRight, ModCtrl|ModShift, ti.KeyCtrlShfRight)
	q.prepareKeyMod(KeyLeft, ModCtrl|ModShift, ti.KeyCtrlShfLeft)
	q.prepareKeyMod(KeyUp, ModCtrl|ModShift, ti.KeyCtrlShfUp)
	q.prepareKeyMod(KeyDown, ModCtrl|ModShift, ti.KeyCtrlShfDown)
	q.prepareKeyMod(KeyHome, ModCtrl|ModShift, ti.KeyCtrlShfHome)
	q.prepareKeyMod(KeyEnd, ModCtrl|ModShift, ti.KeyCtrlShfEnd)

	// Sadly, xterm handling of keycodes is somewhat erratic.  In
	// particular, different codes are sent depending on application
	// mode is in use or not, and the entries for many of these are
	// simply absent from terminfo on many systems.  So we insert
	// a number of escape sequences if they are not already used, in
	// order to have the widest correct usage.  Note that prepareKey
	// will not inject codes if the escape sequence is already known.
	// We also only do this for terminals that have the application
	// mode present.

	// Cursor mode
	if ti.EnterKeypad != "" {
		q.prepareKey(KeyUp, "\x1b[A")
		q.prepareKey(KeyDown, "\x1b[B")
		q.prepareKey(KeyRight, "\x1b[C")
		q.prepareKey(KeyLeft, "\x1b[D")
		q.prepareKey(KeyEnd, "\x1b[F")
		q.prepareKey(KeyHome, "\x1b[H")
		q.prepareKey(KeyDelete, "\x1b[3~")
		q.prepareKey(KeyHome, "\x1b[1~")
		q.prepareKey(KeyEnd, "\x1b[4~")
		q.prepareKey(KeyPgUp, "\x1b[5~")
		q.prepareKey(KeyPgDn, "\x1b[6~")

		// Application mode
		q.prepareKey(KeyUp, "\x1bOA")
		q.prepareKey(KeyDown, "\x1bOB")
		q.prepareKey(KeyRight, "\x1bOC")
		q.prepareKey(KeyLeft, "\x1bOD")
		q.prepareKey(KeyHome, "\x1bOH")
	}

	outer:
	// Add key mappings for control keys.
	for i := 0; i < ' '; i++ {
		// Do not insert direct key codes for ambiguous keys.
		// For example, ESC is used for lots of other keys, so
		// when parsing this we don't want to fast path handling
		// of it, but instead wait a bit before parsing it as in
		// isolation.
		for esc := range q.keycodes {
			if []byte(esc)[0] == byte(i) {
				continue outer
			}
		}

		q.keyexist[Key(i)] = true

		mod := ModCtrl
		switch Key(i) {
		case KeyBS, KeyTAB, KeyESC, KeyCR:
			// directly typeable- no control sequence
			mod = ModNone
		}
		q.keycodes[string(rune(i))] = &tKeyCode{key: Key(i), mod: mod}
	}
}

func (q *qScreen) Fini() {
	ti := q.ti
	q.Lock()
	q.cells.Resize(0, 0)
	q.TPuts(ti.ShowCursor)
	q.TPuts(ti.AttrOff)
	q.TPuts(ti.Clear)
	q.TPuts(ti.ExitCA)
	q.TPuts(ti.ExitKeypad)
	q.TPuts(ti.TParm(ti.MouseMode, 0))
	q.curstyle = Style(-1)
	q.clear = false
	q.fini = true
	q.Unlock()

	if q.quit != nil {
		close(q.quit)
	}

	q.out.Close()
	q.in.Close()
}

func (q *qScreen) SetStyle(style Style) {
	q.Lock()
	if !q.fini {
		q.style = style
	}
	q.Unlock()
}

func (q *qScreen) Clear() {
	q.Fill(' ', q.style)
}

func (q *qScreen) Fill(r rune, style Style) {
	q.Lock()
	if !q.fini {
		q.cells.Fill(r, style)
	}
	q.Unlock()
}

func (q *qScreen) SetContent(x, y int, mainc rune, combc []rune, style Style) {
	q.Lock()
	if !q.fini {
		q.cells.SetContent(x, y, mainc, combc, style)
	}
	q.Unlock()
}

func (q *qScreen) GetContent(x, y int) (rune, []rune, Style, int) {
	q.Lock()
	mainc, combc, style, width := q.cells.GetContent(x, y)
	q.Unlock()
	return mainc, combc, style, width
}

func (q *qScreen) SetCell(x, y int, style Style, ch ...rune) {
	if len(ch) > 0 {
		q.SetContent(x, y, ch[0], ch[1:], style)
	} else {
		q.SetContent(x, y, ' ', nil, style)
	}
}

func (q *qScreen) encodeRune(r rune, buf []byte) []byte {

	nb := make([]byte, 6)
	ob := make([]byte, 6)
	num := utf8.EncodeRune(ob, r)
	ob = ob[:num]
	dst := 0
	var err error
	if enc := q.encoder; enc != nil {
		enc.Reset()
		dst, _, err = enc.Transform(nb, ob, true)
	}
	if err != nil || dst == 0 || nb[0] == '\x1a' {
		// Combining characters are elided
		if len(buf) == 0 {
			if acs, ok := q.acs[r]; ok {
				buf = append(buf, []byte(acs)...)
			} else if fb, ok := q.fallback[r]; ok {
				buf = append(buf, []byte(fb)...)
			} else {
				buf = append(buf, '?')
			}
		}
	} else {
		buf = append(buf, nb[:dst]...)
	}

	return buf
}

func (q *qScreen) sendFgBg(fg Color, bg Color) {
	ti := q.ti
	if ti.Colors == 0 {
		return
	}
	if q.truecolor {
		if ti.SetFgBgRGB != "" &&
			fg != ColorDefault && bg != ColorDefault {
			r1, g1, b1 := fg.RGB()
			r2, g2, b2 := bg.RGB()
			q.TPuts(ti.TParm(ti.SetFgBgRGB,
				int(r1), int(g1), int(b1),
				int(r2), int(g2), int(b2)))
		} else {
			if fg != ColorDefault && ti.SetFgRGB != "" {
				r, g, b := fg.RGB()
				q.TPuts(ti.TParm(ti.SetFgRGB,
					int(r), int(g), int(b)))
			}
			if bg != ColorDefault && ti.SetBgRGB != "" {
				r, g, b := bg.RGB()
				q.TPuts(ti.TParm(ti.SetBgRGB,
					int(r), int(g), int(b)))
			}
		}
		return
	}

	if fg != ColorDefault {
		if v, ok := q.colors[fg]; ok {
			fg = v
		} else {
			v = FindColor(fg, q.palette)
			q.colors[fg] = v
			fg = v
		}
	}

	if bg != ColorDefault {
		if v, ok := q.colors[bg]; ok {
			bg = v
		} else {
			v = FindColor(bg, q.palette)
			q.colors[bg] = v
			bg = v
		}
	}

	if ti.SetFgBg != "" && fg != ColorDefault && bg != ColorDefault {
		q.TPuts(ti.TParm(ti.SetFgBg, int(fg), int(bg)))
	} else {
		if fg != ColorDefault && ti.SetFg != "" {
			q.TPuts(ti.TParm(ti.SetFg, int(fg)))
		}
		if bg != ColorDefault && ti.SetBg != "" {
			q.TPuts(ti.TParm(ti.SetBg, int(bg)))
		}
	}
}

func (q *qScreen) drawCell(x, y int) int {

	ti := q.ti

	mainc, combc, style, width := q.cells.GetContent(x, y)
	if !q.cells.Dirty(x, y) {
		return width
	}

	if q.cy != y || q.cx != x {
		q.TPuts(ti.TGoto(x, y))
		q.cx = x
		q.cy = y
	}

	if style == StyleDefault {
		style = q.style
	}
	if style != q.curstyle {
		fg, bg, attrs := style.Decompose()

		q.TPuts(ti.AttrOff)

		q.sendFgBg(fg, bg)
		if attrs&AttrBold != 0 {
			q.TPuts(ti.Bold)
		}
		if attrs&AttrUnderline != 0 {
			q.TPuts(ti.Underline)
		}
		if attrs&AttrReverse != 0 {
			q.TPuts(ti.Reverse)
		}
		if attrs&AttrBlink != 0 {
			q.TPuts(ti.Blink)
		}
		if attrs&AttrDim != 0 {
			q.TPuts(ti.Dim)
		}
		q.curstyle = style
	}
	// now emit runes - taking care to not overrun width with a
	// wide character, and to ensure that we emit exactly one regular
	// character followed up by any residual combing characters

	if width < 1 {
		width = 1
	}

	var str string

	buf := make([]byte, 0, 6)

	buf = q.encodeRune(mainc, buf)
	for _, r := range combc {
		buf = q.encodeRune(r, buf)
	}

	str = string(buf)
	if width > 1 && str == "?" {
		// No FullWidth character support
		str = "? "
		q.cx = -1
	}

	// XXX: check for hazeltine not being able to display ~

	if x > q.w-width {
		// too wide to fit; emit a single space instead
		width = 1
		str = " "
	}
	io.WriteString(q.out, str)
	q.cx += width
	q.cells.SetDirty(x, y, false)
	if width > 1 {
		q.cx = -1
	}

	return width
}

func (q *qScreen) ShowCursor(x, y int) {
	q.Lock()
	q.cursorx = x
	q.cursory = y
	q.Unlock()
}

func (q *qScreen) HideCursor() {
	q.ShowCursor(-1, -1)
}

func (q *qScreen) showCursor() {

	x, y := q.cursorx, q.cursory
	w, h := q.cells.Size()
	if x < 0 || y < 0 || x >= w || y >= h {
		q.hideCursor()
		return
	}
	q.TPuts(q.ti.TGoto(x, y))
	q.TPuts(q.ti.ShowCursor)
	q.cx = x
	q.cy = y
}

func (q *qScreen) TPuts(s string) {
	q.ti.TPuts(q.out, s, q.baud)
}

func (q *qScreen) Show() {
	q.Lock()
	if !q.fini {
		q.resize()
		q.draw()
	}
	q.Unlock()
}

func (q *qScreen) clearScreen() {
	fg, bg, _ := q.style.Decompose()
	q.sendFgBg(fg, bg)
	q.TPuts(q.ti.Clear)
	q.clear = false
}

func (q *qScreen) hideCursor() {
	// does not update cursor position
	if q.ti.HideCursor != "" {
		q.TPuts(q.ti.HideCursor)
	} else {
		// No way to hide cursor, stick it
		// at bottom right of screen
		q.cx, q.cy = q.cells.Size()
		q.TPuts(q.ti.TGoto(q.cx, q.cy))
	}
}

func (q *qScreen) draw() {
	// clobber cursor position, because we're gonna change it all
	q.cx = -1
	q.cy = -1

	// hide the cursor while we move stuff around
	q.hideCursor()

	if q.clear {
		q.clearScreen()
	}

	for y := 0; y < q.h; y++ {
		for x := 0; x < q.w; x++ {
			width := q.drawCell(x, y)
			if width > 1 {
				if x+1 < q.w {
					// this is necessary so that if we ever
					// go back to drawing that cell, we
					// actually will *draw* it.
					q.cells.SetDirty(x+1, y, true)
				}
			}
			x += width - 1
		}
	}

	// restore the cursor
	q.showCursor()
}

func (q *qScreen) EnableMouse() {
	if len(q.mouse) != 0 {
		q.TPuts(q.ti.TParm(q.ti.MouseMode, 1))
	}
}

func (q *qScreen) DisableMouse() {
	if len(q.mouse) != 0 {
		q.TPuts(q.ti.TParm(q.ti.MouseMode, 0))
	}
}

func (q *qScreen) Size() (int, int) {
	q.Lock()
	w, h := q.w, q.h
	q.Unlock()
	return w, h
}

func (q *qScreen) getWinSize() (int, int, error) {
	return q.w, q.h, nil
}

func (q *qScreen) resize() {
	if w, h, e := q.getWinSize(); e == nil {
		if w != q.w || h != q.h || q.forcesize {
			q.cx = -1
			q.cy = -1

			q.forcesize = false

			q.cells.Resize(w, h)
			q.cells.Invalidate()
			q.h = h
			q.w = w
			ev := NewEventResize(w, h)
			q.PostEvent(ev)
		}
	}
}

func (q *qScreen) Colors() int {
	// this doesn't change, no need for lock
	if q.truecolor {
		return 1 << 24
	}
	return q.ti.Colors
}

func (q *qScreen) PollEvent() Event {
	select {
	case <-q.quit:
		return nil
	case ev := <-q.evch:
		return ev
	}
}

// buildAcsMap builds a map of characters that we translate from Unicode to
// alternate character encodings.  To do this, we use the standard VT100 ACS
// maps.  This is only done if the terminal lacks support for Unicode; we
// always prefer to emit Unicode glyphs when we are able.
func (t *qScreen) buildAcsMap() {
	acsstr := t.ti.AltChars
	t.acs = make(map[rune]string)
	for len(acsstr) > 2 {
		srcv := acsstr[0]
		dstv := string(acsstr[1])
		if r, ok := vtACSNames[srcv]; ok {
			t.acs[r] = t.ti.EnterAcs + dstv + t.ti.ExitAcs
		}
		acsstr = acsstr[2:]
	}
}

func (q *qScreen) PostEventWait(ev Event) {
	q.evch <- ev
}

func (q *qScreen) PostEvent(ev Event) error {
	select {
	case q.evch <- ev:
		return nil
	default:
		return ErrEventQFull
	}
}

func (q *qScreen) clip(x, y int) (int, int) {
	w, h := q.cells.Size()
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > w-1 {
		x = w - 1
	}
	if y > h-1 {
		y = h - 1
	}
	return x, y
}

func (q *qScreen) postMouseEvent(x, y, btn int) {

	// XTerm mouse events only report at most one button at a time,
	// which may include a wheel button.  Wheel motion events are
	// reported as single impulses, while other button events are reported
	// as separate press & release events.

	button := ButtonNone
	mod := ModNone

	// Mouse wheel has bit 6 set, no release events.  It should be noted
	// that wheel events are sometimes misdelivered as mouse button events
	// during a click-drag, so we debounce these, considering them to be
	// button press events unless we see an intervening release event.
	switch btn & 0x43 {
	case 0:
		button = Button1
		q.wasbtn = true
	case 1:
		button = Button2
		q.wasbtn = true
	case 2:
		button = Button3
		q.wasbtn = true
	case 3:
		button = ButtonNone
		q.wasbtn = false
	case 0x40:
		if !q.wasbtn {
			button = WheelUp
		} else {
			button = Button1
		}
	case 0x41:
		if !q.wasbtn {
			button = WheelDown
		} else {
			button = Button2
		}
	}

	if btn&0x4 != 0 {
		mod |= ModShift
	}
	if btn&0x8 != 0 {
		mod |= ModAlt
	}
	if btn&0x10 != 0 {
		mod |= ModCtrl
	}

	// Some terminals will report mouse coordinates outside the
	// screen, especially with click-drag events.  Clip the coordinates
	// to the screen in that case.
	x, y = q.clip(x, y)

	ev := NewEventMouse(x, y, button, mod)
	q.PostEvent(ev)
}

// parseSgrMouse attempts to locate an SGR mouse record at the start of the
// buffer.  It returns true, true if it found one, and the associated bytes
// be removed from the buffer.  It returns true, false if the buffer might
// contain such an event, but more bytes are necessary (partial match), and
// false, false if the content is definitely *not* an SGR mouse record.
func (q *qScreen) parseSgrMouse(buf *bytes.Buffer) (bool, bool) {

	b := buf.Bytes()

	var x, y, btn, state int
	dig := false
	neg := false
	motion := false
	i := 0
	val := 0

	for i = range b {
		switch b[i] {
		case '\x1b':
			if state != 0 {
				return false, false
			}
			state = 1

		case '\x9b':
			if state != 0 {
				return false, false
			}
			state = 2

		case '[':
			if state != 1 {
				return false, false
			}
			state = 2

		case '<':
			if state != 2 {
				return false, false
			}
			val = 0
			dig = false
			neg = false
			state = 3

		case '-':
			if state != 3 && state != 4 && state != 5 {
				return false, false
			}
			if dig || neg {
				return false, false
			}
			neg = true // stay in state

		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			if state != 3 && state != 4 && state != 5 {
				return false, false
			}
			val *= 10
			val += int(b[i] - '0')
			dig = true // stay in state

		case ';':
			if neg {
				val = -val
			}
			switch state {
			case 3:
				btn, val = val, 0
				neg, dig, state = false, false, 4
			case 4:
				x, val = val-1, 0
				neg, dig, state = false, false, 5
			default:
				return false, false
			}

		case 'm', 'M':
			if state != 5 {
				return false, false
			}
			if neg {
				val = -val
			}
			y = val - 1

			motion = (btn & 32) != 0
			btn &^= 32
			if b[i] == 'm' {
				// mouse release, clear all buttons
				btn |= 3
				btn &^= 0x40
				q.buttondn = false
			} else if motion {
				/*
				 * Some broken terminals appear to send
				 * mouse button one motion events, instead of
				 * encoding 35 (no buttons) into these events.
				 * We resolve these by looking for a non-motion
				 * event first.
				 */
				if !q.buttondn {
					btn |= 3
					btn &^= 0x40
				}
			} else {
				q.buttondn = true
			}
			// consume the event bytes
			for i >= 0 {
				buf.ReadByte()
				i--
			}
			q.postMouseEvent(x, y, btn)
			return true, true
		}
	}

	// incomplete & inconclusve at this point
	return true, false
}

// parseXtermMouse is like parseSgrMouse, but it parses a legacy
// X11 mouse record.
func (q *qScreen) parseXtermMouse(buf *bytes.Buffer) (bool, bool) {

	b := buf.Bytes()

	state := 0
	btn := 0
	x := 0
	y := 0

	for i := range b {
		switch state {
		case 0:
			switch b[i] {
			case '\x1b':
				state = 1
			case '\x9b':
				state = 2
			default:
				return false, false
			}
		case 1:
			if b[i] != '[' {
				return false, false
			}
			state = 2
		case 2:
			if b[i] != 'M' {
				return false, false
			}
			state++
		case 3:
			btn = int(b[i])
			state++
		case 4:
			x = int(b[i]) - 32 - 1
			state++
		case 5:
			y = int(b[i]) - 32 - 1
			for i >= 0 {
				buf.ReadByte()
				i--
			}
			q.postMouseEvent(x, y, btn)
			return true, true
		}
	}
	return true, false
}

func (q *qScreen) parseFunctionKey(buf *bytes.Buffer) (bool, bool) {
	b := buf.Bytes()
	partial := false
	for e, k := range q.keycodes {
		esc := []byte(e)
		if (len(esc) == 1) && (esc[0] == '\x1b') {
			continue
		}
		if bytes.HasPrefix(b, esc) {
			// matched
			var r rune
			if len(esc) == 1 {
				r = rune(b[0])
			}
			mod := k.mod
			if q.escaped {
				mod |= ModAlt
				q.escaped = false
			}
			ev := NewEventKey(k.key, r, mod)
			q.PostEvent(ev)
			for i := 0; i < len(esc); i++ {
				buf.ReadByte()
			}
			return true, true
		}
		if bytes.HasPrefix(esc, b) {
			partial = true
		}
	}
	return partial, false
}

func (q *qScreen) parseRune(buf *bytes.Buffer) (bool, bool) {
	b := buf.Bytes()
	if b[0] >= ' ' && b[0] <= 0x7F {
		// printable ASCII easy to deal with -- no encodings
		mod := ModNone
		if q.escaped {
			mod = ModAlt
			q.escaped = false
		}
		ev := NewEventKey(KeyRune, rune(b[0]), mod)
		q.PostEvent(ev)
		buf.ReadByte()
		return true, true
	}

	if b[0] < 0x80 {
		// Low numbered values are control keys, not runes.
		return false, false
	}

	utfb := make([]byte, 12)
	for l := 1; l <= len(b); l++ {
		q.decoder.Reset()
		nout, nin, e := q.decoder.Transform(utfb, b[:l], true)
		if e == transform.ErrShortSrc {
			continue
		}
		if nout != 0 {
			r, _ := utf8.DecodeRune(utfb[:nout])
			if r != utf8.RuneError {
				mod := ModNone
				if q.escaped {
					mod = ModAlt
					q.escaped = false
				}
				ev := NewEventKey(KeyRune, r, mod)
				q.PostEvent(ev)
			}
			for nin > 0 {
				buf.ReadByte()
				nin--
			}
			return true, true
		}
	}
	// Looks like potential escape
	return true, false
}

func (q *qScreen) scanInput(buf *bytes.Buffer, expire bool) {

	q.Lock()
	defer q.Unlock()

	for {
		b := buf.Bytes()
		if len(b) == 0 {
			buf.Reset()
			return
		}

		partials := 0

		if part, comp := q.parseRune(buf); comp {
			continue
		} else if part {
			partials++
		}

		if part, comp := q.parseFunctionKey(buf); comp {
			continue
		} else if part {
			partials++
		}

		// Only parse mouse records if this term claims to have
		// mouse support

		if q.ti.Mouse != "" {
			if part, comp := q.parseXtermMouse(buf); comp {
				continue
			} else if part {
				partials++
			}

			if part, comp := q.parseSgrMouse(buf); comp {
				continue
			} else if part {
				partials++
			}
		}

		if partials == 0 || expire {
			if b[0] == '\x1b' {
				if len(b) == 1 {
					ev := NewEventKey(KeyEsc, 0, ModNone)
					q.PostEvent(ev)
					q.escaped = false
				} else {
					q.escaped = true
				}
				buf.ReadByte()
				continue
			}
			// Nothing was going to match, or we timed out
			// waiting for more data -- just deliver the characters
			// to the app & let them sort it out.  Possibly we
			// should only do this for control characters like ESC.
			by, _ := buf.ReadByte()
			mod := ModNone
			if q.escaped {
				q.escaped = false
				mod = ModAlt
			}
			ev := NewEventKey(KeyRune, rune(by), mod)
			q.PostEvent(ev)
			continue
		}

		// well we have some partial data, wait until we get
		// some more
		break
	}
}

func (q *qScreen) inputLoop() {
	buf := &bytes.Buffer{}

	chunk := make([]byte, 128)
	for {
		select {
		case <-q.quit:
			return
		default:
		}
		n, e := q.in.Read(chunk)
		switch e {
		case io.EOF:
			// If we timeout waiting for more bytes, then it's
			// time to give up on it.  Even at 300 baud it takes
			// less than 0.5 ms to transmit a whole byte.
			if buf.Len() > 0 {
				q.scanInput(buf, true)
			}
			continue
		case nil:
		default:
			return
		}
		buf.Write(chunk[:n])
		// Now we need to parse the input buffer for events
		q.scanInput(buf, false)
	}
}

func (q *qScreen) Sync() {
	q.Lock()
	q.cx = -1
	q.cy = -1
	if !q.fini {
		q.resize()
		q.clear = true
		q.cells.Invalidate()
		q.draw()
	}
	q.Unlock()
}

func (q *qScreen) CharacterSet() string {
	return q.charset
}

func (q *qScreen) RegisterRuneFallback(orig rune, fallback string) {
	q.Lock()
	q.fallback[orig] = fallback
	q.Unlock()
}

func (q *qScreen) UnregisterRuneFallback(orig rune) {
	q.Lock()
	delete(q.fallback, orig)
	q.Unlock()
}

func (q *qScreen) CanDisplay(r rune, checkFallbacks bool) bool {

	if enc := q.encoder; enc != nil {
		nb := make([]byte, 6)
		ob := make([]byte, 6)
		num := utf8.EncodeRune(ob, r)

		enc.Reset()
		dst, _, err := enc.Transform(nb, ob[:num], true)
		if dst != 0 && err == nil && nb[0] != '\x1A' {
			return true
		}
	}
	// Terminal fallbacks always permitted, since we assume they are
	// basically nearly perfect renditions.
	if _, ok := q.acs[r]; ok {
		return true
	}
	if !checkFallbacks {
		return false
	}
	if _, ok := q.fallback[r]; ok {
		return true
	}
	return false
}

func (q *qScreen) HasMouse() bool {
	return len(q.mouse) != 0
}

func (q *qScreen) HasKey(k Key) bool {
	if k == KeyRune {
		return true
	}
	return q.keyexist[k]
}

func (q *qScreen) Resize(_ int, _ int, w int, h int) {
	q.Lock()
	if w != q.w || h != q.h {
		q.forcesize = true
	}
	q.w, q.h = w, h
	q.Unlock()
	q.resize()
}
