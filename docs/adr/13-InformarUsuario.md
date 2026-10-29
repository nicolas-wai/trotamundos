---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Informar al usuario de la solicitud de admisión de otro usuario

## Contexto y Problema

Debido a que la única manera de que un usuario pueda acceder al itinerario de otro es mediante una solicitud, se plantea esta solución.

## Decision Drivers

- QA09 - Security: Itinerarios compartidos

## Opciones Consideradas

- Informar al usuario del posible intruso
- Permitir libre acceso a todos los usuarios

## Resultado de la decisión

Opción elegida: Informar al usuario del posible intruso, lo que da mayor transparencia de la situación en ese momento

### Consecuencias

- Bueno, porque el usuario tiene conocimiento de la situación
- Bueno, porque es fácil hacer que el usuario indique la opción que quiere
- Malo, porque depende de la respuesta del usuario, haciendo que tenga que estar pendiente de si le llegan solicitudes
- Malo, porque puede llegar a existir un colapso de solicitudes

### Confirmación

Mediante el uso del cliente-servidor, es posible llevar a cabo esta interacción, pero la experiencia del usuario dependerá de que esté activo.

## Pros y Cons de las opciones

### Permitir libre acceso a todos los usuarios

- Bueno, porque mejora la performance general del sistema
- Malo, porque la decisión debe pertenecer al usuario
- Malo, porque el usuario no puede tener control de quien puede acceder a su información