package sound

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

// wavOf builds a real WAV file: sixteen bits, one channel, at a rate that is
// not the one everything is played at, so that the resampling is exercised too.
func wavOf(t *testing.T, rate, samples int) []byte {
	t.Helper()
	var body bytes.Buffer
	for i := 0; i < samples; i++ {
		v := int16(math.Sin(float64(i)/8) * 8000)
		binary.Write(&body, binary.LittleEndian, v)
	}
	data := body.Bytes()

	var f bytes.Buffer
	f.WriteString("RIFF")
	binary.Write(&f, binary.LittleEndian, uint32(36+len(data)))
	f.WriteString("WAVEfmt ")
	binary.Write(&f, binary.LittleEndian, uint32(16))     // chunk size
	binary.Write(&f, binary.LittleEndian, uint16(1))      // PCM
	binary.Write(&f, binary.LittleEndian, uint16(1))      // one channel
	binary.Write(&f, binary.LittleEndian, uint32(rate))   //
	binary.Write(&f, binary.LittleEndian, uint32(rate*2)) // bytes a second
	binary.Write(&f, binary.LittleEndian, uint16(2))      // bytes a frame
	binary.Write(&f, binary.LittleEndian, uint16(16))     // bits a sample
	f.WriteString("data")
	binary.Write(&f, binary.LittleEndian, uint32(len(data)))
	f.Write(data)
	return f.Bytes()
}

func TestDecodingAWav(t *testing.T) {
	const samples = 4800
	pcm, err := decode(wavOf(t, 24000, samples), SampleRate)
	if err != nil {
		t.Fatal(err)
	}

	// One channel at half the rate becomes two channels at the full one, so
	// four times the bytes, give or take what the resampler does at the ends.
	want := samples * 2 * 4
	if len(pcm) < want*9/10 || len(pcm) > want*11/10 {
		t.Errorf("decoded to %d bytes, want about %d", len(pcm), want)
	}
	if bytes.Equal(pcm, make([]byte, len(pcm))) {
		t.Error("it decoded to silence")
	}
}

func TestDecodingSomethingThatIsNotASound(t *testing.T) {
	_, err := decode([]byte("this is a text file"), SampleRate)
	if err == nil {
		t.Fatal("a text file is not a sound")
	}
	// The message has to say what it looked at, because the usual cause is a
	// file that is not what its name says.
	for _, want := range []string{"RIFF", "OggS", "this"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}

	if _, err := decode(nil, SampleRate); err == nil {
		t.Error("nothing at all is not a sound either")
	}
	if _, err := decode([]byte("OggS but not really"), SampleRate); err == nil {
		t.Error("something claiming to be an Ogg but is not should fail")
	}
}

func TestAFadeWalksToItsTarget(t *testing.T) {
	var f fade
	f.start(0, 1, 100, false) // six frames at sixty a second

	seen := []float64{}
	for i := 0; i < 20; i++ {
		v, done := f.advance()
		seen = append(seen, v)
		if done {
			break
		}
	}

	if len(seen) > 8 {
		t.Errorf("a fade of 100ms took %d frames", len(seen))
	}
	if last := seen[len(seen)-1]; last != 1 {
		t.Errorf("it arrived at %v, want 1", last)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Errorf("it did not keep rising: %v", seen)
			break
		}
	}
}

func TestAFadeDownEndsAtSilence(t *testing.T) {
	var f fade
	f.start(1, 0, 50, true)
	for i := 0; i < 100; i++ {
		v, done := f.advance()
		if done {
			if v != 0 {
				t.Errorf("it arrived at %v, want silence", v)
			}
			if !f.stopAtEnd {
				t.Error("a fade out should say to stop at the end of it")
			}
			return
		}
		if v < 0 || v > 1 {
			t.Fatalf("the volume went to %v", v)
		}
	}
	t.Error("the fade never arrived")
}

func TestNoTimeToFadeMeansArrivingAtOnce(t *testing.T) {
	var f fade
	f.start(0, 0.5, 0, false)
	if f.current != 0.5 {
		t.Errorf("the volume is %v, want 0.5 straight away", f.current)
	}
	v, done := f.advance()
	if !done || v != 0.5 {
		t.Errorf("advance gave %v, %v", v, done)
	}
}

func TestHowLongSomeSoundLasts(t *testing.T) {
	// Two channels of sixteen bits: four bytes a frame.
	pcm := make([]byte, SampleRate*4)
	if got := Silence(pcm, SampleRate).Seconds(); got != 1 {
		t.Errorf("a second of sound measured %v", got)
	}
	if got := Silence(pcm, 0); got != 0 {
		t.Errorf("at no rate at all it measured %v", got)
	}
}
