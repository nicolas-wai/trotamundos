---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Renderizado al cliente centrado en SSR

## Contexto y Problema

Un set de drivers exigen tiempos cortos de respuesta para el primer acceso al sistema y subsecuentes actualizaciones. Debemos definir la arquitectura del cliente de modo que atienda a estas necesidades.

## Decision Drivers

- QA03 - Latencia de primer contenido
- QA02 - Tiempo respuesta Web

## Opciones Consideradas

- Server Side Rendering
- Cliente SPA

## Resultado de la decisión

Opción elegida: Server Side Rendering basado en templates, ya que permite simplificar la responsabilidad del cliente, y logra la mayor performance inicial. Su aplicación no limita el escalado de lógica en el cliente, permitiendo (por ej.) aportar responsividad en casos de uso puntuales.

### Consecuencias

- Bueno, porque obtiene una carga inicial rápida
- Bueno, porque le delega menos complejidad al cliente
- Bueno, porque se alinea más a los conocimientos del equipo
- Malo, porque reduce responsividad
- Malo, delega lógica al servidor

### Confirmación

Al entrar al sitio web, los tiempos de respuesta concuerdan con las métricas pedidas en QA03. Si se ejecuta un curl con petición GET al sitio, devuelve la página renderizada (no json).

## Pros y Cons de las opciones

### Cliente SPA

- Bueno, porque obtiene compatibilidad con el server stateless
- Bueno, porque descongestiona de lógica al servidor
- Bueno, porque añade responsividad
- Malo, porque aumenta el tiempo de carga inicial (principal objetivo)