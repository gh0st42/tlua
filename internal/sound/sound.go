// Package sound plays a game's sound effects and music through Ebitengine.
//
// It is the noisy half of the console's sfx() and music(): package picolua
// finds a sound and reads it, and this decodes it and puts it out of the
// speakers. Keeping the two apart is what lets everything else be tested
// without a sound card, and what would let something other than Ebitengine be
// put underneath.
package sound

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"tlua/internal/picolua"
)

// Engine is what the console's sfx() and music() reach.
var _ picolua.Sound = (*Engine)(nil)

// SampleRate is what everything is played at. Both decoders resample to it, so
// a game's sounds can be whatever its tools produced.
const SampleRate = 48000

// Engine is the console's sound: a handful of channels for effects and one
// player for the music.
type Engine struct {
	ctx *audio.Context

	// clips holds each sound decoded, by the name it was found under. A sound
	// effect is played over and over, and decoding an Ogg every time something
	// jumps would be a great deal of work for one noise.
	clips map[string][]byte

	channels [picolua.Channels]*audio.Player

	music     *audio.Player
	musicName string
	fade      fade

	volume float64

	// tps is how many times a second the game is run, which is what a fade
	// measured in milliseconds is counted against.
	tps int
}

// New starts the sound engine. Ebitengine's audio only works inside a running
// game, so this belongs to the window rather than to the interpreter.
func New() *Engine {
	ctx := audio.CurrentContext()
	if ctx == nil {
		ctx = audio.NewContext(SampleRate)
	}
	return &Engine{
		ctx:    ctx,
		clips:  map[string][]byte{},
		volume: 1,
		tps:    picolua.DefaultTPS,
	}
}

// clip reports a sound decoded, decoding and remembering it the first time.
func (e *Engine) clip(name string, data []byte) ([]byte, error) {
	if pcm, ok := e.clips[name]; ok {
		return pcm, nil
	}
	if data == nil {
		return nil, fmt.Errorf("%s has not been read", name)
	}
	pcm, err := decode(data, e.ctx.SampleRate())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	e.clips[name] = pcm
	return pcm, nil
}

// Play starts a sound on a channel, or on whichever one is free.
func (e *Engine) Play(name string, data []byte, channel int, volume float64) (int, error) {
	pcm, err := e.clip(name, data)
	if err != nil {
		return -1, err
	}

	if channel < 0 {
		channel = e.freeChannel()
	}
	channel %= picolua.Channels

	// A player is made for each go rather than rewound, because a sound played
	// again before it has finished should be two sounds and not one restarted.
	if old := e.channels[channel]; old != nil {
		old.Close()
	}
	player := e.ctx.NewPlayerFromBytes(pcm)
	player.SetVolume(volume * e.volume)
	player.Play()
	e.channels[channel] = player
	return channel, nil
}

// freeChannel reports a channel that has finished, or the first one if they are
// all busy: a game that plays nine things at once would rather lose the oldest
// than be told no.
func (e *Engine) freeChannel() int {
	for i, player := range e.channels {
		if player == nil || !player.IsPlaying() {
			return i
		}
	}
	return 0
}

// Stop silences a channel, or all of them.
func (e *Engine) Stop(channel int) {
	if channel < 0 {
		for i := range e.channels {
			e.close(i)
		}
		return
	}
	if channel < picolua.Channels {
		e.close(channel)
	}
}

func (e *Engine) close(channel int) {
	if player := e.channels[channel]; player != nil {
		player.Close()
		e.channels[channel] = nil
	}
}

// Music starts the music, which loops until something else is asked for.
func (e *Engine) Music(name string, data []byte, volume float64, fadeMS int) error {
	pcm, err := e.clip(name, data)
	if err != nil {
		return err
	}
	if e.music != nil {
		e.music.Close()
	}

	// The loop makes the stream endless, so the player never runs out.
	loop := audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm)))
	player, err := e.ctx.NewPlayer(loop)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	e.music, e.musicName = player, name

	e.fade.start(0, volume, fadeMS, e.tps, false)
	player.SetVolume(e.fade.volume() * e.volume)
	player.Play()
	return nil
}

