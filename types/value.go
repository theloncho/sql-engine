// Package types define el sistema de tipos del motor SQL.
// Un Value puede ser Int, Float, String, Bool, o NULL.
package types

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind identifica la variante de un Value.
type Kind int

const (
	KindNull Kind = iota
	KindInt
	KindFloat
	KindString
	KindBool
)

// Value representa un valor SQL tipado. El campo Kind indica qué campo está activo.
// Se usa una struct con campos directos (no interface{}) para eficiencia y claridad.
type Value struct {
	Kind Kind
	IVal int64
	FVal float64
	SVal string
	BVal bool
}

// --- Constructores ---

// Null crea un nuevo Value que representa un valor SQL NULL.
func Null() Value { return Value{Kind: KindNull} }

// IntVal crea un nuevo Value de tipo entero de 64 bits (KindInt).
func IntVal(v int64) Value { return Value{Kind: KindInt, IVal: v} }

// FloatVal crea un nuevo Value de tipo punto flotante de 64 bits (KindFloat).
func FloatVal(v float64) Value { return Value{Kind: KindFloat, FVal: v} }

// StringVal crea un nuevo Value de tipo texto/cadena (KindString).
func StringVal(v string) Value { return Value{Kind: KindString, SVal: v} }

// BoolVal crea un nuevo Value de tipo booleano (KindBool).
func BoolVal(v bool) Value { return Value{Kind: KindBool, BVal: v} }

// IsNull retorna true si el valor es NULL.
func (v Value) IsNull() bool { return v.Kind == KindNull }

// String retorna una representación legible del valor (para REPL/printer).
func (v Value) String() string {
	switch v.Kind {
	case KindNull:
		return "NULL"
	case KindInt:
		return strconv.FormatInt(v.IVal, 10)
	case KindFloat:
		// Evitamos notación científica para valores normales.
		s := strconv.FormatFloat(v.FVal, 'f', -1, 64)
		return s
	case KindString:
		return v.SVal
	case KindBool:
		if v.BVal {
			return "true"
		}
		return "false"
	default:
		return "?"
	}
}

// TypeName retorna el nombre del tipo para mensajes de error y catálogo.
func (v Value) TypeName() string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindString:
		return "string"
	case KindBool:
		return "bool"
	default:
		return "unknown"
	}
}

// --- Comparación ---

// Cmp compara dos valores. Retorna:
//   -1 si v < other
//    0 si v == other
//    1 si v > other
//   error si los tipos no son comparables.
// NULL comparado con cualquier cosa retorna error (lógica de tres valores se maneja en expr.go).
func (v Value) Cmp(other Value) (int, error) {
	if v.IsNull() || other.IsNull() {
		return 0, fmt.Errorf("cannot compare NULL values directly")
	}
	// Promoción numérica: int op float → ambos como float.
	lf, rf, numeric := promoteToFloat(v, other)
	if numeric {
		if lf < rf {
			return -1, nil
		} else if lf > rf {
			return 1, nil
		}
		return 0, nil
	}
	if v.Kind != other.Kind {
		return 0, fmt.Errorf("type mismatch: cannot compare %s and %s", v.TypeName(), other.TypeName())
	}
	switch v.Kind {
	case KindString:
		return strings.Compare(v.SVal, other.SVal), nil
	case KindBool:
		if v.BVal == other.BVal {
			return 0, nil
		} else if !v.BVal && other.BVal {
			return -1, nil
		}
		return 1, nil
	}
	return 0, fmt.Errorf("unsupported type for comparison: %s", v.TypeName())
}

// Equal reporta igualdad estricta de tipo y valor. NULL != NULL (retorna false, nil).
func (v Value) Equal(other Value) (bool, error) {
	if v.IsNull() || other.IsNull() {
		// En SQL, NULL = NULL es UNKNOWN, no TRUE.
		return false, nil
	}
	c, err := v.Cmp(other)
	if err != nil {
		return false, err
	}
	return c == 0, nil
}

// --- Aritmética ---

