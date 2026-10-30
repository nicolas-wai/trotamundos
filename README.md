# trotamundos

## Objetivo del proyecto
La cátedra nos presenta con el desafío de construir el prototipo para una plataforma que centralice información acerca de viajes (vuelos, hoteles, itinerarios, etc.). Como equipo nuestro objetivo es — en primera instancia — refinar requerimientos funcionales y quality attributes, definir límites del proyecto e iniciar de tal forma la documentación del proceso de diseño e implementación.

Una vez sentadas las bases, corresponde tomar los drivers bien refinados, priorizarlos y darle inicio a una "round de diseño" que será llevada según el modelo Attribute-Driven-Design. En este proceso iterativo-incremental se sistematiza en etapas la tarea de definir aspectos críticos del diseño, descomponer el sistema — eliminando incertidumbre en cada paso — y elegir soluciones que se apliquen a drivers seleccionados. A lo largo del proceso es de vital importancia validar la aplicación del método en cada iteración, la constante documentación y el respeto por el modelo — que introduce certeza a la ardua tarea de diseñar arquitecturas de software —

Por ultimo, nos toca aplicar el diseño generado en la round a un prototipo real. Esta implementación inicial permite ver qué tan buenas fueron las decisiones arquitectónicas tomadas.

## Método de trabajo
Inicialmente, el grupo pensó en seguir las recomendaciones del paper "Better Together" para hacer un diseño colaborativo humano/LLM. La captura de drivers y su revisión (Paso 1 ADD) fue principalmente hecha por el grupo y, al momento de generar el archivo Architecture.md, como sugiere el paper, nos resultó difícil juzgar la validez de lo producido por el LLM. Por falta de experiencia en construcción de proyectos reales del grupo, caeríamos en el sesgo muy fácilmente.

En base a esto, decidimos hacer en primera instancia las iteraciones de ADD a mano (`/docs/02-add/iteracion-n.md`). Posteriormente, se simula la iteración por medio del LLM siguiendo las recomendaciones y prompts del paper "Better Together" (`/docs/02-add/iteracion-n-llm.md`). Finalmente, estas se contrastan y ponderan, dando como resultado un conjunto de decisiones. Las mismas se documentan en formato MADR en la carpeta `/docs/adr/`. Cabe aclarar que para tener una base más firme, la iteración 1 es puramente nuestra. A partir de la iteración 2 se implementa el esquema antes descrito.

## Escenarios de atributos de calidad
- Se aprovechó la construccion previa de la [skill](https://github.com/Dalviux9th/wonderful-skill) para escenarios de qa para orientar el diseño de escenarios
- Los parametros de medida de respuestas fueron consultados al LLM con la skill asumiendo un equipo de startup que busca bajo costos y una implementacion rapida

## ¿Por qué consideramos que el diseño es "good enough" para el prototipo?

El propósito de esta primera ronda es producir un diseño que permita construir un prototipo básico y evaluar la viabilidad técnica y de mercado de la startup. Siguiendo el método ADD, el diseño es suficiente cuando:

1. Los drivers de mayor prioridad tienen decisiones arquitectónicas que los abordan
2. La incertidumbre restante está identificada y tiene un camino de resolución definido.
3. Lo que queda sin diseñar no bloquea la construcción del prototipo ni invalida la estructura ya decidida.

Apuntamos a las decisiones estructurales que más adelante son dificiles/costosas de cambiar (Estilo general, integraciones, datos, seguridad) y dejamos al prototipo parametros que son baratos de ajustar (Y algunos requieren ser validados a partir del mismo) (umbrales, intervalos, tiempos de respuesta reales).

### Iteraciones realizadas

| Iteración | Foco | Drivers abordados | ADRs |
| --------- | ---- | ----------------- | ---- |
| 1 | Estructura general | US01, C1, QA13 | 01, 02 |
| 2 | Integraciones externas | US01, QA01, QA04, QA14, QA15 | 03 a 07 |
| 3 | Escalabilidad y persistencia | QA05, QA06, QA07 | 08 a 11 |
| 4 | Seguridad | QA08, QA09, QA15 | 12 a 16 |
| 5 | Presentación | QA02, QA03, QA11 | 17 a 20 |

### Pendientes

- US03 Filtros y whitelists, es una capa sobre las reservas/dashboard y una configuración sobre la seguridad
- US05 Resolución de problemas, reutiliza los puertos y adaptadores de agencias
- US06 Analítica, baja prioridad (ver riesgos identificados)
- US08 Recomendaciones, 2° Entrega
- QA12 Capacidades nativas, baja prioridad, depende de como se resuelva finalmente el cliente móvil
- QA12 Proveedores de redes sociales, baja prioridad, puede ser un adapter mas depende de lo que se requiera hacer con ellas

### Parcialmente abordado

- QA01 - Falta definir que logre la medida de respuesta de 5 min el 95% de las veces, requiere pruebas de latencia sobre el prototipo
- QA04 - Falta un sistema claro de deteccion y recuperación, se pueden probar health checks sobre el prototipo
- QA03 - Adaptar el diseño web a un especifico cliente móvil
- QA10 - Explicitar como se agrega, el diseño ya apoya la integración sencilla
- QA13 - Explicitar como configura un pais, el diseño ya apoya la integración y escalabilidad sencilla

### Riesgos identificados
Los riesgos que quedaron sin definir hay que explicitarlos. Se deben revisar frente al prototipo funcionando.

- El sharding por idViajero choca con los itinerarios compartidos
- El sharding por idViajero choca con la analitica (cruce de viajeros)
- Starving de escritura por la cola de prioridad
- Rollback cuando un cliente ya vio un cambio que no persistió
- Sin revocación inmediata del access token
- Puntos unicos de falla (cola/load balancer)