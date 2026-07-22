package lexer

import "testing"

func TestLexer_Keywords(t *testing.T) {
	tests := []struct {
		input string
		want  []TokenKind
	}{
		{"SELECT", []TokenKind{SELECT, EOF}},
		{"select FROM where", []TokenKind{SELECT, FROM, WHERE, EOF}},
		{"ORDER BY", []TokenKind{ORDER, BY, EOF}},
		{"GROUP BY", []TokenKind{GROUP, BY, EOF}},
		{"INNER JOIN ON", []TokenKind{INNER, JOIN, ON, EOF}},
		{"COUNT SUM AVG MIN MAX", []TokenKind{COUNT, SUM, AVG, MIN, MAX, EOF}},
		{"DISTINCT EXPLAIN LIMIT", []TokenKind{DISTINCT, EXPLAIN, LIMIT, EOF}},
		{"NULL TRUE FALSE", []TokenKind{NULL_KW, TRUE_KW, FALSE_KW, EOF}},
		{"ASC DESC", []TokenKind{ASC, DESC, EOF}},
	}
	for _, tt := range tests {
		l := New(tt.input)
		if len(l.Errors) > 0 {
			t.Errorf("input %q: unexpected errors: %v", tt.input, l.Errors)
		}
		if len(l.Tokens) != len(tt.want) {
			t.Errorf("input %q: token count = %d, want %d", tt.input, len(l.Tokens), len(tt.want))
			continue
		}
		for i, want := range tt.want {
			if l.Tokens[i].Kind != want {
				t.Errorf("input %q token[%d]: kind = %s, want %s",
					tt.input, i, KindString(l.Tokens[i].Kind), KindString(want))
			}
		}
	}
}

func TestLexer_Operators(t *testing.T) {
	tests := []struct {
		input string
		want  []TokenKind
	}{
		{"= <> < > <= >=", []TokenKind{EQ, NEQ, LT, GT, LTE, GTE, EOF}},
		{"+ - * /", []TokenKind{PLUS, MINUS, STAR, SLASH, EOF}},
		{"( ) , . ;", []TokenKind{LPAREN, RPAREN, COMMA, DOT, SEMI, EOF}},
	}
	for _, tt := range tests {
		l := New(tt.input)
		if len(l.Tokens) != len(tt.want) {
			t.Errorf("input %q: token count = %d, want %d", tt.input, len(l.Tokens), len(tt.want))
			continue
		}
		for i, want := range tt.want {
			if l.Tokens[i].Kind != want {
				t.Errorf("input %q token[%d]: kind = %s, want %s",
					tt.input, i, KindString(l.Tokens[i].Kind), KindString(want))
			}
		}
	}
}

func TestLexer_Literals(t *testing.T) {
	l := New("42 3.14 'hello world' 'it''s'")
	want := []struct {
		kind    TokenKind
		literal string
	}{
		{INT_LIT, "42"},
		{FLOAT_LIT, "3.14"},
		{STRING_LIT, "hello world"},
		{STRING_LIT, "it's"}, // escape ''
		{EOF, ""},
	}
	if len(l.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", l.Errors)
	}
	for i, w := range want {
		if i >= len(l.Tokens) {
			t.Fatalf("missing token[%d]", i)
		}
		if l.Tokens[i].Kind != w.kind {
			t.Errorf("token[%d] kind = %s, want %s", i, KindString(l.Tokens[i].Kind), KindString(w.kind))
		}
		if l.Tokens[i].Literal != w.literal {
			t.Errorf("token[%d] literal = %q, want %q", i, l.Tokens[i].Literal, w.literal)
		}
	}
}

func TestLexer_Position(t *testing.T) {
	l := New("SELECT\nFROM")
	if l.Tokens[0].Line != 1 || l.Tokens[0].Col != 1 {
		t.Errorf("SELECT pos = (%d,%d), want (1,1)", l.Tokens[0].Line, l.Tokens[0].Col)
	}
	if l.Tokens[1].Line != 2 || l.Tokens[1].Col != 1 {
		t.Errorf("FROM pos = (%d,%d), want (2,1)", l.Tokens[1].Line, l.Tokens[1].Col)
	}
}

func TestLexer_Comments(t *testing.T) {
	l := New("SELECT -- this is a comment\n42")
	if len(l.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", l.Errors)
	}
	if l.Tokens[0].Kind != SELECT {
		t.Error("expected SELECT")
	}
	if l.Tokens[1].Kind != INT_LIT || l.Tokens[1].Literal != "42" {
		t.Error("expected INT_LIT 42")
	}
}

func TestLexer_IllegalChar(t *testing.T) {
	l := New("SELECT @ name")
	if len(l.Errors) == 0 {
		t.Error("expected error for illegal char @")
	}
}

func TestLexer_UnterminatedString(t *testing.T) {
	l := New("'unterminated")
	if len(l.Errors) == 0 {
		t.Error("expected error for unterminated string")
	}
}

func TestLexer_FullQuery(t *testing.T) {
	q := "SELECT id, name FROM employees WHERE salary > 50000 AND active = TRUE"
	l := New(q)
	if len(l.Errors) > 0 {
		t.Fatalf("errors: %v", l.Errors)
	}
	// Solo verificamos que los primeros tokens son correctos.
	expected := []TokenKind{SELECT, IDENT, COMMA, IDENT, FROM, IDENT, WHERE, IDENT, GT, INT_LIT, AND, IDENT, EQ, TRUE_KW, EOF}
	if len(l.Tokens) != len(expected) {
		t.Fatalf("token count = %d, want %d", len(l.Tokens), len(expected))
	}
	for i, want := range expected {
		if l.Tokens[i].Kind != want {
			t.Errorf("token[%d] = %s, want %s", i, KindString(l.Tokens[i].Kind), KindString(want))
		}
	}
}
