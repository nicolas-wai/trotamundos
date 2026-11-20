---
estado: aceptada
fecha: 2026-11-19
fuente: Grupo
---

# Patrón de despliegue

## Contexto y Problema

Las iteraciones anteriores dejaron decisiones que afectan la disponibilidad pero no definen cómo se despliegan los módulos: el servidor monolito modular se replica detrás de un balanceador de carga ([ADR08](./08-LoadBalabncer.md)), las solicitudes pasan por una cola de prioridad ([ADR09](./09-ColaPrioridad.md)) y los datos viven en un nodo con base primaria, réplicas de lectura y caché ([ADR10](./10-DBRelacionalCache.md), [ADR11](./11-ShardingDBporViajero.md)). Además, el servidor es stateless gracias a los tokens de vida corta ([ADR16](./16-TokensVidaCorta.md)).

La pregunta es: ¿cómo desplegamos cada uno de estos módulos para que una caída no planificada de un nodo no deje al sistema sin servicio? El alcance de la decisión es la vista de despliegue: módulo del balanceador, réplicas de backend, middleware cola de prioridad y módulo de datos.

## Decision Drivers

- QA04 - Inactividad no planificada
- QA13 - Multiregión

## Opciones Consideradas

- Redundancia activa (hot spare)
- Redundancia pasiva (warm spare)
- Repuesto en frío (cold spare)

## Resultado de la decisión

Opción elegida: Redundancia pasiva, por ser el punto medio. Ofrece recuperación rápida respecto a repuesto en frío y tiene un costo de ejecución y mantenimiento (consistencia) menor que la redundancia activa. Para el backend se respeta la redundancia activa ya definida en ADR08 (n réplicas del monolito modular según disponibilidad geográfica).

El patrón de despliegue queda así ([vista](../02-add/iteracion-6.md#vistas)):

- Un **cluster balanceado** con un nodo para cada réplica del monolito modular de backend. Son tantas réplicas (redundancia activa) como dicten las necesidades geográficas y computacionales en cada etapa de vida del sistema (variable a demanda).
- El **balanceador de carga** se ubica en su propio nodo y reparte el tráfico entre esas n réplicas.
- La **cola de prioridades** ([ADR09](./09-ColaPrioridad.md)) se despliega como nodo intermedio entre la lógica y los datos. En ese mismo nodo se implementa el healthcheck de la base de datos (ver [ADR22](./22-Heartbeat.md)).
- El **módulo de datos** se despliega aparte: un nodo primario y una instancia de redundancia pasiva. Ambos son instancias del módulo de datos definido en la [iteración 3](../02-add/iteracion-3.md) (primaria, réplicas de lectura y caché). La instancia primaria envía actualizaciones de estado de forma periódica a la instancia pasiva.

Esto se replica n veces según se considere en cada etapa de vida del sistema. Se despliega en diferentes ubicaciones geográficas y se delega el balanceo de carga a servidores DNS y asociados.

### Consecuencias

- Bueno, porque ante la caída de una réplica del backend el resto sigue atendiendo, y al ser stateless ([ADR16](./16-TokensVidaCorta.md)) no se pierden sesiones.
- Bueno, porque la recuperación de la base de datos es rápida al conmutar a la instancia pasiva.
- Bueno, porque evita el costo de mantener dos bases procesando todas las escrituras en simultáneo.
- Malo, porque agrega lógica de healthcheck, activación y actualización de la instancia pasiva.
- Malo, porque la sincronización es periódica, por lo que las escrituras posteriores a la última sincronización podrían no estar en la instancia pasiva al momento de conmutar.
- Neutral, hay que definir el período de sincronización para equilibrar costo y datos expuestos (tradeoff).

### Confirmación

Para confirmar el cumplimiento de este ADR se debe verificar en el despliegue que:

- El balanceador distribuye tráfico entre n réplicas del monolito modular y, al apagar una o varias de ellas, el sistema sigue respondiendo.
- La cola de prioridades está desplegada como nodo entre el backend y los datos.
- Existe una instancia pasiva del nodo de datos que recibe sincronizaciones periódicas de la primaria.
- Al apagar el nodo primario de datos, el tráfico pasa a la instancia pasiva. El sistema vuelve a responder normalmente.

## Pros y Cons de las opciones

### Redundancia activa (hot spare)

- Bueno, porque mantiene mayor confianza para el sistema.
- Bueno, porque la recuperación es rápida.
- Malo, porque pierde eficiencia al mantener ambas instancias procesando.
- Malo, porque tiene mayor costo de ejecución e incluye mantener la consistencia entre instancias.

### Repuesto en frío (cold spare)

- Bueno, porque no tiene costo de CPU por mantenimiento de replica.
- Malo, porque la recuperación es más lenta (crítico, compromete cumplimiento del driver).
- Malo, porque agrega lógica de healthcheck y activación.
- Malo, porque pierde los datos no actualizados.

## Más información

Riesgo a revisar en una próxima iteración: el nodo intermedio concentra la cola de prioridades y el monitoreo de la base de datos. Junto al balanceador de carga, son puntos débiles en la disponibilidad del sistema y un falla en cualquiera de ellos significaría una pérdida grave y difícil de recuperar.