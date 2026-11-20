---
estado: aceptada
fecha: 2026-11-20
fuente: Grupo
---

# Concurrencia en la consulta del historial del usuario para el sistema de recomendación

## Contexto y Problema

El sistema de recomendación requiere información del historial de arreglos del usuario para adaptar a sus gustos. Previamente, se definió un sistema de sharding por idViajero ([ADR11](11-ShardingDBporViajero.md)) y una cola de prioridad en las lecturas de la base de datos ([ADR09](09-ColaPrioridad.md)) que simplifica y agiliza la busqueda ya que las recomendaciones son en base al usuario que las solicita.

Sin embargo, el historial se encuentra distribuido en distintas tablas, la forma en que se realizan las consultas son clave para tener un buen tiempo de generación

## Decision Drivers

- QA17 - Performance: Generacion de recomendaciones

## Opciones Consideradas

- Consultas concurrentes
- Consultas secuenciales
- Mantener una unica estructura de datos con toda la informacion del historial

## Resultado de la decisión

Opción elegida: Consultas concurrentes

### Consecuencias

- Bueno, porque reduce el tiempo de obtencion del historial
- BUeno, porque se pueden centralizar luego los resultados para formar la informacion general
- Bueno, porque aprovecha el sharding
- Malo, porque incrementa la carga en la base de datos
- Malo, porque un hilo de ejecución retrasa la entrega del resultado final

### Confirmación

Se debe verificar la implementacion mediante pruebas de rendimiento y concurrencia, ajustando el grado de concurrencia para no perder tiempos en el overhead. Tambien se debe verificar que la integracion entre los distintos historiales sea correcta

## Pros y Cons de las opciones

### Consultas secuenciales

- Bueno, porque es sencillo de implementar y facilita el testing
- Malo, porque perjudican el tiempo total de respuesta

## Estructura con informacion de los historiales

- Bueno, porque toda la informacion ya se encuentra centralizada para la consulta
- Bueno, porque reduce el numero de consultas y no sobrecarga la base de datos
- Malo, porque requiere mantener actualizada la estructura en cada moficiacion del historial
- Malo, porque puede llevar a inconsistencias que requiere un mayor costo de mantenimiento
