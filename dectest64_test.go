package decimal

import (
	"strconv"
	"strings"
	"testing"
)

// arg64 converts a decTest operand, which is either a numeric string or '#'
// followed by the hexadecimal DPD encoding.
func arg64(s string) (Decimal64, error) {
	if len(s) > 0 && s[0] == '#' {
		x, ok := hexArg64(s)
		if !ok {
			return x, errSkip
		}
		return x, nil
	}
	var c Context
	x, err := c.Parse64(s)
	if err != nil || c.Flags != 0 {
		// Operands are meant to be representable; anything else is a test
		// of string conversion, which tosci and apply handle themselves.
		return x, errSkip
	}
	return x, nil
}

func args64(args []string) ([]Decimal64, error) {
	out := make([]Decimal64, len(args))
	for i, s := range args {
		x, err := arg64(s)
		if err != nil {
			return nil, err
		}
		out[i] = x
	}
	return out, nil
}

func unary64(f func(c *Context, x Decimal64) Decimal64) decOp {
	return func(c *Context, args []string) (string, error) {
		x, err := args64(args[:1])
		if err != nil {
			return "", err
		}
		return f(c, x[0]).String(), nil
	}
}

func binary64(f func(c *Context, x, y Decimal64) Decimal64) decOp {
	return func(c *Context, args []string) (string, error) {
		x, err := args64(args[:2])
		if err != nil {
			return "", err
		}
		return f(c, x[0], x[1]).String(), nil
	}
}

// compare64 adapts a comparison: decTest expects -1, 0, 1 or the propagated
// NaN.
func compare64(f func(c *Context, x, y Decimal64) Ordering) decOp {
	return binaryNaN64(func(c *Context, x, y Decimal64) string {
		return strconv.Itoa(int(f(c, x, y)))
	})
}

// binaryNaN64 wraps an operation whose IEEE 754 form does not return a
// decimal, or treats NaNs differently: decTest expects NaN operands to
// propagate as they do for arithmetic.
func binaryNaN64(f func(c *Context, x, y Decimal64) string) decOp {
	return func(c *Context, args []string) (string, error) {
		x, err := args64(args[:2])
		if err != nil {
			return "", err
		}
		if x[0].IsNaN() || x[1].IsNaN() {
			f(c, x[0], x[1]) // for the flags
			var quiet Context
			return pack64(quiet.nan(&format64, x[0].unpack(), x[1].unpack())).String(), nil
		}
		return f(c, x[0], x[1]), nil
	}
}