// Add suma dos valores numéricos.
func Add(a, b Value) (Value, error) {
	return applyArith(a, b, '+')
}

// Sub resta dos valores numéricos.
func Sub(a, b Value) (Value, error) {
	return applyArith(a, b, '-')
}

// Mul multiplica dos valores numéricos.
func Mul(a, b Value) (Value, error) {
	return applyArith(a, b, '*')
}

// Div divide dos valores numéricos. Retorna error si el divisor es cero.
func Div(a, b Value) (Value, error) {
	return applyArith(a, b, '/')
}

func applyArith(a, b Value, op rune) (Value, error) {
	if a.IsNull() || b.IsNull() {
		return Null(), nil // NULL propagation
	}
	af, bf, numeric := promoteToFloat(a, b)
	if !numeric {
		return Null(), fmt.Errorf("arithmetic requires numeric types, got %s and %s", a.TypeName(), b.TypeName())
	}
	var result float64
	switch op {
	case '+':
		result = af + bf
	case '-':
		result = af - bf
	case '*':
		result = af * bf
	case '/':
		if bf == 0 {
			return Null(), fmt.Errorf("division by zero")
		}
		result = af / bf
	}
	// Si ambos operandos son int y la operación es entera, devolvemos int.
	if a.Kind == KindInt && b.Kind == KindInt && op != '/' {
		return IntVal(int64(result)), nil
	}
	return FloatVal(result), nil
}

// promoteToFloat intenta promover ambos valores a float64 para aritmética/comparación.
// Retorna (lf, rf, ok): ok=true si ambos son numéricos.
func promoteToFloat(a, b Value) (float64, float64, bool) {
	toF := func(v Value) (float64, bool) {
		switch v.Kind {
		case KindInt:
			return float64(v.IVal), true
		case KindFloat:
			return v.FVal, true
		}
		return 0, false
	}
	lf, lok := toF(a)
	rf, rok := toF(b)
	return lf, rf, lok && rok
}

// --- Parsing desde string (usado por el loader CSV) ---

// ParseValue intenta inferir el tipo de una cadena CSV.
// Orden de intento: bool → int → float → string.
// Cadena vacía → NULL.
func ParseValue(s string) Value {
	if s == "" {
		return Null()
	}
	sl := strings.ToLower(strings.TrimSpace(s))
	if sl == "null" {
		return Null()
	}
	if sl == "true" {
		return BoolVal(true)
	}
	if sl == "false" {
		return BoolVal(false)
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntVal(i)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatVal(f)
	}
	return StringVal(s)
}

// ParseValueAs parsea una cadena como un tipo declarado explícitamente.
func ParseValueAs(s string, kind Kind) (Value, error) {
	if s == "" {
		return Null(), nil
	}
	sl := strings.ToLower(strings.TrimSpace(s))
	if sl == "null" {
		return Null(), nil
	}
	switch kind {
	case KindInt:
		i, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return Null(), fmt.Errorf("cannot parse %q as int: %w", s, err)
		}
		return IntVal(i), nil
	case KindFloat:
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return Null(), fmt.Errorf("cannot parse %q as float: %w", s, err)
		}
		return FloatVal(f), nil
	case KindBool:
		if sl == "true" || sl == "1" {
			return BoolVal(true), nil
		}
		if sl == "false" || sl == "0" {
			return BoolVal(false), nil
		}
		return Null(), fmt.Errorf("cannot parse %q as bool", s)
	case KindString:
		return StringVal(s), nil
	}
	return Null(), fmt.Errorf("unknown kind %d", kind)
}

// KindFromString convierte un nombre de tipo ("int", "float", "string", "bool") a Kind.
func KindFromString(s string) (Kind, error) {
	switch strings.ToLower(s) {
	case "int", "integer":
		return KindInt, nil
	case "float", "decimal", "double", "real":
		return KindFloat, nil
	case "string", "text", "varchar":
		return KindString, nil
	case "bool", "boolean":
		return KindBool, nil
	}
	return KindNull, fmt.Errorf("unknown type %q", s)
}
