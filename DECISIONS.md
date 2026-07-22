# Bitácora de Decisiones de Diseño

Documento que registra las decisiones de diseño relevantes tomadas durante el desarrollo del motor SQL en memoria.

---

## H1 — Cargador CSV y Sistema de Tipos

### Decisión 1: Tipo `Value` como struct con campos directos (no `interface{}`)

**¿Qué se decidió?**
Representar los valores SQL con una struct `Value{Kind, IVal, FVal, SVal, BVal}` en lugar de usar `interface{}` como campo único.

**Opciones evaluadas:**
- `interface{}` con type assertions en cada operación.
- Struct con un campo de cada tipo + discriminador `Kind`.

**Justificación:**
La struct con discriminador evita allocaciones de heap para enteros y flotantes (Go permite que los valores pequeños vivan en la pila si no escapan). Además, hace los type assertions explícitos en el código del evaluador, mejorando la legibilidad y el rendimiento sin reflexión.

**Consulta de IA:** Pregunté sobre el trade-off entre `interface{}` y structs discriminadas para representar tipos algebraicos en Go. Usé la respuesta para confirmar la decisión que ya había considerado; la implementación final es propia.

---

### Decisión 2: Celdas CSV vacías → NULL

**¿Qué se decidió?**
Las celdas vacías en CSV se convierten automáticamente a `NULL`.

**Justificación:**
SQL trata la ausencia de dato como NULL. Un string vacío `""` es un string válido; la ausencia de dato (celda vacía) es semánticamente diferente. El data set de ejemplo (Dave sin salario) lo demuestra.

---

### Decisión 3: Inferencia de tipos desde la primera fila de datos

**¿Qué se decidió?**
El cargador infiere el tipo de cada columna del primer valor no-NULL de esa columna.

**Opciones evaluadas:**
- Inferir desde todas las filas (más robusto pero O(n) en preproceso).
- Inferir desde la primera fila no-NULL (O(k) donde k = posición del primer no-NULL).
- Requerir declaración explícita.

**Justificación:**
El subconjunto especificado acepta ambas formas (encabezado `col:tipo` o inferencia). La inferencia desde la primera fila no-NULL es el equilibrio entre simplicidad y robustez para datos de prueba homogéneos.

---

## H2 — Lexer y Parser

### Decisión 4: Keywords case-insensitive, identificadores case-sensitive

**¿Qué se decidió?**
Las palabras reservadas SQL son case-insensitive (`SELECT` = `select`). Los nombres de tabla y columna son case-sensitive.

**Justificación:**
Sigue el comportamiento estándar de la mayoría de bases de datos. La insensibilidad de keywords es una convención SQL; la sensibilidad de identificadores permite tablas `Employee` y `employee` coexistir (relevante si el usuario carga CSVs con distintas convenciones de naming).

---

### Decisión 5: Parser por descenso recursivo (no Pratt)

**¿Qué se decidió?**
Implementar el parser como descenso recursivo clásico con una función por nivel de precedencia.

**Opciones evaluadas:**
- Pratt parser (precedencia dirigida por tabla).
- Descenso recursivo con gramática estratificada.

**Justificación:**
Para el subconjunto SQL especificado (sin muchos operadores infijos de distintas precedencias), el descenso recursivo es más legible y directo. Cada función del parser (`parseOr`, `parseAnd`, `parseCmp`, `parseAdd`, `parseMul`) corresponde directamente a un nivel de la gramática.

---

## H3 — Modelo Volcano

### Decisión 6: Interfaz Operator con Next() + Close() + Schema()

**¿Qué se decidió?**
La interfaz `Operator` tiene tres métodos: `Next() (Row, error)`, `Close() error`, y `Schema() OutputSchema`.

**Opciones evaluadas:**
- Solo `Next()` (sin Schema ni Close).
- `Next()` + `Schema()` (sin Close).
- Los tres métodos.

**Justificación:**
- `Next()`: el corazón del modelo iterador.
- `Close()`: necesario para liberar recursos y resetear el estado. Permite reutilizar operadores (p. ej. NestedLoopJoin reinicia el operador inner).
- `Schema()`: permite que los operadores superiores resuelvan nombres de columnas sin requerir información externa. Hace al árbol auto-descriptivo.

**Consulta de IA:** Discutí si incluir Schema en la interfaz o pasarlo por constructor. La discusión confirmó que incluirlo en la interfaz simplifica el planner. Decisión y implementación propias.

---

### Decisión 7: OutputSchema con calificadores de tabla

**¿Qué se decidió?**
`OutputCol` incluye un campo `Table` (nombre/alias de la tabla) además del `Name` de la columna.

**Justificación:**
Es indispensable para JOINs. Sin el calificador de tabla, `SELECT e.name, d.name` no puede distinguir cuál `name` es cuál. El calificador también permite buscar columnas con o sin prefijo de tabla.

---

## H4 — Agregaciones y NULL

### Decisión 8: NULL en agregados sigue el estándar SQL

**¿Qué se decidió?**
`COUNT(*)` cuenta todas las filas. `SUM`, `AVG`, `MIN`, `MAX` y `COUNT(col)` ignoran NULLs. Si todos los valores son NULL, `SUM`/`AVG`/`MIN`/`MAX` retornan NULL.

**Justificación:**
Sigue el comportamiento estándar SQL. Silenciar los NULLs en agregados es la semántica correcta: un NULL no es un cero, es "dato desconocido".

---

### Decisión 9: NULLS LAST en ORDER BY independientemente de la dirección

**¿Qué se decidió?**
Los NULLs van siempre al final, tanto en ASC como en DESC.

**Justificación:**
`NULLS LAST` es el default más intuitivo y el más común en bases de datos relacionales. Un NULL en un `ORDER BY salary DESC` no debería aparecer primero porque no es "el salario más alto"; es dato ausente.

---

## H5 — JOINs y HashJoin

### Decisión 10: NestedLoopJoin como fallback, HashJoin para equi-joins

**¿Qué se decidió?**
El planner detecta si la condición `ON` es un equi-join simple (`col_a = col_b`). Si es así, usa `HashJoin` (O(n+m)). Si no, usa `NestedLoopJoin` (O(n×m)).

**Opciones evaluadas:**
- Siempre NestedLoop (más simple, siempre correcto).
- Siempre HashJoin (requiere descomponer la condición ON).
- Heurística: HashJoin para equi-joins, NestedLoop para el resto.

**Justificación:**
La heurística maximiza el rendimiento en el caso común (casi todos los JOINs son equi-joins) sin sacrificar la corrección para condiciones complejas. NestedLoopJoin sirve como fallback seguro y como punto de comparación (se puede verificar que ambos dan los mismos resultados).

---

### Decisión 11: ResettableOperator para NestedLoopJoin

**¿Qué se decidió?**
La interfaz `ResettableOperator` extiende `Operator` con `Reset()`. El operador inner del NestedLoopJoin debe implementarla.

**Justificación:**
Para cada fila del outer, el NestedLoopJoin necesita reiniciar el inner desde el principio. En lugar de reabrir o recrear el operador (costoso), un `Reset()` reposiciona el cursor en O(1). `TableScan.Reset()` simplemente pone `cursor = 0`.
