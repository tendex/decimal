package decimal

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
)

// Scan implements sql.Scanner, so that a Decimal64 can be a destination of
// database/sql's Rows.Scan. It accepts the syntax of Parse64 as a string or
// []byte, which is how drivers deliver DECIMAL and NUMERIC columns, and
// keeps the scale of the column: 1.5 in a NUMERIC(10,2) scans as 1.50. An
// int64 or uint64 converts as New64 does. A float64 or float32, as a driver
// delivers a FLOAT or DOUBLE column, converts by way of the shortest decimal
// that identifies it, so that 0.1 scans as 0.1 rather than as the exact
// binary value New64FromFloat would give. A conversion with more than 16
// digits rounds to nearest even.
//
// A SQL NULL is an error; scan into a sql.Null[Decimal64] to accept one.
func (x *Decimal64) Scan(src any) error {
	var v Decimal64
	var err error
	switch src := src.(type) {
	case string:
		v, err = Parse64(src)
	case []byte:
		var c Context
		n, perr := c.parseBytes(&format64, "Parse64", src)
		v, err = pack64(n), perr
	case int64:
		v = New64(src, 0)
	case uint64:
		v = New64FromUint(src, 0)
	case float64:
		v, err = Parse64(strconv.FormatFloat(src, 'g', -1, 64))
	case float32:
		v, err = Parse64(strconv.FormatFloat(float64(src), 'g', -1, 32))
	case nil:
		return errors.New("decimal: cannot scan NULL into a Decimal64; use sql.Null[Decimal64]")
	default:
		return fmt.Errorf("decimal: cannot scan %T into a Decimal64", src)
	}
	if err != nil {
		return err
	}
	*x = v
	return nil
}

// Value implements driver.Valuer, so that a Decimal64 can be an argument of
// database/sql's Exec and Query. The value is a string in plain decimal
// notation with every digit of the coefficient, as Text('f', -1) gives it:
// 19.990 is "19.990", and 1.25E+30 is "1250000000000000000000000000000".
// Any DECIMAL or NUMERIC column accepts that form, and the scale of x
// survives where the column's allows. Infinities and NaNs are their String
// forms, which a database may reject.
func (x Decimal64) Value() (driver.Value, error) {
	return x.Text('f', -1), nil
}
