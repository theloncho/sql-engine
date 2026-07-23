// Package parser implementa el analizador sintáctico por descenso recursivo.
// Transforma una lista de tokens (producida por el lexer) en un AST (*SelectStmt).
//
// La gramática soportada está documentada en GRAMMAR.md.
// Los errores de sintaxis incluyen posición (línea:columna) para mensajes útiles.
// Package parser handles SQL parsing logic.
// This file processes tokens and builds the AST.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/theloncho/sql-engine/lexer"
)

// ParseError representa un error de sintaxis con posición.
type ParseError struct {
	Message string
	Line    int
	Col     int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("syntax error at line %d, col %d: %s", e.Line, e.Col, e.Message)
}

// Parser mantiene el estado del análisis sintáctico.
type Parser struct {
	tokens []lexer.Token
	pos    int // posición del token actual
}

// New crea un parser a partir de los tokens del lexer.
// Omite tokens ILLEGAL (sus errores ya están en l.Errors).
func New(tokens []lexer.Token) *Parser {
	filtered := make([]lexer.Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Kind != lexer.ILLEGAL {
			filtered = append(filtered, t)
		}
	}
	return &Parser{tokens: filtered}
}

// Parse es el punto de entrada principal: parsea una consulta SELECT completa.
func Parse(input string) (*SelectStmt, error) {
	l := lexer.New(input)
	if len(l.Errors) > 0 {
		return nil, fmt.Errorf("lexer errors: %s", strings.Join(l.Errors, "; "))
	}
	p := New(l.Tokens)
	return p.parseSelect()
}

// --- Navegación por tokens ---

