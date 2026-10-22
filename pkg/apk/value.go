// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"fmt"
	"math"
	"strconv"
)

// Res_value data types.
const (
	typeNull       = 0x00
	typeReference  = 0x01
	typeAttribute  = 0x02
	typeString     = 0x03
	typeFloat      = 0x04
	typeDimension  = 0x05
	typeFraction   = 0x06
	typeDynRef     = 0x07
	typeDynAttr    = 0x08
	typeIntDec     = 0x10
	typeIntHex     = 0x11
	typeIntBoolean = 0x12
	typeColorFirst = 0x1c
	typeColorLast  = 0x1f
)

// value is a typed resource value (Res_value) with its string, when it is one.
type value struct {
	Type uint8
	Data uint32
	Str  string
}

func (v value) isRef() bool {
	return v.Type == typeReference || v.Type == typeDynRef
}

func (v value) isString() bool {
	return v.Type == typeString
}

// Int returns the value as an integer, parsing strings when needed.
func (v value) Int() (int64, bool) {
	switch {
	case v.Type >= typeIntDec && v.Type <= typeIntBoolean:
		return int64(int32(v.Data)), true //nolint:gosec // Res_value ints are signed 32-bit
	case v.Type == typeString:
		n, err := strconv.ParseInt(v.Str, 0, 64)
		return n, err == nil
	}

	return 0, false
}

// Bool returns the value as a boolean.
func (v value) Bool() (bool, bool) {
	switch v.Type {
	case typeIntBoolean, typeIntDec, typeIntHex:
		return v.Data != 0, true
	case typeString:
		b, err := strconv.ParseBool(v.Str)
		return b, err == nil
	}

	return false, false
}

// String renders the value the way aapt2 prints it.
func (v value) String() string {
	switch {
	case v.Type == typeString:
		return v.Str
	case v.Type == typeReference || v.Type == typeDynRef:
		return fmt.Sprintf("@0x%08x", v.Data)
	case v.Type == typeAttribute || v.Type == typeDynAttr:
		return fmt.Sprintf("?0x%08x", v.Data)
	case v.Type == typeIntBoolean:
		return strconv.FormatBool(v.Data != 0)
	case v.Type == typeIntHex:
		return fmt.Sprintf("0x%x", v.Data)
	case v.Type == typeIntDec:
		return strconv.FormatInt(int64(int32(v.Data)), 10) //nolint:gosec // signed 32-bit
	case v.Type == typeFloat:
		return strconv.FormatFloat(float64(math.Float32frombits(v.Data)), 'g', -1, 32)
	case v.Type >= typeColorFirst && v.Type <= typeColorLast:
		return fmt.Sprintf("#%08x", v.Data)
	case v.Type == typeNull:
		return ""
	}

	return fmt.Sprintf("(type 0x%02x)0x%x", v.Type, v.Data)
}
