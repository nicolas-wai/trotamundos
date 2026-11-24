# trotamundos

## Objetivo del proyecto
La cátedra nos presenta con el desafío de construir el prototipo para una plataforma que centralice información acerca de viajes (vuelos, hoteles, itinerarios, etc.). Como equipo nuestro objetivo es — en primera instancia — refinar requerimientos funcionales y quality attributes, definir límites del proyecto e iniciar de tal forma la documentación del proceso de diseño e implementación.

Una vez sentadas las bases, corresponde tomar los drivers bien refinados, priorizarlos y darle inicio a una "round de diseño" que será llevada según el modelo Attribute-Driven-Design. En este proceso iterativo-incremental se sistematiza en etapas la tarea de definir aspectos críticos del diseño, descomponer el sistema — eliminando incertidumbre en cada paso — y elegir soluciones que se apliquen a drivers seleccionados. A lo largo del proceso es de vital importancia validar la aplicación del método en cada iteración, la constante documentación y el respeto por el modelo — que introduce certeza a la ardua tarea de diseñar arquitecturas de software —

Por ultimo, nos toca aplicar el diseño generado en la round a un prototipo real. Esta implementación inicial permite ver qué tan buenas fueron las decisiones arquitectónicas tomadas.

| Documento | Descripción |
| --------- | ----------- |
| [Drivers arquitectónicos](docs/01-drivers/ArchitecturalDrivers.md) | Propósito de la ronda, user stories, escenarios de calidad, constraints, concerns, utility tree y priorización |
| [Diagrama de contexto](docs/01-drivers/contexto.md) | Límites del sistema |
| [Método ADD](docs/02-add/ADD.md) | Descripción de los pasos del método usados en cada iteración |
| [Enunciado resumido](docs/00-enunciados-resumenes/resumen-enunciado.md) | Resumen del enunciado de la cátedra |

## Cómo correr el prototipo

```
make run          # https://localhost:8080 (balanceador); las réplicas escuchan en https://127.0.0.1:8081 y :8082
```

Si hay archivos `.db` de una versión anterior, borrar `data/` antes de correr (`make clean`): el esquema no se migra.

El sistema se sirve por HTTPS, al arrancar se genera (o se reutiliza de `data/tls/`) un certificado autofirmado para `localhost` y `127.0.0.1`, por lo que el navegador avisa la primera vez que la conexion no es privada.

## Diferencias entre el diseño y la implementación

El prototipo corre en una sola PC, sin servidores. Por eso hay partes del diseño que no se pueden implementar tal cual. En todos los casos se mantienen los componentes, las interfaces y los eventos definidos en el diseño, y solo se reemplaza la infraestructura que hay detrás:

- **Un nodo por máquina y escalado para 2500 req/s [ADR-08, ADR-21]:** un proceso Go simula todos los nodos y hay dos réplicas en la misma PC.
- **Cola de mensajes P2P [ADR-04] y caché distribuida [ADR-10]:** `InMemoryMessageQueue` e `InMemoryCacheAdapter` detrás de los mismos puertos.
- **Heartbeat por red y elección de líder por consenso [ADR-21, ADR-22]:** canal en memoria; lidera la réplica viva de menor id. Como los nodos comparten proceso, no hay red entre ellos.
- **Proveedores de correo, agencias y sistemas de viaje reales, y TLS hacia ellos [ADR-05, ADR-12]:** stubs determinísticos con fallas inyectables.
- **OAuth 2.0 real con los proveedores de correo [ADR-15]:** `MockOAuth2AuthAdapter` con el mismo flujo (código, intercambio, tokens, revocación).

## Método de trabajo
Inicialmente, el grupo pensó en seguir las recomendaciones del paper "Better Together" para hacer un diseño colaborativo humano/LLM. La captura de drivers y su revisión (Paso 1 ADD) fue principalmente hecha por el grupo y, al momento de generar el archivo Architecture.md, como sugiere el paper, nos resultó difícil juzgar la validez de lo producido por el LLM. Por falta de experiencia en construcción de proyectos reales del grupo, caeríamos en el sesgo muy fácilmente.

En base a esto, decidimos hacer en primera instancia las iteraciones de ADD a mano (`/docs/02-add/iteracion-n.md`). Posteriormente, se simula la iteración por medio del LLM siguiendo las recomendaciones y prompts del paper "Better Together" (`/docs/anexo-llm/iteracion-n-llm.md`). Finalmente, estas se contrastan y ponderan, dando como resultado un conjunto de decisiones. Las mismas se documentan en formato MADR en la carpeta `/docs/adr/`. Cabe aclarar que para tener una base más firme, la iteración 1 es puramente nuestra. A partir de la iteración 2 se implementa el esquema antes descrito.

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

