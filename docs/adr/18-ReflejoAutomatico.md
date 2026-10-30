---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Reflejo automático de actualizaciones manuales

## Contexto y Problema

Al diseñar la presentacion de un sistema con servidor stateless y persistencia asincrona, surge una latencia entre la accion del usuario y el reflejo en la base de datos. Si el cliente debe esperar la respuesta del backend para visualizar la creación o edición manual de un arreglo de viaje, el usuario percibirá lentitud en el sistema, lo que degrada la usabilidad y empeora la agilidad.

## Decision Drivers

- QA03 - Primer contenido mobile
- QA02 - Tiempo respuesta Web

## Opciones Consideradas

- Reflejar cambios manuales automáticamente (Optimistic Update)
- Solo reflejar en el dashboard lo que hay en la base de datos

## Resultado de la decisión

Opción elegida: Reflejar cambios manuales automáticamente (Optimistic Update), porque el objetivo principal es atacar el tiempo de respuesta percibido en el cliente. Al no depender de la confirmación de los datos que contiene el backend para actualizar la vista, se garantiza que los cambios locales se presenten en el momento, satisfaciendo los requerimientos de usabilidad.

### Consecuencias

- Bueno, porque genera experiencia de usauario fluida
- Bueno, porque evita que el usuario piense que su operacion no funciono mientras el guardado viaja por la red
- Malo, porque introduce complejidad técnica en el cliente para manejar rollbacks visuales

### Confirmación

Se utilizará esta opción para acortar el tiempo de respuesta en el cliente. Se deberá verificar durante la implementación que, al crear o editar un viaje, la interfaz del Dashboard muestre el cambio instantáneamente apoyándose en la lógica interna del cliente, delegando la confirmación del request en segundo plano.

## Pros y Cons de las opciones

### Solo reflejar en el dashboard lo que hay en la base de datos

- Bueno, porque ofrece una consistencia clara de datos y elimina la necesidad de manejar rollbacks visuales
- Malo, porque el usuario ve el dashboard con delay mientras la base se actualiza (QA02/QA03 Negativo)

