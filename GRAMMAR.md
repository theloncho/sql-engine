# Gramática SQL soportada (EBNF)

Este documento describe formalmente el subconjunto de SQL implementado por el motor.

## Gramática EBNF

```ebnf
(* Consulta completa *)
query       ::= [ "EXPLAIN" ] select_stmt [ ";" ]

(* SELECT *)
select_stmt ::= "SELECT" [ "DISTINCT" ] col_list
                "FROM" table_ref
                { join_clause }
                [ where_clause ]
                [ group_clause ]
                [ order_clause ]
                [ limit_clause ]

(* Columnas del SELECT *)
col_list    ::= "*"
              | col_item { "," col_item }

col_item    ::= "*"
              | expr [ "AS" IDENT ]

(* Referencias de tabla *)
table_ref   ::= IDENT [ "AS" IDENT ]

(* JOIN *)
join_clause ::= "INNER" "JOIN" table_ref "ON" expr

(* Cláusulas *)
where_clause  ::= "WHERE" expr
group_clause  ::= "GROUP" "BY" expr_list
order_clause  ::= "ORDER" "BY" order_item { "," order_item }
limit_clause  ::= "LIMIT" INT_LIT

order_item  ::= expr [ "ASC" | "DESC" ]
expr_list   ::= expr { "," expr }

(* Expresiones — jerarquía de precedencia *)
expr        ::= or_expr

or_expr     ::= and_expr { "OR" and_expr }

and_expr    ::= not_expr { "AND" not_expr }

not_expr    ::= "NOT" not_expr
              | cmp_expr

cmp_expr    ::= add_expr
              | add_expr cmp_op add_expr

cmp_op      ::= "=" | "<>" | "<" | ">" | "<=" | ">="

add_expr    ::= mul_expr { ( "+" | "-" ) mul_expr }

mul_expr    ::= unary_expr { ( "*" | "/" ) unary_expr }

unary_expr  ::= "-" unary_expr
              | primary

primary     ::= INT_LIT
              | FLOAT_LIT
              | STRING_LIT
              | "NULL"
              | "TRUE"
              | "FALSE"
              | IDENT "." IDENT        (* referencia calificada: tabla.columna *)
              | IDENT                  (* referencia simple: columna o función futura *)
              | agg_func
              | "(" expr ")"

agg_func    ::= agg_name "(" "*" ")"      (* COUNT(*) *)
              | agg_name "(" expr ")"

agg_name    ::= "COUNT" | "SUM" | "AVG" | "MIN" | "MAX"

(* Tokens terminales *)
INT_LIT     ::= ["-"] digit { digit }
FLOAT_LIT   ::= ["-"] digit { digit } "." digit { digit }
STRING_LIT  ::= "'" { char | "''" } "'"    (* '' es escape para comilla simple *)
IDENT       ::= letter { letter | digit | "_" }

letter      ::= "a"..."z" | "A"..."Z"
digit       ::= "0"..."9"
```

## Keywords reservadas (case-insensitive)

```
SELECT  FROM    WHERE   AND     OR      NOT     AS
ORDER   BY      ASC     DESC    LIMIT   GROUP   INNER
JOIN    ON      DISTINCT EXPLAIN NULL   TRUE    FALSE
COUNT   SUM     AVG     MIN     MAX
INT     INTEGER FLOAT   DECIMAL DOUBLE  REAL    STRING
TEXT    VARCHAR BOOL    BOOLEAN
```

## Tipos de datos

| Tipo SQL | Aliases | Tipo Go interno |
|----------|---------|-----------------|
| `INT` | `INTEGER` | `int64` |
| `FLOAT` | `DECIMAL`, `DOUBLE`, `REAL` | `float64` |
| `STRING` | `TEXT`, `VARCHAR` | `string` |
| `BOOL` | `BOOLEAN` | `bool` |
| `NULL` | — | valor especial |

## Reglas de NULL

- Las celdas CSV vacías se convierten a `NULL`.
- `NULL = NULL` evalúa a UNKNOWN (no TRUE).
- `NULL` en comparaciones produce UNKNOWN → la fila **no pasa** el filtro WHERE.
- `NULL` en `AND`/`OR` sigue la lógica de tres valores SQL.
- `SUM`, `AVG`, `MIN`, `MAX` ignoran valores `NULL`.
- `COUNT(*)` cuenta todas las filas; `COUNT(col)` ignora NULLs en `col`.
- `ORDER BY` coloca los NULLs al final (NULLS LAST), tanto en ASC como en DESC.

## Operadores aritméticos

| Operador | Significado | Tipos válidos |
|----------|-------------|---------------|
| `+` | Suma | INT, FLOAT |
| `-` | Resta / negación unaria | INT, FLOAT |
| `*` | Multiplicación | INT, FLOAT |
| `/` | División real | INT, FLOAT |

**Promoción numérica**: si un operando es FLOAT y el otro INT, ambos se promueven a FLOAT.
**División entera**: `INT / INT` que no genere fracción devuelve INT.

## Limitaciones del subconjunto

- No se soportan subconsultas (`SELECT ... FROM (SELECT ...)`).
- No se soporta `LEFT`/`RIGHT`/`FULL OUTER JOIN`.
- No se soporta `HAVING`.
- No se soportan funciones de ventana (`OVER`).
- No se soporta `INSERT`, `UPDATE`, `DELETE` (motor de solo lectura).
- No hay índices ni estadísticas; el planner es básico.