// StopMusic fades the music out and stops it.
func (e *Engine) StopMusic(fadeMS int) {
	if e.music == nil {
		return
	}
	if fadeMS <= 0 {
		e.silenceMusic()
		return
	}
	e.fade.start(e.fade.volume(), 0, fadeMS, e.tps, true)
}

// Update moves a fade along by one frame. The host calls it once a tick.
func (e *Engine) Update() {
	if e.music == nil {
		return
	}
	volume, finished := e.fade.advance()
	if finished && e.fade.stopAtEnd {
		e.silenceMusic()
		return
	}
	e.music.SetVolume(volume * e.volume)
}

func (e *Engine) silenceMusic() {
	if e.music != nil {
		e.music.Close()
	}
	e.music, e.musicName = nil, ""
	e.fade = fade{}
}

// fade is a volume walked towards another one, a frame at a time.
//
// Counting in frames rather than measuring the time is what makes a fade fit a
// game loop: it moves when the game moves, and a game that is paused or busy
// does not come back to find the music already silent. The frames are counted
// rather than the volume accumulated, so that a fade of thirty frames takes
// thirty and not thirty-one.
type fade struct {
	from, to  float64
	at, of    int
	stopAtEnd bool
}

// start sets a fade going between two volumes over a number of milliseconds,
// counted in frames at the rate the game is being run. Nothing to fade over
// means arriving at once.
func (f *fade) start(from, to float64, ms, tps int, stopAtEnd bool) {
	if tps <= 0 {
		tps = picolua.DefaultTPS
	}
	*f = fade{from: from, to: to, of: max(ms*tps/1000, 0), stopAtEnd: stopAtEnd}
}

// volume reports where the fade has got to.
func (f *fade) volume() float64 {
	if f.of <= 0 {
		return f.to
	}
	return f.from + (f.to-f.from)*float64(f.at)/float64(f.of)
}

// advance moves the fade on by a frame, reporting the volume now and whether
// it has arrived.
func (f *fade) advance() (volume float64, finished bool) {
	if f.at < f.of {
		f.at++
	}
	return f.volume(), f.at >= f.of
}

// SetTPS says how often the game is being run, so that a fade of so many
// milliseconds lasts that long however fast that is.
func (e *Engine) SetTPS(rate int) {
	if rate > 0 {
		e.tps = rate
	}
}

// NowPlaying reports the music, or nothing.
func (e *Engine) NowPlaying() string { return e.musicName }

// Volume is how loud everything is played, from 0 to 1.
func (e *Engine) Volume() float64 { return e.volume }

// SetVolume changes it, taking whatever is already playing with it.
func (e *Engine) SetVolume(v float64) {
	e.volume = v
	if e.music != nil {
		e.music.SetVolume(e.fade.volume() * v)
	}
}

// Close stops everything, for a program that has ended.
func (e *Engine) Close() {
	e.Stop(-1)
	e.silenceMusic()
}

// decode turns the bytes of a sound file into the samples the audio device
// wants, at the rate everything else is played at.
//
// Which decoder to use is read off the front of the file rather than off the
// name it was found under: a game's sounds are whatever its tools produced, and
// a .wav that is really an Ogg is a thing that happens.
func decode(data []byte, rate int) ([]byte, error) {
	var (
		stream io.Reader
		err    error
	)
	switch {
	case bytes.HasPrefix(data, []byte("RIFF")):
		stream, err = wav.DecodeWithSampleRate(rate, bytes.NewReader(data))
	case bytes.HasPrefix(data, []byte("OggS")):
		stream, err = vorbis.DecodeWithSampleRate(rate, bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("not a sound file: it begins %q, and a .wav begins RIFF and an .ogg OggS", head(data))
	}
	if err != nil {
		return nil, err
	}

	pcm, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	if len(pcm) == 0 {
		return nil, fmt.Errorf("there is no sound in it")
	}
	return pcm, nil
}

// head reports the first few bytes of a file, for saying what it looked like.
func head(data []byte) string {
	if len(data) > 4 {
		data = data[:4]
	}
	return string(data)
}

// Silence is how long a sound of a given size lasts, for a test or a tool that
// wants to know.
func Silence(pcm []byte, rate int) time.Duration {
	const bytesPerSample = 4 // sixteen bits, two channels
	if rate <= 0 {
		return 0
	}
	return time.Duration(len(pcm)/bytesPerSample) * time.Second / time.Duration(rate)
}
