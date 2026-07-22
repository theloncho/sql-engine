// Package lexer implementa el tokenizador (analizador léxico) del subconjunto SQL.
// El lexer es un autómata determinista de estados: avanza carácter a carácter
// y agrupa los caracteres en tokens con posición (línea, columna).
package lexer

import (
	"fmt"
	"strings"
)

// Lexer mantiene el estado del tokenizador.
type Lexer struct {
	input  []rune // entrada completa como runes (soporte Unicode en identificadores)
	pos    int    // posición actual en input
	line   int    // línea actual (1-indexed)
	col    int    // columna actual (1-indexed)
	Tokens []Token
	Errors []string
}

// New crea un nuevo Lexer y tokeniza toda la entrada de inmediato.
// Los errores léxicos se acumulan en Errors (no detiene el tokenizado).
func New(input string) *Lexer {
	l := &Lexer{
		input: []rune(input),
		line:  1,
		col:   1,
	}
	l.tokenize()
	return l
}

// tokenize recorre toda la entrada y llena l.Tokens.
func (l *Lexer) tokenize() {
	for {
		l.skipWhitespaceAndComments()
		if l.pos >= len(l.input) {
			l.Tokens = append(l.Tokens, Token{Kind: EOF, Line: l.line, Col: l.col})
			return
		}

		startLine, startCol := l.line, l.col
		ch := l.current()

		switch {
		case isLetter(ch) || ch == '_':
			tok := l.readIdentOrKeyword(startLine, startCol)
			l.Tokens = append(l.Tokens, tok)

		case isDigit(ch):
			tok := l.readNumber(startLine, startCol)
			l.Tokens = append(l.Tokens, tok)

		case ch == '\'':
			tok, err := l.readString(startLine, startCol)
			if err != nil {
				l.Errors = append(l.Errors, err.Error())
			}
			l.Tokens = append(l.Tokens, tok)

		case ch == '=':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: EQ, Literal: "=", Line: startLine, Col: startCol})

		case ch == '<':
			l.advance()
			if l.pos < len(l.input) && l.current() == '>' {
				l.advance()
				l.Tokens = append(l.Tokens, Token{Kind: NEQ, Literal: "<>", Line: startLine, Col: startCol})
			} else if l.pos < len(l.input) && l.current() == '=' {
				l.advance()
				l.Tokens = append(l.Tokens, Token{Kind: LTE, Literal: "<=", Line: startLine, Col: startCol})
			} else {
				l.Tokens = append(l.Tokens, Token{Kind: LT, Literal: "<", Line: startLine, Col: startCol})
			}

		case ch == '>':
			l.advance()
			if l.pos < len(l.input) && l.current() == '=' {
				l.advance()
				l.Tokens = append(l.Tokens, Token{Kind: GTE, Literal: ">=", Line: startLine, Col: startCol})
			} else {
				l.Tokens = append(l.Tokens, Token{Kind: GT, Literal: ">", Line: startLine, Col: startCol})
			}

		case ch == '+':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: PLUS, Literal: "+", Line: startLine, Col: startCol})

		case ch == '-':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: MINUS, Literal: "-", Line: startLine, Col: startCol})

		case ch == '*':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: STAR, Literal: "*", Line: startLine, Col: startCol})

		case ch == '/':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: SLASH, Literal: "/", Line: startLine, Col: startCol})

		case ch == '(':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: LPAREN, Literal: "(", Line: startLine, Col: startCol})

		case ch == ')':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: RPAREN, Literal: ")", Line: startLine, Col: startCol})

		case ch == ',':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: COMMA, Literal: ",", Line: startLine, Col: startCol})

		case ch == '.':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: DOT, Literal: ".", Line: startLine, Col: startCol})

		case ch == ';':
			l.advance()
			l.Tokens = append(l.Tokens, Token{Kind: SEMI, Literal: ";", Line: startLine, Col: startCol})

		default:
			l.Errors = append(l.Errors, fmt.Sprintf("illegal character %q at line %d, col %d", ch, l.line, l.col))
			l.Tokens = append(l.Tokens, Token{Kind: ILLEGAL, Literal: string(ch), Line: startLine, Col: startCol})
			l.advance()
		}
	}
}

// --- Métodos auxiliares del tokenizador ---

func (l *Lexer) current() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) advance() {
	if l.pos >= len(l.input) {
		return
	}
	if l.input[l.pos] == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	l.pos++
}

// skipWhitespaceAndComments salta espacios, tabulaciones, saltos de línea,
// y comentarios de línea (-- ...) y de bloque (/* ... */).
func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.input) {
		ch := l.current()
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
			continue
		}
		// Comentario de línea: -- hasta fin de línea.
		if ch == '-' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '-' {
			for l.pos < len(l.input) && l.current() != '\n' {
				l.advance()
			}
			continue
		}
		// Comentario de bloque: /* ... */
		if ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '*' {
			l.advance()
			l.advance()
			for l.pos < len(l.input) {
				if l.current() == '*' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '/' {
					l.advance()
					l.advance()
					break
				}
				l.advance()
			}
			continue
		}
		break
	}
}

// readIdentOrKeyword lee un identificador o palabra clave.
func (l *Lexer) readIdentOrKeyword(startLine, startCol int) Token {
	var sb strings.Builder
	for l.pos < len(l.input) && (isLetter(l.current()) || isDigit(l.current()) || l.current() == '_') {
		sb.WriteRune(l.current())
		l.advance()
	}
	lit := sb.String()
	kind := LookupKeyword(lit)
	return Token{Kind: kind, Literal: lit, Line: startLine, Col: startCol}
}

// readNumber lee un entero o flotante.
func (l *Lexer) readNumber(startLine, startCol int) Token {
	var sb strings.Builder
	isFloat := false
	for l.pos < len(l.input) && (isDigit(l.current()) || l.current() == '.') {
		if l.current() == '.' {
			if isFloat {
				break // segundo punto: stop
			}
			isFloat = true
		}
		sb.WriteRune(l.current())
		l.advance()
	}
	kind := INT_LIT
	if isFloat {
		kind = FLOAT_LIT
	}
	return Token{Kind: kind, Literal: sb.String(), Line: startLine, Col: startCol}
}

// readString lee un string delimitado por comillas simples ('...').
// Soporta escape de comilla doble ('': representa una comilla simple).
func (l *Lexer) readString(startLine, startCol int) (Token, error) {
	l.advance() // consumir la ' inicial
	var sb strings.Builder
	for {
		if l.pos >= len(l.input) {
			return Token{Kind: ILLEGAL, Line: startLine, Col: startCol},
				fmt.Errorf("unterminated string literal at line %d, col %d", startLine, startCol)
		}
		ch := l.current()
		if ch == '\'' {
			l.advance()
			// ¿Escape ''? → produce una comilla simple en el resultado.
			if l.pos < len(l.input) && l.current() == '\'' {
				sb.WriteRune('\'')
				l.advance()
				continue
			}
			break // fin del string
		}
		sb.WriteRune(ch)
		l.advance()
	}
	return Token{Kind: STRING_LIT, Literal: sb.String(), Line: startLine, Col: startCol}, nil
}

// isLetter reporta si ch es una letra ASCII o underscore.
func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// isDigit reporta si ch es un dígito ASCII.
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}
