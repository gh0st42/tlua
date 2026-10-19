package lualib

// Animations for the png module: png.frames reads an animated GIF, APNG or
// WebP, or a picture that does not move, as a list of frames. Each frame is
// the whole picture as it looks at that moment, so showing a frame never
// needs the ones before it.
//
// Go's own decoders give the first image of a PNG and nothing of an animated
// WebP, so APNG and WebP are taken apart here (their chunks, with each
// frame's place, delay and how it joins the picture) and each frame's
// pixels are decoded by image/png and golang.org/x/image/webp.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
	"time"

	"golang.org/x/image/webp"

	lua "github.com/yuin/gopher-lua"
)

// maxAnimationBytes is the most memory the frames of one animation may take
// (each is a whole picture of 4 bytes a pixel). A longer one gives its first
// frame only.
var maxAnimationBytes int64 = 256 << 20

type frame struct {
	pix   *image.NRGBA
	delay time.Duration
}

type animation struct {
	width, height int
	frames        []frame
	loops         int  // how many times it plays in all; 0 for ever
	truncated     bool // too big to hold: only the first frame is here
}

// animationDelay is how long a frame shows. A delay of a hundredth of a
// second or less is shown as a tenth, as browsers do, since a file that
// means "as fast as you can" is made so by programs that did not mean it.
func animationDelay(d time.Duration) time.Duration {
	if d <= 10*time.Millisecond {
		return 100 * time.Millisecond
	}
	return d
}

func clearRect(img *image.NRGBA, r image.Rectangle) {
	r = r.Intersect(img.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		o := img.PixOffset(r.Min.X, y)
		clear(img.Pix[o : o+r.Dx()*4])
	}
}

func copyNRGBA(img *image.NRGBA) *image.NRGBA {
	c := image.NewNRGBA(img.Rect)
	copy(c.Pix, img.Pix)
	return c
}

// room checks that count frames of w by h fit; when they do not, one frame
// is all there will be.
func (a *animation) room(count int) int {
	if int64(a.width)*int64(a.height)*4*int64(count) > maxAnimationBytes {
		a.truncated = true
		return 1
	}
	return count
}

// canvas makes the picture that frames are drawn on, or says it is too big.
func (a *animation) canvas() (*image.NRGBA, error) {
	if a.width < 1 || a.height < 1 || int64(a.width)*int64(a.height)*4 > maxAnimationBytes {
		return nil, fmt.Errorf("picture of %dx%d is too big", a.width, a.height)
	}
	return image.NewNRGBA(image.Rect(0, 0, a.width, a.height)), nil
}

// decodeAnimation reads any picture the module reads, as frames: those that
// do not move have one, with a delay of 0.
func decodeAnimation(data []byte) (*animation, error) {
	var a *animation
	var err error
	switch {
	case bytes.HasPrefix(data, []byte("GIF8")):
		a, err = decodeGIFAnimation(data)
	case bytes.HasPrefix(data, []byte(pngSignature)):
		a, err = decodeAPNG(data)
	case isWebP(data):
		a, err = decodeWebPAnimation(data)
	}
	if err != nil || a != nil {
		return a, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	pix := toNRGBA(src)
	return &animation{width: pix.Rect.Dx(), height: pix.Rect.Dy(), frames: []frame{{pix: pix}}}, nil
}

func decodeGIFAnimation(data []byte) (*animation, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	a := &animation{width: g.Config.Width, height: g.Config.Height, loops: g.LoopCount}
	// A GIF counts the times it goes round again after the first: -1 is
	// not at all.
	if a.loops < 0 {
		a.loops = 1
	} else if a.loops > 0 {
		a.loops++
	}
	canvas, err := a.canvas()
	if err != nil {
		return nil, err
	}
	keep := a.room(len(g.Image))
	var before *image.NRGBA
	for i, f := range g.Image[:keep] {
		disposal := byte(gif.DisposalNone)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			before = copyNRGBA(canvas)
		}
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
		delay := 10 // hundredths of a second
		if i < len(g.Delay) {
			delay = g.Delay[i]
		}
		a.frames = append(a.frames, frame{copyNRGBA(canvas), animationDelay(time.Duration(delay) * 10 * time.Millisecond)})
		switch disposal {
		case gif.DisposalBackground:
			clearRect(canvas, f.Bounds())
		case gif.DisposalPrevious:
			if before != nil {
				canvas = before
			}
		}
	}
	if len(a.frames) == 1 {
		a.frames[0].delay = 0
	}
	return a, nil
}

const pngSignature = "\x89PNG\r\n\x1a\n"

type chunk struct {
	kind string
	data []byte
}