func (p *Parser) current() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peek() lexer.Token {
	if p.pos+1 >= len(p.tokens) {
		return lexer.Token{Kind: lexer.EOF}
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) advance() lexer.Token {
	t := p.current()
	p.pos++
	return t
}

func (p *Parser) expect(kind lexer.TokenKind) (lexer.Token, error) {
	t := p.current()
	if t.Kind != kind {
		return t, &ParseError{
			Message: fmt.Sprintf("expected %s, got %s (%q)", lexer.KindString(kind), lexer.KindString(t.Kind), t.Literal),
			Line:    t.Line,
			Col:     t.Col,
		}
	}
	return p.advance(), nil
}

func (p *Parser) check(kinds ...lexer.TokenKind) bool {
	cur := p.current().Kind
	for _, k := range kinds {
		if cur == k {
			return true
		}
	}
	return false
}

func (p *Parser) match(kinds ...lexer.TokenKind) bool {
	if p.check(kinds...) {
		p.advance()
		return true
	}
	return false
}

// --- parseSelect: punto de entrada ---

func (p *Parser) parseSelect() (*SelectStmt, error) {
	stmt := &SelectStmt{}

	// EXPLAIN (opcional)
	explain := false
	if p.current().Kind == lexer.EXPLAIN {
		p.advance()
		explain = true
		_ = explain // el planner lo usará
	}

	if _, err := p.expect(lexer.SELECT); err != nil {
		return nil, err
	}

	// DISTINCT
	if p.current().Kind == lexer.DISTINCT {
		p.advance()
		stmt.Distinct = true
	}

	// Columnas
	cols, err := p.parseSelectCols()
	if err != nil {
		return nil, err
	}
	stmt.Cols = cols

	// FROM
	if _, err := p.expect(lexer.FROM); err != nil {
		return nil, err
	}
	from, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	stmt.From = from

	// JOINs
	for p.current().Kind == lexer.INNER {
		j, err := p.parseJoin()
		if err != nil {
			return nil, err
		}
		stmt.Joins = append(stmt.Joins, *j)
	}

	// WHERE
	if p.current().Kind == lexer.WHERE {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Where = expr
	}

	// GROUP BY
	if p.current().Kind == lexer.GROUP {
		p.advance()
		if _, err := p.expect(lexer.BY); err != nil {
			return nil, err
		}
		groups, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		stmt.GroupBy = groups
	}

	// ORDER BY
	if p.current().Kind == lexer.ORDER {
		p.advance()
		if _, err := p.expect(lexer.BY); err != nil {
			return nil, err
		}
		orders, err := p.parseOrderItems()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orders
	}

	// LIMIT
	if p.current().Kind == lexer.LIMIT {
		p.advance()
		tok, err := p.expect(lexer.INT_LIT)
		if err != nil {
			return nil, err
		}
		n, err := strconv.ParseInt(tok.Literal, 10, 64)
		if err != nil {
			return nil, &ParseError{Message: "invalid LIMIT value", Line: tok.Line, Col: tok.Col}
		}
		stmt.Limit = &n
	}

	// Opcional: ; al final
	if p.current().Kind == lexer.SEMI {
		p.advance()
	}

	// Verificar que consumimos todo
	if p.current().Kind != lexer.EOF {
		t := p.current()
		return nil, &ParseError{
			Message: fmt.Sprintf("unexpected token %s (%q)", lexer.KindString(t.Kind), t.Literal),
			Line:    t.Line, Col: t.Col,
		}
	}

	// Nota: la bandera EXPLAIN se propaga al planner a través de BuildExplain().
	_ = explain

	return stmt, nil
}

// parseSelectCols parsea la lista de columnas después de SELECT.
func (p *Parser) parseSelectCols() ([]SelectCol, error) {
	// ¿Es SELECT * ?
	if p.current().Kind == lexer.STAR {
		p.advance()
		return []SelectCol{{IsWildcard: true}}, nil
	}
	var cols []SelectCol
	for {
		col, err := p.parseSelectCol()
		if err != nil {
			return nil, err
		}
		cols = append(cols, col)
		if p.current().Kind != lexer.COMMA {
			break
		}
		p.advance() // consumir ','
	}
	return cols, nil
}

func (p *Parser) parseSelectCol() (SelectCol, error) {
	if p.current().Kind == lexer.STAR {
		p.advance()
		return SelectCol{IsWildcard: true}, nil
	}
	expr, err := p.parseExpr()
	if err != nil {
		return SelectCol{}, err
	}
	col := SelectCol{Expr: expr}
	if p.current().Kind == lexer.AS {
		p.advance()
		alias, err := p.expect(lexer.IDENT)
		if err != nil {
			return SelectCol{}, err
		}
		col.Alias = alias.Literal
	}
	return col, nil
}

// parseTableRef parsea "nombre [AS alias]".
func (p *Parser) parseTableRef() (*TableRef, error) {
	name, err := p.expect(lexer.IDENT)
	if err != nil {
		return nil, err
	}
	ref := &TableRef{Name: name.Literal}
	if p.current().Kind == lexer.AS {
		p.advance()
		alias, err := p.expect(lexer.IDENT)
		if err != nil {
			return nil, err
		}
		ref.Alias = alias.Literal
	}
	return ref, nil
}

// parseJoin parsea "INNER JOIN tabla ON expr".
func (p *Parser) parseJoin() (*JoinClause, error) {
	if _, err := p.expect(lexer.INNER); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.JOIN); err != nil {
		return nil, err
	}
	tbl, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.ON); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &JoinClause{Table: tbl, On: cond}, nil
}

// parseOrderItems parsea "expr [ASC|DESC] {, expr [ASC|DESC]}".
func (p *Parser) parseOrderItems() ([]OrderItem, error) {
	var items []OrderItem
	for {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		asc := true
		if p.current().Kind == lexer.DESC {
			p.advance()
			asc = false
		} else if p.current().Kind == lexer.ASC {
			p.advance()
		}
		items = append(items, OrderItem{Expr: expr, Asc: asc})
		if p.current().Kind != lexer.COMMA {
			break
		}
		p.advance()
	}
	return items, nil
}

// parseExprList parsea una lista de expresiones separadas por coma.
func (p *Parser) parseExprList() ([]Expr, error) {
	var exprs []Expr
	for {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, e)
		if p.current().Kind != lexer.COMMA {
			break
		}
		p.advance()
	}
	return exprs, nil
}

// --- Parsing de expresiones: descenso recursivo con precedencia ---
//
// Jerarquía de precedencia (menor a mayor):
//   OR → AND → NOT → comparación → suma → multiplicación → unario → primario

