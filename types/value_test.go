package types

import "testing"

func TestParseValue(t *testing.T) {
	tests := []struct {
		input    string
		wantKind Kind
	}{
		{"", KindNull},
		{"null", KindNull},
		{"NULL", KindNull},
		{"true", KindBool},
		{"false", KindBool},
		{"TRUE", KindBool},
		{"42", KindInt},
		{"-7", KindInt},
		{"3.14", KindFloat},
		{"-0.5", KindFloat},
		{"hello", KindString},
		{"  ", KindString}, // espacio no es NULL (no vacío)
	}
	for _, tt := range tests {
		v := ParseValue(tt.input)
		if v.Kind != tt.wantKind {
			t.Errorf("ParseValue(%q) kind = %d, want %d", tt.input, v.Kind, tt.wantKind)
		}
	}
}

func TestValueCmp(t *testing.T) {
	tests := []struct {
		a, b    Value
		wantCmp int
		wantErr bool
	}{
		{IntVal(1), IntVal(2), -1, false},
		{IntVal(2), IntVal(2), 0, false},
		{IntVal(3), IntVal(1), 1, false},
		{FloatVal(1.5), FloatVal(2.5), -1, false},
		{IntVal(2), FloatVal(2.0), 0, false}, // promoción numérica
		{FloatVal(3.0), IntVal(2), 1, false},
		{StringVal("a"), StringVal("b"), -1, false},
		{StringVal("z"), StringVal("z"), 0, false},
		{Null(), IntVal(1), 0, true}, // NULL → error
		{IntVal(1), Null(), 0, true},
	}
	for _, tt := range tests {
		cmp, err := tt.a.Cmp(tt.b)
		if (err != nil) != tt.wantErr {
			t.Errorf("Cmp(%v, %v) error = %v, wantErr %v", tt.a, tt.b, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && cmp != tt.wantCmp {
			t.Errorf("Cmp(%v, %v) = %d, want %d", tt.a, tt.b, cmp, tt.wantCmp)
		}
	}
}

func TestValueArith(t *testing.T) {
	tests := []struct {
		a, b     Value
		op       string
		wantKind Kind
		wantErr  bool
	}{
		{IntVal(3), IntVal(4), "+", KindInt, false},
		{IntVal(10), IntVal(3), "-", KindInt, false},
		{IntVal(3), FloatVal(2.0), "*", KindFloat, false},
		{FloatVal(6.0), FloatVal(2.0), "/", KindFloat, false},
		{IntVal(1), IntVal(0), "/", KindNull, true}, // div/0
		{Null(), IntVal(1), "+", KindNull, false},   // NULL propagation
	}
	for _, tt := range tests {
		var result Value
		var err error
		switch tt.op {
		case "+":
			result, err = Add(tt.a, tt.b)
		case "-":
			result, err = Sub(tt.a, tt.b)
		case "*":
			result, err = Mul(tt.a, tt.b)
		case "/":
			result, err = Div(tt.a, tt.b)
		}
		if (err != nil) != tt.wantErr {
			t.Errorf("%v %s %v: error = %v, wantErr = %v", tt.a, tt.op, tt.b, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && result.Kind != tt.wantKind {
			t.Errorf("%v %s %v: kind = %d, want %d", tt.a, tt.op, tt.b, result.Kind, tt.wantKind)
		}
	}
}

func TestValueString(t *testing.T) {
	tests := []struct {
		v    Value
		want string
	}{
		{Null(), "NULL"},
		{IntVal(42), "42"},
		{FloatVal(3.14), "3.14"},
		{StringVal("hello"), "hello"},
		{BoolVal(true), "true"},
		{BoolVal(false), "false"},
	}
	for _, tt := range tests {
		if got := tt.v.String(); got != tt.want {
			t.Errorf("Value.String() = %q, want %q", got, tt.want)
		}
	}
}