func pngChunks(data []byte) ([]chunk, error) {
	var out []chunk
	for p := len(pngSignature); p+12 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[p:]))
		if n < 0 || p+12+n > len(data) {
			return nil, errors.New("png: truncated")
		}
		out = append(out, chunk{string(data[p+4 : p+8]), data[p+8 : p+8+n]})
		p += 12 + n
	}
	return out, nil
}

func writePNGChunk(buf *bytes.Buffer, kind string, data []byte) {
	binary.Write(buf, binary.BigEndian, uint32(len(data)))
	body := append([]byte(kind), data...)
	buf.Write(body)
	binary.Write(buf, binary.BigEndian, crc32.ChecksumIEEE(body))
}

// decodeAPNG reads an animated PNG; a PNG that is not animated is the
// ordinary decoder's (nil, nil).
func decodeAPNG(data []byte) (*animation, error) {
	chunks, err := pngChunks(data)
	if err != nil {
		return nil, err
	}
	type apngFrame struct {
		bounds         image.Rectangle
		delay          time.Duration
		dispose, blend byte
		idat           [][]byte
	}
	var ihdr []byte
	var shared []chunk // what each frame's own PNG needs besides its pixels
	var frames []*apngFrame
	var cur *apngFrame
	loops := 0
	animated := false
	for _, c := range chunks {
		switch c.kind {
		case "IHDR":
			ihdr = c.data
		case "PLTE", "tRNS", "gAMA", "sRGB", "cHRM", "iCCP":
			shared = append(shared, c)
		case "acTL":
			if len(c.data) >= 8 {
				animated = true
				loops = int(binary.BigEndian.Uint32(c.data[4:]))
			}
		case "fcTL":
			d := c.data
			if len(d) < 26 {
				return nil, errors.New("apng: bad frame control")
			}
			w, h := int(binary.BigEndian.Uint32(d[4:])), int(binary.BigEndian.Uint32(d[8:]))
			x, y := int(binary.BigEndian.Uint32(d[12:])), int(binary.BigEndian.Uint32(d[16:]))
			num, den := int(binary.BigEndian.Uint16(d[20:])), int(binary.BigEndian.Uint16(d[22:]))
			if den == 0 {
				den = 100
			}
			cur = &apngFrame{
				bounds:  image.Rect(x, y, x+w, y+h),
				delay:   animationDelay(time.Duration(num) * time.Second / time.Duration(den)),
				dispose: d[24], blend: d[25],
			}
			frames = append(frames, cur)
		case "IDAT":
			// Part of the animation only when a frame control came first.
			if cur != nil {
				cur.idat = append(cur.idat, c.data)
			}
		case "fdAT":
			if cur != nil && len(c.data) > 4 {
				cur.idat = append(cur.idat, c.data[4:])
			}
		}
	}
	if !animated || len(frames) == 0 || len(ihdr) != 13 {
		return nil, nil
	}
	a := &animation{
		width: int(binary.BigEndian.Uint32(ihdr)), height: int(binary.BigEndian.Uint32(ihdr[4:])), loops: loops,
	}
	canvas, err := a.canvas()
	if err != nil {
		return nil, err
	}
	for _, f := range frames[:a.room(len(frames))] {
		if !f.bounds.In(canvas.Rect) || f.bounds.Empty() {
			return nil, errors.New("apng: a frame is outside the picture")
		}
		var buf bytes.Buffer
		buf.WriteString(pngSignature)
		hdr := append([]byte{}, ihdr...)
		binary.BigEndian.PutUint32(hdr[0:], uint32(f.bounds.Dx()))
		binary.BigEndian.PutUint32(hdr[4:], uint32(f.bounds.Dy()))
		writePNGChunk(&buf, "IHDR", hdr)
		for _, c := range shared {
			writePNGChunk(&buf, c.kind, c.data)
		}
		for _, d := range f.idat {
			writePNGChunk(&buf, "IDAT", d)
		}
		writePNGChunk(&buf, "IEND", nil)
		img, err := png.Decode(&buf)
		if err != nil {
			return nil, fmt.Errorf("apng: %w", err)
		}
		var before *image.NRGBA
		if f.dispose == 2 {
			before = copyNRGBA(canvas)
		}
		op := draw.Src
		if f.blend == 1 {
			op = draw.Over
		}
		draw.Draw(canvas, f.bounds, img, img.Bounds().Min, op)
		a.frames = append(a.frames, frame{copyNRGBA(canvas), f.delay})
		switch f.dispose {
		case 1:
			clearRect(canvas, f.bounds)
		case 2:
			canvas = before
		}
	}
	if len(a.frames) == 1 {
		a.frames[0].delay = 0
	}
	return a, nil
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func le24(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }

func putLE24(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }

func riffChunks(data []byte) ([]chunk, error) {
	var out []chunk
	for p := 0; p+8 <= len(data); {
		n := int(binary.LittleEndian.Uint32(data[p+4:]))
		if n < 0 || p+8+n > len(data) {
			return nil, errors.New("webp: truncated")
		}
		out = append(out, chunk{string(data[p : p+4]), data[p+8 : p+8+n]})
		p += 8 + n + n%2
	}
	return out, nil
}

func writeRIFF(chunks []chunk) []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range chunks {
		body.WriteString(c.kind)
		binary.Write(&body, binary.LittleEndian, uint32(len(c.data)))
		body.Write(c.data)
		if len(c.data)%2 == 1 {
			body.WriteByte(0)
		}
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, binary.LittleEndian, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// decodeWebPAnimation reads an animated WebP; one that is not animated is
// the ordinary decoder's (nil, nil).
func decodeWebPAnimation(data []byte) (*animation, error) {
	chunks, err := riffChunks(data[12:])
	if err != nil {
		return nil, err
	}
	a := &animation{}
	animated := false
	count := 0
	for _, c := range chunks {
		switch c.kind {
		case "VP8X":
			if len(c.data) < 10 {
				return nil, errors.New("webp: bad VP8X")
			}
			animated = c.data[0]&2 != 0
			a.width, a.height = le24(c.data[4:])+1, le24(c.data[7:])+1
		case "ANIM":
			if len(c.data) >= 6 {
				a.loops = int(binary.LittleEndian.Uint16(c.data[4:]))
			}
		case "ANMF":
			count++
		}
	}
	if !animated || count == 0 {
		return nil, nil
	}
	canvas, err := a.canvas()
	if err != nil {
		return nil, err
	}
	keep := a.room(count)
	for _, c := range chunks {
		if c.kind != "ANMF" || len(c.data) < 16 {
			continue
		}
		if len(a.frames) == keep {
			break
		}
		x, y := le24(c.data[0:])*2, le24(c.data[3:])*2
		w, h := le24(c.data[6:])+1, le24(c.data[9:])+1
		bounds := image.Rect(x, y, x+w, y+h)
		if !bounds.In(canvas.Rect) {
			return nil, errors.New("webp: a frame is outside the picture")
		}
		flags := c.data[15]
		inner, err := riffChunks(c.data[16:])
		if err != nil {
			return nil, err
		}
		// The frame's bitstream is a picture of its own, which the still
		// decoder reads once it has the header a still picture has.
		vp8x := make([]byte, 10)
		for _, ic := range inner {
			if ic.kind == "ALPH" {
				vp8x[0] = 1 << 4
			}
		}
		putLE24(vp8x[4:], w-1)
		putLE24(vp8x[7:], h-1)
		img, err := webp.Decode(bytes.NewReader(writeRIFF(append([]chunk{{"VP8X", vp8x}}, inner...))))
		if err != nil {
			return nil, fmt.Errorf("webp: %w", err)
		}
		op := draw.Over
		if flags&2 != 0 { // blending is off: the frame replaces what is there
			op = draw.Src
		}
		draw.Draw(canvas, bounds, img, img.Bounds().Min, op)
		delay := animationDelay(time.Duration(le24(c.data[12:])) * time.Millisecond)
		a.frames = append(a.frames, frame{copyNRGBA(canvas), delay})
		if flags&1 != 0 { // dispose to the background: clear the frame's place
			clearRect(canvas, bounds)
		}
	}
	if len(a.frames) == 1 {
		a.frames[0].delay = 0
	}
	return a, nil
}

// pushFrames makes the table png.frames returns: the frames, each
// { image =, delay = seconds }, and width, height, loops (0 for ever) and
// truncated.
func pushFrames(L *lua.LState, a *animation) {
	t := L.NewTable()
	for _, f := range a.frames {
		entry := L.NewTable()
		pushImage(L, f.pix)
		entry.RawSetString("image", L.Get(-1))
		L.Pop(1)
		entry.RawSetString("delay", lua.LNumber(f.delay.Seconds()))
		t.Append(entry)
	}
	t.RawSetString("width", lua.LNumber(a.width))
	t.RawSetString("height", lua.LNumber(a.height))
	t.RawSetString("loops", lua.LNumber(a.loops))
	t.RawSetString("truncated", lua.LBool(a.truncated))
	L.Push(t)
}

func framesOf(L *lua.LState, data []byte) int {
	a, err := decodeAnimation(data)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	pushFrames(L, a)
	return 1
}

// pngFrames is png.frames(path): the frames of a picture, or nil and why. A
// fused program or a bundle reads its own files first.
func pngFrames(L *lua.LState) int {
	data, err := readFile(L, L.CheckString(1))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	return framesOf(L, data)
}

// pngDecodeFrames is png.decodeFrames(data): the same, from the bytes.
func pngDecodeFrames(L *lua.LState) int {
	return framesOf(L, []byte(L.CheckString(1)))
}
