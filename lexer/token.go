// Package lexer define los tipos de tokens del subconjunto SQL soportado.
package lexer

// TokenKind identifica el tipo semántico de un token.
type TokenKind int

const (
	// Literales y nombres
	EOF        TokenKind = iota
	ILLEGAL              // carácter no reconocido
	IDENT                // nombre de tabla/columna
	INT_LIT              // literal entero: 42
	FLOAT_LIT            // literal flotante: 3.14
	STRING_LIT           // literal string: 'hello'

	// Operadores de comparación
	EQ  // =
	NEQ // <>
	LT  // <
	GT  // >
	LTE // <=
	GTE // >=

	// Operadores aritméticos
	PLUS  // +
	MINUS // -
	STAR  // *
	SLASH // /

	// Puntuación
	LPAREN // (
	RPAREN // )
	COMMA  // ,
	DOT    // .
	SEMI   // ;

	// Keywords SQL
	SELECT
	FROM
	WHERE
	AND
	OR
	NOT
	AS
	ORDER
	BY
	ASC
	DESC
	LIMIT
	GROUP
	INNER
	JOIN
	ON
	DISTINCT
	EXPLAIN
	NULL_KW  // NULL keyword
	TRUE_KW  // TRUE keyword
	FALSE_KW // FALSE keyword

	// Funciones de agregación
	COUNT
	SUM
	AVG
	MIN
	MAX

	// Keywords de tipos (para futuros usos)
	INT_KW
	FLOAT_KW
	STRING_KW
	BOOL_KW
)

// keywords mapea las palabras reservadas SQL (en minúsculas) a su TokenKind.
var keywords = map[string]TokenKind{
	"select":   SELECT,
	"from":     FROM,
	"where":    WHERE,
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"as":       AS,
	"order":    ORDER,
	"by":       BY,
	"asc":      ASC,
	"desc":     DESC,
	"limit":    LIMIT,
	"group":    GROUP,
	"inner":    INNER,
	"join":     JOIN,
	"on":       ON,
	"distinct": DISTINCT,
	"explain":  EXPLAIN,
	"null":     NULL_KW,
	"true":     TRUE_KW,
	"false":    FALSE_KW,
	"count":    COUNT,
	"sum":      SUM,
	"avg":      AVG,
	"min":      MIN,
	"max":      MAX,
	"int":      INT_KW,
	"integer":  INT_KW,
	"float":    FLOAT_KW,
	"string":   STRING_KW,
	"bool":     BOOL_KW,
	"boolean":  BOOL_KW,
}

// LookupKeyword retorna el TokenKind de una palabra clave (insensible a mayúsculas),
// o IDENT si no es una palabra reservada.
func LookupKeyword(s string) TokenKind {
	lower := toLower(s)
	if kind, ok := keywords[lower]; ok {
		return kind
	}
	return IDENT
}

// KindString retorna el nombre legible de un TokenKind (para mensajes de error).
func KindString(k TokenKind) string {
	names := map[TokenKind]string{
		EOF: "EOF", ILLEGAL: "ILLEGAL", IDENT: "IDENT",
		INT_LIT: "INT", FLOAT_LIT: "FLOAT", STRING_LIT: "STRING",
		EQ: "=", NEQ: "<>", LT: "<", GT: ">", LTE: "<=", GTE: ">=",
		PLUS: "+", MINUS: "-", STAR: "*", SLASH: "/",
		LPAREN: "(", RPAREN: ")", COMMA: ",", DOT: ".", SEMI: ";",
		SELECT: "SELECT", FROM: "FROM", WHERE: "WHERE",
		AND: "AND", OR: "OR", NOT: "NOT", AS: "AS",
		ORDER: "ORDER", BY: "BY", ASC: "ASC", DESC: "DESC",
		LIMIT: "LIMIT", GROUP: "GROUP",
		INNER: "INNER", JOIN: "JOIN", ON: "ON",
		DISTINCT: "DISTINCT", EXPLAIN: "EXPLAIN",
		NULL_KW: "NULL", TRUE_KW: "TRUE", FALSE_KW: "FALSE",
		COUNT: "COUNT", SUM: "SUM", AVG: "AVG", MIN: "MIN", MAX: "MAX",
	}
	if s, ok := names[k]; ok {
		return s
	}
	return "UNKNOWN"
}

// toLower convierte un string a minúsculas sin usar strings.ToLower para evitar
// importaciones innecesarias dentro del paquete lexer.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// Token es la unidad mínima de información léxica.
type Token struct {
	Kind    TokenKind
	Literal string // texto original del token
	Line    int    // línea (1-indexed)
	Col     int    // columna (1-indexed)
}