var decOps64 = map[string]decOp{
	"add":      binary64((*Context).Add64),
	"subtract": binary64((*Context).Sub64),
	"multiply": binary64((*Context).Mul64),
	"divide":   binary64((*Context).Quo64),
	"quantize": binary64((*Context).Quantize64),
	"fma": func(c *Context, args []string) (string, error) {
		x, err := args64(args[:3])
		if err != nil {
			return "", err
		}
		return c.FMA64(x[0], x[1], x[2]).String(), nil
	},
	"remaindernear": func(c *Context, args []string) (string, error) {
		if strings.Contains(decWantConds, "division_impossible") {
			return "", errSkip // a General Decimal Arithmetic restriction
		}
		return binary64((*Context).Remainder64)(c, args)
	},
	"remainder": func(c *Context, args []string) (string, error) {
		if strings.Contains(decWantConds, "division_impossible") {
			return "", errSkip
		}
		return binary64((*Context).Mod64)(c, args)
	},

	"compare":    compare64((*Context).Compare64),
	"comparesig": compare64((*Context).CompareSignal64),
	"comparetotal": func(c *Context, args []string) (string, error) {
		x, err := args64(args[:2])
		if err != nil {
			return "", err
		}
		return strconv.Itoa(x[0].CmpTotal(x[1])), nil
	},
	"comparetotmag": func(c *Context, args []string) (string, error) {
		x, err := args64(args[:2])
		if err != nil {
			return "", err
		}
		return strconv.Itoa(x[0].Abs().CmpTotal(x[1].Abs())), nil
	},
	"samequantum": func(c *Context, args []string) (string, error) {
		x, err := args64(args[:2])
		if err != nil {
			return "", err
		}
		return bool01(x[0].SameQuantum(x[1])), nil
	},

	// decTest max and min are maxNum and minNum of IEEE 754-2008, which
	// differ from maximumNumber and minimumNumber of 754-2019 only in
	// returning a NaN when an operand is a signaling NaN.
	"max":    minMax64(true, false),
	"min":    minMax64(false, false),
	"maxmag": minMax64(true, true),
	"minmag": minMax64(false, true),

	"tointegralx": unary64((*Context).RoundToIntegralExact64),
	"tointegral": unary64(func(c *Context, x Decimal64) Decimal64 {
		return c.RoundToIntegral64(x, c.Rounding)
	}),
	"nextplus":  unary64((*Context).NextUp64),
	"nextminus": unary64((*Context).NextDown64),
	"logb":      unary64((*Context).LogB64),
	"reduce":    unary64((*Context).Reduce64),
	"scaleb": func(c *Context, args []string) (string, error) {
		x, err := args64(args[:2])
		if err != nil {
			return "", err
		}
		// decTest takes the scale as a decimal and restricts its range.
		switch n, ok := scaleArg64(x[1]); {
		case x[0].IsNaN() || x[1].IsNaN():
			return pack64(c.nan(&format64, x[0].unpack(), x[1].unpack())).String(), nil
		case !ok:
			return pack64(c.invalid()).String(), nil
		default:
			return c.ScaleB64(x[0], n).String(), nil
		}
	},

	// plus, minus and abs are arithmetic in decTest (0+x, 0-x), unlike the
	// quiet sign operations of IEEE 754.
	"plus": unary64(func(c *Context, x Decimal64) Decimal64 {
		return c.Add64(zeroLike64(x), x)
	}),
	"minus": unary64(func(c *Context, x Decimal64) Decimal64 {
		return c.Sub64(zeroLike64(x), x)
	}),
	"abs": unary64(func(c *Context, x Decimal64) Decimal64 {
		if x.Signbit() && !x.IsNaN() {
			return c.Sub64(zeroLike64(x), x)
		}
		return c.Add64(zeroLike64(x), x)
	}),
	"copy":       noHex(unary64(func(c *Context, x Decimal64) Decimal64 { return x })),
	"copyabs":    noHex(unary64(func(c *Context, x Decimal64) Decimal64 { return x.Abs() })),
	"copynegate": noHex(unary64(func(c *Context, x Decimal64) Decimal64 { return x.Neg() })),
	"copysign":   noHex(binary64(func(c *Context, x, y Decimal64) Decimal64 { return x.CopySign(y) })),
	"class": func(c *Context, args []string) (string, error) {
		x, err := args64(args[:1])
		if err != nil {
			return "", err
		}
		return decClassNames[x[0].Class()], nil
	},

	"canonical": unary64(func(c *Context, x Decimal64) Decimal64 { return x.Canonical() }),
	"tosci":     apply64,
	"apply":     apply64,
}

// apply64 converts the operand under the context, which is all that tosci
// and apply do.
func apply64(c *Context, args []string) (string, error) {
	if len(args[0]) > 0 && args[0][0] == '#' {
		x, err := arg64(args[0])
		return x.String(), err
	}
	x, _ := c.Parse64(args[0])
	return x.String(), nil
}

func minMax64(max, mag bool) decOp {
	return binary64(func(c *Context, x, y Decimal64) Decimal64 {
		if x.IsSignaling() || y.IsSignaling() {
			return pack64(c.nan(&format64, x.unpack(), y.unpack()))
		}
		return pack64(c.minMax(&format64, x.unpack(), y.unpack(), max, mag, true))
	})
}

func TestDecTest64(t *testing.T) { runDecTests(t, "dd*.decTest", decOps64, hex64) }
