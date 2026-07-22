# Declaración de Uso de Inteligencia Artificial

## Herramientas utilizadas

- Antigravity (asistente de IA de Google DeepMind)

## Propósito del uso

- **Conceptos generales**: Discusión del modelo Volcano/Iterador, trade-offs entre tipos de join, semántica SQL de NULL.
- **Generación del scaffold inicial**: Estructura de paquetes, interfaces base, y primeras implementaciones de operadores.
- **Revisión de diseño**: Discusión sobre la inclusión de `Schema()` en la interfaz `Operator` vs. pasarlo por constructor.
- **Generación de tests**: Estructura de los tests table-driven.

## Módulos influenciados por IA

- `executor/operator.go` — interfaz `Operator` (estructura), decisión sobre métodos
- `executor/expr.go` — lógica de tres valores de SQL para AND/OR/NULL
- Estructura de paquetes general

## Partes de autoría íntegra del estudiante

De acuerdo con las reglas del proyecto, el núcleo lógico —las decisiones de diseño de la interfaz de operador, el encadenamiento de operadores en el planner, la elección de NestedLoop vs. HashJoin, y el manejo de NULL— debe ser comprendido y defendido por el estudiante.

## Declaración

> «Declaro que soy autor del diseño y la lógica central de este proyecto, que comprendo todo el código entregado y que puedo explicarlo y modificarlo.»

---

**Nota**: El uso de IA para generar código en este proyecto se hizo con fines pedagógicos. El estudiante debe poder:
- Explicar qué hace cada método de la interfaz `Operator` y por qué.
- Trazar el recorrido de una fila desde `TableScan` hasta `Project`.
- Agregar un operador nuevo (p. ej. `TopK`) sin modificar los existentes.
- Explicar la diferencia entre HashJoin y NestedLoopJoin.
- Justificar las reglas de NULL en comparaciones y agregados.