| Iteración | Foco | Drivers abordados | ADRs | Documento | Contraste LLM |
| --------- | ---- | ----------------- | ---- | --------- | ------------- |
| 1 | Estructura general | US01, C1, QA13 | [01](docs/adr/01-ArquitecturaBase.md), [02](docs/adr/02-OrgInternaServidor.md) | [iteracion-1](docs/02-add/iteracion-1.md) | no aplica |
| 2 | Integraciones externas | US01, QA01, QA04, QA14, QA15 | [03](docs/adr/03-PubSub.md), [04](docs/adr/04-ColaMensajesP2P.md), [05](docs/adr/05-PortsAdapters.md), [06](docs/adr/06-NormalizarEnBroker.md), [07](docs/adr/07-CircuitBreaker.md) | [iteracion-2](docs/02-add/iteracion-2.md) | [iteracion-2-llm](docs/anexo-llm/iteracion-2-llm.md) |
| 3 | Escalabilidad y persistencia | QA05, QA06, QA07 | [08](docs/adr/08-LoadBalancer.md), [09](docs/adr/09-ColaPrioridad.md), [10](docs/adr/10-DBRelacionalCache.md), [11](docs/adr/11-ShardingDBporViajero.md) | [iteracion-3](docs/02-add/iteracion-3.md) | [iteracion-3-llm](docs/anexo-llm/iteracion-3-llm.md) |
| 4 | Seguridad | QA08, QA09, QA15 | [12](docs/adr/12-EncriptarDatos.md), [13](docs/adr/13-InformarUsuario.md), [14](docs/adr/14-AutenticarUsuario.md), [15](docs/adr/15-EmailsConOAuth.md), [16](docs/adr/16-TokensVidaCorta.md) | [iteracion-4](docs/02-add/iteracion-4.md) | [iteracion-4-llm](docs/anexo-llm/iteracion-4-llm.md) |
| 5 | Presentación | QA02, QA03, QA11 | [17](docs/adr/17-PayloadReducido.md), [18](docs/adr/18-ReflejoAutomatico.md), [19](docs/adr/19-PersonalizacionLocalStorage.md), [20](docs/adr/20-SSR.md) | [iteracion-5](docs/02-add/iteracion-5.md) | [iteracion-5-llm](docs/anexo-llm/iteracion-5-llm.md) |
| 6 | Deployment y disponibilidad | QA04, QA13 | [21](docs/adr/21-Deployment.md), [22](docs/adr/22-Heartbeat.md) | [iteracion-6](docs/02-add/iteracion-6.md) | [iteracion-6-llm](docs/anexo-llm/iteracion-6-llm.md) |
| 7 | Recomendaciones | US08, QA17, QA18 | [23](docs/adr/23-AsincronismoRecomendaciones.md), [24](docs/adr/24-ConcurrenciaConsultaHistorial.md), [25](docs/adr/25-RecomendacionBasadaEnContenido.md) | [iteracion-7](docs/02-add/iteracion-7.md) | [iteracion-7-llm](docs/anexo-llm/iteracion-7-llm.md) |
### Vistas arquitectónicas

| Vista | Tipo | Ubicación |
| ----- | ---- | --------- |
| Contexto | Contexto | [contexto.md](docs/01-drivers/contexto.md) |
| Descomposición y arquitectura general | Módulos y C&C | [iteracion-1, paso 6](docs/02-add/iteracion-1.md#paso-6---documentación) |
| Integraciones externas | C&C y secuencia | [iteracion-2, paso 6](docs/02-add/iteracion-2.md#paso-6---documentación) |
| Escalabilidad y persistencia | Módulos y C&C | [iteracion-3, paso 6](docs/02-add/iteracion-3.md#paso-6---documentación) |
| Seguridad | Módulos y 2 secuencias | [iteracion-4, paso 6](docs/02-add/iteracion-4.md#paso-6---documentación) |
| Presentación | Módulos, C&C y secuencia | [iteracion-5, paso 6](docs/02-add/iteracion-5.md#paso-6---documentación) |
| Despliegue y healthchecks | Despliegue | [iteracion-6, paso 6](docs/02-add/iteracion-6.md#paso-6---documentación) |
| Recomendaciones | Módulos y secuencia | [iteracion-7, paso 6](docs/02-add/iteracion-7.md#paso-6---documentación) |


### Pendientes

- US03 Filtros y whitelists, es una capa sobre las reservas/dashboard y una configuración sobre la seguridad
- US05 Resolución de problemas, reutiliza los puertos y adaptadores de agencias
- US06 Analítica, baja prioridad (ver riesgos identificados)
- QA12 Capacidades nativas, baja prioridad, depende de como se resuelva finalmente el cliente móvil
- QA16 Proveedores de redes sociales, baja prioridad, puede ser un adapter mas depende de lo que se requiera hacer con ellas

### Parcialmente abordado

- QA03 - Adaptar el diseño web a un especifico cliente móvil
- QA10 - Explicitar como se agrega, el diseño ya apoya la integración sencilla
- QA13 - Explicitar como configura un pais, el diseño ya apoya la integración y escalabilidad sencilla