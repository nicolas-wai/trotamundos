---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Transferencia de información crítica

## Contexto y Problema

Ante una demanda de performance, es razonable seleccionar que elementos serán presentados al usuario al interactuar con el sistema. Para ello, debemos resolver de qué manera este podrá verlos

## Decision Drivers

- QA03 - Primer contenido mobile
- QA02 - Tiempo respuesta Web
- QA11 - UI Simple centrada en el viaje

## Opciones Consideradas

- Reducir payload
- Solo reflejar en el dashboard lo que hay en la base de datos

## Resultado de la decisión

Opción elegida: Reducir payload, porque el objetivo principal es que el usuario vea la información del dashboard de la manera más rápida posible, por lo que hay que elegir una alternativa que garanice velocidad.

### Consecuencias

- Bueno, porque reduce el trafico de red
- Bueno, porque acelera el renderizado
- Malo, porque el usuario deberá realizar llamadas para poder ver mayor detalle

### Confirmación

Se utilizara esta opción para reducir el tiempo de respuesta del cliente, aunque no obtengamos transparencia real con el cliente, pero no se dependerá de los datos del backend.

## Pros y Cons de las opciones

### Reflejar lo que hay en la base de datos

- Bueno, porque existe consistencia clara
- Malo, porque el usuario ve el dashboard con delay mientras la base se actualiza (peor performance)