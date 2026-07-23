# Motor de Consultas SQL en Memoria

Motor SQL completo escrito en Go que implementa el **modelo Volcano (iteradores)** para ejecutar consultas SQL sobre datos cargados desde archivos CSV.

## Compilación y ejecución

```bash
# Compilar el proyecto completo
go build ./...

# Ejecutar la suite completa de pruebas unitarias
go test -v ./...

# Ejecutar el REPL interactivo
go run . -load data/employees.csv,data/departments.csv

# Ejecutar una consulta directa
go run . -load data/employees.csv -q "SELECT * FROM employees LIMIT 5"

# Ver el plan de ejecución (EXPLAIN)
go run . -load data/employees.csv -q "SELECT dept_id, COUNT(*) FROM employees GROUP BY dept_id" -explain
```

## Estructura del proyecto

```
sql-engine/
├── main.go             # CLI + punto de entrada
├── data/               # Archivos CSV de ejemplo
│   ├── employees.csv
│   ├── departments.csv
│   └── products.csv
├── types/              # Sistema de tipos: Int, Float, String, Bool, NULL
├── catalog/            # Catálogo de tablas en memoria
├── loader/             # Cargador CSV con inferencia de tipos
├── lexer/              # Tokenizador SQL con posición de errores
├── parser/             # Parser descendente recursivo → AST
├── executor/           # Operadores Volcano: Scan, Filter, Project, Sort, Limit, Aggregate, Join, Distinct
├── planner/            # Construye el árbol de operadores desde el AST
├── explain/            # EXPLAIN: imprime el árbol de operadores
└── repl/               # REPL interactivo + printer ASCII
```

## SQL soportado (resumen)

```sql
SELECT [DISTINCT] col1, col2, agg(col) [AS alias], ...
FROM tabla [AS alias]
[INNER JOIN tabla2 [AS alias2] ON condición]
[WHERE expresión]
[GROUP BY col1, col2, ...]
[ORDER BY col1 [ASC|DESC], ...]
[LIMIT n]
```

Ver [GRAMMAR.md](GRAMMAR.md) para la gramática EBNF completa.

## Hitos implementados

| Hito | Descripción | Estado |
|------|-------------|--------|
| H1 | Cargador CSV, catálogo, inferencia de tipos | ✅ |
| H2 | Lexer + Parser → AST con errores posicionados | ✅ |
| H3 | Operadores Scan/Filter/Project + REPL | ✅ |
| H4 | ORDER BY, LIMIT, GROUP BY, agregaciones, NULLs | ✅ |
| H5 | INNER JOIN (NestedLoop + Hash), DISTINCT, EXPLAIN | ✅ |

## Ejemplos de consultas

```sql
-- Básico
SELECT * FROM employees;

-- WHERE con tipos y NULL
SELECT name, salary FROM employees WHERE salary > 80000 AND active = TRUE;

-- ORDER BY con NULLS LAST
SELECT name, salary FROM employees ORDER BY salary DESC;

-- GROUP BY con agregaciones
SELECT dept_id, COUNT(*), AVG(salary), MAX(salary) FROM employees GROUP BY dept_id;

-- INNER JOIN (usa HashJoin automáticamente para equi-joins)
SELECT e.name, d.name FROM employees AS e INNER JOIN departments AS d ON e.dept_id = d.id;

-- DISTINCT
SELECT DISTINCT dept_id FROM employees;

-- LIMIT
SELECT * FROM products ORDER BY price DESC LIMIT 3;

-- EXPLAIN: ver el árbol sin ejecutar
EXPLAIN SELECT dept_id, COUNT(*) FROM employees GROUP BY dept_id;
```

## Comandos del REPL

| Comando | Descripción |
|---------|-------------|
| `SELECT ...` | Ejecutar consulta SQL |
| `EXPLAIN SELECT ...` | Mostrar árbol de operadores |
| `load <archivo.csv>` | Cargar CSV como tabla |
| `\d` / `tables` | Listar tablas del catálogo |
| `help` | Mostrar ayuda |
| `quit` / `exit` | Salir |

## Carga de CSV con tipos declarados

Las columnas pueden declarar su tipo en el encabezado:

```csv
id:int,name:string,salary:float,active:bool
1,Alice,75000.00,true
```

Sin declaración, los tipos se infieren automáticamente. Las celdas vacías se convierten a `NULL`.

## Diseño del modelo Volcano

La interfaz central es:

```go
type Operator interface {
    Next()   (Row, error)  // siguiente fila, o (nil, nil) si EOF
    Close()  error         // liberar recursos
    Schema() OutputSchema  // columnas de salida
}
```

Los operadores se componen en árbol:
```
Project [name, dept]
  └─ Sort (ORDER BY salary DESC)
       └─ Filter (WHERE active = TRUE)
            └─ TableScan(employees)
```

Agregar un operador nuevo **no requiere modificar ningún operador existente** (principio abierto/cerrado).

## Automatización con Makefile

Si dispones de la herramienta `make`, puedes utilizar los siguientes atajos:

```bash
make build    # Compila el ejecutable sql-engine
make test     # Ejecuta la suite de pruebas unitarias e integración
make run      # Compila y ejecuta el motor con datos de ejemplo
make clean    # Limpia los binarios generados
```

