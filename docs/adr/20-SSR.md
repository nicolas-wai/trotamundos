---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Renderizado

## Contexto y Problema

El sistema nos requiere que el usuario acceda inicialmente lo más rápido posible. Sin embargo, tampoco se puede mostrar cualquier tipo de información, por lo que se buscará una solución razonable a esta situación

## Decision Drivers

- QA03 - Primer contenido mobile
- QA02 - Tiempo respuesta Web

## Opciones Consideradas

- Server Side Rendering
- Cliente SPA

## Resultado de la decisión

Opción elegida: Server Side Rendering basado en templates, ya que permite simplificar la responsabilidad del cliente, y logra la mayor performance inicial

### Consecuencias

- Bueno, porque obtiene una carga inicial rápida
- Bueno, porque le delega menos complejidad al cliente
- Bueno, porque se alinea más a los conocimientos del equipo
- Malo, porque reduce responsividad
- Malo, delega lógica al servidor

### Confirmación

Se justifica el uso de Server-Side Rendering (SSR) para la operación de alta/creación debido a que reduce la complejidad en el cliente, garantiza la renderización centralizada y segura de las vistas en el servidor, mejorando el tiempo de carga inicial

## Pros y Cons de las opciones

### Cliente SPA

- Bueno, porque obtiene compatibilidad con el server stateless
- Bueno, porque descongestiona de lógica al servidor
- Bueno, porque añade responsividad
- Malo, porque aumenta el tiempo de carga inicial (principal objetivo)