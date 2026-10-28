package lua

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

func intMin(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func intMax(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func defaultFormat(v interface{}, f fmt.State, c rune) {
	buf := make([]string, 0, 10)
	buf = append(buf, "%")
	for i := 0; i < 128; i++ {
		if f.Flag(i) {
			buf = append(buf, string(rune(i)))
		}
	}

	if w, ok := f.Width(); ok {
		buf = append(buf, strconv.Itoa(w))
	}
	if p, ok := f.Precision(); ok {
		buf = append(buf, "."+strconv.Itoa(p))
	}
	buf = append(buf, string(c))
	format := strings.Join(buf, "")
	fmt.Fprintf(f, format, v)
}

type flagScanner struct {
	flag       byte
	start      string
	end        string
	buf        []byte
	str        string
	Length     int
	Pos        int
	HasFlag    bool
	ChangeFlag bool
}

func newFlagScanner(flag byte, start, end, str string) *flagScanner {
	return &flagScanner{flag, start, end, make([]byte, 0, len(str)), str, len(str), 0, false, false}
}

func (fs *flagScanner) AppendString(str string) { fs.buf = append(fs.buf, str...) }

func (fs *flagScanner) AppendChar(ch byte) { fs.buf = append(fs.buf, ch) }

func (fs *flagScanner) String() string { return string(fs.buf) }

func (fs *flagScanner) Next() (byte, bool) {
	c := byte('\000')
	fs.ChangeFlag = false
	if fs.Pos == fs.Length {
		if fs.HasFlag {
			fs.AppendString(fs.end)
		}
		return c, true
	} else {
		c = fs.str[fs.Pos]
		if c == fs.flag {
			if fs.Pos < (fs.Length-1) && fs.str[fs.Pos+1] == fs.flag {
				fs.HasFlag = false
				fs.AppendChar(fs.flag)
				fs.Pos += 2
				return fs.Next()
			} else if fs.Pos != fs.Length-1 {
				if fs.HasFlag {
					fs.AppendString(fs.end)
				}
				fs.AppendString(fs.start)
				fs.ChangeFlag = true
				fs.HasFlag = true
			}
		}
	}
	fs.Pos++
	return c, false
}

// strftime formats a time as C's strftime does in the C locale, for os.date.
// The E and O modifiers are accepted and ignored; a conversion it does not
// know is left as written.
func strftime(t time.Time, cfmt string) string {
	var b strings.Builder
	yday := t.YearDay() - 1 // 0 to 365
	wday := int(t.Weekday())
	for i := 0; i < len(cfmt); i++ {
		c := cfmt[i]
		if c != '%' || i+1 >= len(cfmt) {
			b.WriteByte(c)
			continue
		}
		i++
		c = cfmt[i]
		if (c == 'E' || c == 'O') && i+1 < len(cfmt) {
			i++
			c = cfmt[i]
		}
		switch c {
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'A':
			b.WriteString(t.Format("Monday"))
		case 'b', 'h':
			b.WriteString(t.Format("Jan"))
		case 'B':
			b.WriteString(t.Format("January"))
		case 'c':
			b.WriteString(t.Format("Mon Jan _2 15:04:05 2006"))
		case 'C':
			fmt.Fprintf(&b, "%02d", t.Year()/100)
		case 'd':
			b.WriteString(t.Format("02"))
		case 'D', 'x':
			b.WriteString(t.Format("01/02/06"))
		case 'e':
			b.WriteString(t.Format("_2"))
		case 'F':
			b.WriteString(t.Format("2006-01-02"))
		case 'g':
			y, _ := t.ISOWeek()
			fmt.Fprintf(&b, "%02d", y%100)
		case 'G':
			y, _ := t.ISOWeek()
			fmt.Fprintf(&b, "%d", y)
		case 'H':
			b.WriteString(t.Format("15"))
		case 'I':
			b.WriteString(t.Format("03"))
		case 'j':
			fmt.Fprintf(&b, "%03d", yday+1)
		case 'm':
			b.WriteString(t.Format("01"))
		case 'M':
			b.WriteString(t.Format("04"))
		case 'n':
			b.WriteByte('\n')
		case 'p':
			b.WriteString(t.Format("PM"))
		case 'P':
			b.WriteString(t.Format("pm"))
		case 'r':
			b.WriteString(t.Format("03:04:05 PM"))
		case 'R':
			b.WriteString(t.Format("15:04"))
		case 's':
			fmt.Fprintf(&b, "%d", t.Unix())
		case 'S':
			b.WriteString(t.Format("05"))
		case 't':
			b.WriteByte('\t')
		case 'T', 'X':
			b.WriteString(t.Format("15:04:05"))
		case 'u':
			if wday == 0 {
				b.WriteByte('7')
			} else {
				fmt.Fprintf(&b, "%d", wday)
			}
		case 'U':
			fmt.Fprintf(&b, "%02d", (yday+7-wday)/7)
		case 'V':
			_, w := t.ISOWeek()
			fmt.Fprintf(&b, "%02d", w)
		case 'w':
			fmt.Fprintf(&b, "%d", wday)
		case 'W':
			fmt.Fprintf(&b, "%02d", (yday+7-(wday+6)%7)/7)
		case 'y':
			b.WriteString(t.Format("06"))
		case 'Y':
			fmt.Fprintf(&b, "%d", t.Year())
		case 'z':
			b.WriteString(t.Format("-0700"))
		case 'Z':
			b.WriteString(t.Format("MST"))
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isInteger(v LNumber) bool {
	return float64(v) == float64(int64(v))
	//_, frac := math.Modf(float64(v))
	//return frac == 0.0
}

func isArrayKey(v LNumber) bool {
	return isInteger(v) && v < LNumber(int((^uint(0))>>1)) && v > LNumber(0) && v < LNumber(MaxArrayIndex)
}

func parseNumber(number string) (LNumber, error) {
	if v, ok := str2number(number); ok {
		return v, nil
	}
	return LNumber(0), fmt.Errorf("cannot convert %q to a number", number)
}

// isLuaSpace is C's isspace, which Lua skips around a number.
func isLuaSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func trimLuaSpace(s string) string {
	for len(s) > 0 && isLuaSpace(s[0]) {
		s = s[1:]
	}
	for len(s) > 0 && isLuaSpace(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// str2number reads a number the way Lua 5.1's luaO_str2d does: a decimal
// with an optional fraction and exponent (1, 1.5, .5, 5., 1e5, 2E-3), or a
// hexadecimal integer (0x1F), either with a sign, and space around it.
// Go's own syntax is not Lua's: no octal 010, no 0b101, no 1_000, and
// unlike C's strtod, no inf, nan or hexadecimal fractions.
func str2number(s string) (LNumber, bool) {
	s = trimLuaSpace(s)
	body := s
	neg := false
	if len(body) > 0 && (body[0] == '+' || body[0] == '-') {
		neg = body[0] == '-'
		body = body[1:]
	}
	if len(body) > 2 && body[0] == '0' && (body[1] == 'x' || body[1] == 'X') {
		digits := body[2:]
		for i := 0; i < len(digits); i++ {
			if !isHexDigit(digits[i]) {
				return 0, false
			}
		}
		v, err := strconv.ParseUint(digits, 16, 64)
		f := float64(v)
		if err != nil {
			// Too many digits for 64 bits: as strtoul, the largest there is.
			f = float64(^uint64(0))
		}
		if neg {
			f = -f
		}
		return LNumber(f), true
	}
	if !isDecimalNumber(body) {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, LNumberBit)
	if err != nil {
		// Out of range is HUGE_VAL or 0, as strtod gives; anything else
		// was not a number after all.
		if ne, ok := err.(*strconv.NumError); !ok || ne.Err != strconv.ErrRange {
			return 0, false
		}
	}
	return LNumber(v), true
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// isDecimalNumber checks digits, an optional point and fraction, and an
// optional exponent, with at least one digit before the exponent.
func isDecimalNumber(s string) bool {
	i, digits := 0, 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	return i == len(s)
}

func popenArgs(arg string) (string, []string) {
	cmd := "/bin/sh"
	args := []string{"-c"}
	if LuaOS == "windows" {
		cmd = "C:\\Windows\\system32\\cmd.exe"
		args = []string{"/c"}
	}
	args = append(args, arg)
	return cmd, args
}

func isGoroutineSafe(lv LValue) bool {
	switch v := lv.(type) {
	case *LFunction, *LUserData, *LState:
		return false
	case *LTable:
		return v.Metatable == LNil
	default:
		return true
	}
}

func readBufioSize(reader *bufio.Reader, size int64) ([]byte, error, bool) {
	result := []byte{}
	read := int64(0)
	var err error
	var n int
	for read != size {
		buf := make([]byte, size-read)
		n, err = reader.Read(buf)
		if err != nil {
			break
		}
		read += int64(n)
		result = append(result, buf[:n]...)
	}
	e := err
	if e != nil && e == io.EOF {
		e = nil
	}

	return result, e, len(result) == 0 && err == io.EOF
}

func readBufioLine(reader *bufio.Reader) ([]byte, error, bool) {
	result := []byte{}
	var buf []byte
	var err error
	var isprefix bool = true
	for isprefix {
		buf, isprefix, err = reader.ReadLine()
		if err != nil {
			break
		}
		result = append(result, buf...)
	}
	e := err
	if e != nil && e == io.EOF {
		e = nil
	}

	return result, e, len(result) == 0 && err == io.EOF
}

func int2Fb(val int) int {
	e := 0
	x := val
	for x >= 16 {
		x = (x + 1) >> 1
		e++
	}
	if x < 8 {
		return x
	}
	return ((e + 1) << 3) | (x - 8)
}

func strCmp(s1, s2 string) int {
	len1 := len(s1)
	len2 := len(s2)
	for i := 0; ; i++ {
		c1 := -1
		if i < len1 {
			c1 = int(s1[i])
		}
		c2 := -1
		if i != len2 {
			c2 = int(s2[i])
		}
		switch {
		case c1 < c2:
			return -1
		case c1 > c2:
			return +1
		case c1 < 0:
			return 0
		}
	}
}

func unsafeFastStringToReadOnlyBytes(s string) (bs []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&bs))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len
	return
}