func (p *Parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.current().Kind == lexer.OR {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "OR", Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.current().Kind == lexer.AND {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "AND", Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (Expr, error) {
	if p.current().Kind == lexer.NOT {
		p.advance()
		expr, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "NOT", Expr: expr}, nil
	}
	return p.parseCmp()
}

func (p *Parser) parseCmp() (Expr, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	ops := map[lexer.TokenKind]string{
		lexer.EQ: "=", lexer.NEQ: "<>",
		lexer.LT: "<", lexer.GT: ">",
		lexer.LTE: "<=", lexer.GTE: ">=",
	}
	if op, ok := ops[p.current().Kind]; ok {
		p.advance()
		right, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: op, Right: right}, nil
	}
	return left, nil
}

func (p *Parser) parseAdd() (Expr, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.check(lexer.PLUS, lexer.MINUS) {
		op := p.advance().Literal
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: op, Right: right}
	}
	return left, nil
}

func (p *Parser) parseMul() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.check(lexer.STAR, lexer.SLASH) {
		op := p.advance().Literal
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: op, Right: right}
	}
	return left, nil
}

func (p *Parser) parseUnary() (Expr, error) {
	if p.current().Kind == lexer.MINUS {
		p.advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "-", Expr: expr}, nil
	}
	return p.parsePrimary()
}

// parsePrimary parsea el nivel más bajo: literales, identificadores, funciones, subexpresiones.
func (p *Parser) parsePrimary() (Expr, error) {
	t := p.current()
	switch t.Kind {
	case lexer.INT_LIT:
		p.advance()
		v, err := strconv.ParseInt(t.Literal, 10, 64)
		if err != nil {
			return nil, &ParseError{Message: "invalid integer", Line: t.Line, Col: t.Col}
		}
		return &IntLiteral{Value: v}, nil

	case lexer.FLOAT_LIT:
		p.advance()
		v, err := strconv.ParseFloat(t.Literal, 64)
		if err != nil {
			return nil, &ParseError{Message: "invalid float", Line: t.Line, Col: t.Col}
		}
		return &FloatLiteral{Value: v}, nil

	case lexer.STRING_LIT:
		p.advance()
		return &StringLiteral{Value: t.Literal}, nil

	case lexer.NULL_KW:
		p.advance()
		return &NullLiteral{}, nil

	case lexer.TRUE_KW:
		p.advance()
		return &BoolLiteral{Value: true}, nil

	case lexer.FALSE_KW:
		p.advance()
		return &BoolLiteral{Value: false}, nil

	case lexer.COUNT, lexer.SUM, lexer.AVG, lexer.MIN, lexer.MAX:
		return p.parseAggFunc()

	case lexer.IDENT:
		p.advance()
		// ¿Es "tabla.columna"?
		if p.current().Kind == lexer.DOT {
			p.advance()
			col, err := p.expect(lexer.IDENT)
			if err != nil {
				return nil, err
			}
			return &Identifier{Table: t.Literal, Column: col.Literal}, nil
		}
		return &Identifier{Column: t.Literal}, nil

	case lexer.LPAREN:
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.RPAREN); err != nil {
			return nil, err
		}
		return expr, nil

	default:
		return nil, &ParseError{
			Message: fmt.Sprintf("unexpected token %s (%q)", lexer.KindString(t.Kind), t.Literal),
			Line:    t.Line, Col: t.Col,
		}
	}
}

// parseAggFunc parsea "AGG(* | expr)".
func (p *Parser) parseAggFunc() (Expr, error) {
	nameToken := p.advance()
	name := strings.ToUpper(nameToken.Literal)
	if _, err := p.expect(lexer.LPAREN); err != nil {
		return nil, err
	}
	agg := &AggFunc{Name: name}
	if p.current().Kind == lexer.STAR {
		p.advance()
		agg.IsStar = true
	} else {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		agg.Arg = expr
	}
	if _, err := p.expect(lexer.RPAREN); err != nil {
		return nil, err
	}
	return agg, nil
}
