# ImplementationPlan — Prototipo Trotamundos

> Fuente de verdad: `ArchitectureDesign.md` (AD) y `docs/01-drivers/ArchitecturalDrivers.md`.
> Regla de trabajo (CLAUDE.md §5): **una fase por sesión**. Al cerrar una fase: `go test ./...` en verde, checkboxes marcados `[x]` y se detiene para revisión humana.

---

## 0. Decisiones transversales del prototipo

### 0.1 Alcance (vertical slices)

| Fase | Slice                                                            | Flujos de AD §5 que atraviesa | User stories / QA                         |
| ---- | ---------------------------------------------------------------- | ----------------------------- | ----------------------------------------- |
| 1    | Esqueleto arquitectónico completo (sin lógica)                   | —                             | C1, C2, Concern tecnologías genéricas     |
| 2    | Región ejecutable: LB, ruta de datos, failover y autenticación   | 5.5, 5.7                      | QA04, QA05, QA06, QA07, QA09 (auth), ADR16 |
| 3    | Dashboard SSR y CRUD manual con optimistic update                | 5.1 (1ª mitad), 5.2           | US01, QA02, QA03, QA11                    |
| 4    | Ingesta de proveedores con circuit breaker y vinculación OAuth   | 5.3, 5.4                      | US02, US04, QA01, QA04, QA08, QA14, QA15  |
| 5    | Itinerarios compartidos y recomendaciones con HITL               | 5.6, 5.1 (2ª mitad), 5.8      | US07 (parcial), US08, QA09, QA17, QA18    |

**Fuera de alcance** (sin refinamiento en AD §1.3, solo quedan como entidades/puertos del esqueleto cuando el modelo de dominio los nombra):
US03 filtros y whitelists (solo entidad `TrustedSender` y atributo `Sender`), US05 agencia preferida, US06 analítica, redes sociales (US07/QA16), QA10 sin trabajo dedicado (favorecido por el registro de tipos de arreglo), QA12 y **cliente móvil** (solo se soporta `GET /dashboard` con `Accept: application/json`, AD §6.2).

### 0.2 Equivalencias de producción → prototipo (CLAUDE.md §4)

| Elemento de AD                                  | Equivalente local (misma interfaz/puerto)                                                    |
| ----------------------------------------------- | -------------------------------------------------------------------------------------------- |
| HTTPS / TLS (ADR12)                             | HTTP en localhost, comentario `// [Prototipo] HTTP local simulando HTTPS/TLS`                |
| DNS y balanceo geográfico (ADR21)               | Una sola región configurada por `RegionConfig` (QA13). Sin DNS                               |
| Load Balancer (ADR08)                           | `LoadBalancer` propio (round-robin con `httputil.ReverseProxy`) en `:8080`                  |
| N réplicas del monolito (ADR08, ADR21)          | N=2 instancias de `Replica` en el mismo proceso (`:8081`, `:8082`), sin estado compartido    |
| Cola P2P (ADR04)                                | `InMemoryMessageQueue` (canal + mapa de no confirmados, re-entrega si no hay `Ack`)          |
| Cola de prioridades (ADR09)                     | `PriorityQueueNode` con dos colas FIFO en memoria (lecturas antes que escrituras)            |
| Caché (ADR10)                                   | `InMemoryCacheAdapter`                                                                        |
| Base primaria, réplicas de lectura, shards      | Archivos SQLite (`modernc.org/sqlite`) por shard: `primary-shard-N.db` y `replica-shard-N.db` |
| Nodo de datos pasivo (ADR21)                    | Segundo `DataNode` con sus propios `.db`; sincronización con `VACUUM INTO` (E5)              |
| Heartbeat (ADR22)                               | Canal en memoria detrás de `HeartbeatPort`; pulso 10 s / 3 pérdidas (configurable en tests)  |
| OAuth 2.0 del correo (ADR15)                    | `MockOAuth2AuthAdapter` determinístico (código → intercambio → tokens → revocación)          |
| Proveedores de correo, agencias, sist. de viaje | `StubEmailProviderAPI`, `StubAgencyAPI`, `StubTravelSystemAPI` con formatos **distintos** y fallas inyectables |
| Notificación al dueño (E7, canal no definido)   | `InMemoryOwnerNotifier`                                                                       |
| Catálogo de proveedores para recomendaciones    | `StubProviderCatalog` con metadatos determinísticos                                          |

### 0.3 Estructura de directorios (mapeo 1 a 1 con AD §3 y §4)

Módulo Go en la raíz (`go.mod`, `module github.com/nicolas-wai/trotamundos`). El código vive bajo `src/` reemplazando las carpetas placeholder actuales (`src/servidor/*`, `src/cliente/web`), con identificadores en inglés.

```text
cmd/trotamundos/main.go              # Raíz de composición: arma una región completa (ADR21)
src/
├── config/                          # RegionConfig (QA13): regionId, réplicas, shards, proveedores, intervalos
├── contracts/                       # AD §6 — solo biblioteca estándar
│   ├── httpdto/                     # §6.2 Requests/Responses JSON (tags en español: "idViaje", "arreglos"...)
│   ├── apierror/                    # §6.1 { codigo, mensaje } + catálogo de códigos
│   └── events/                      # §6.3 E1..E8
├── ports/                           # AD §6.4 — un archivo por puerto
├── client/web/                      # [Contenedor] Cliente web (AD §4.1), embebido con embed
│   ├── templates/                   # html/template del dashboard (Renderizado por SSR)
│   └── static/                      # runtime.js, optimistic_state.js, visual_preferences.js, recommendations_panel.js
├── loadbalancer/                    # [Contenedor] Load Balancer y Health Monitor (AD §4.3)
│   ├── balancer.go                  # Balanceador de carga
│   └── replica_heartbeat_monitor.go # Monitor de heartbeats de réplicas
├── replica/                         # [Contenedor] Monolito modular (AD §4.4)
│   ├── replica.go                   # Ensambla los módulos de una réplica
│   ├── api/                         # API y controladores (rutas en español)
│   ├── tokenvalidator/              # Validador de token firmado (middleware)
│   ├── ssr/                         # Renderizador SSR
│   ├── authentication/{domain,application,adapters}
│   ├── permissions/{domain,application,adapters}          # Control de permisos
│   ├── travelarrangements/{domain,application,adapters}   # Arreglos de viaje
│   ├── updateconsumer/              # Consumidor de actualizaciones
│   ├── priorityqueueclient/         # Cliente de cola de prioridades (implementa PriorityQueuePort)
│   ├── heartbeat/                   # Emisor de heartbeat
│   ├── integrations/
│   │   ├── domain/                  # ExternalProvider, ProviderUpdate
│   │   ├── scheduler/               # Planificador de sondeo
│   │   ├── circuitbreaker/          # Circuit breaker y reintentos acotados
│   │   ├── providerport/            # Puerto único de proveedores (registro de adapters)
│   │   ├── adapters/{email,agency,travelsystem}/  # Adapter Email / Agencias / Sistemas de viajes
│   │   ├── broker/                  # Broker (middleware con normalización)
│   │   └── oauthmanager/            # Gestor OAuth de correo
│   └── recommendations/
│       ├── domain/
│       ├── collector/               # Recolector concurrente
│       ├── engine/                  # Motor de filtrado basado en contenido
│       └── hitl/                    # Retroalimentación HITL (Analyzer + Updater)
├── messagequeue/                    # [Contenedor] Cola de mensajes P2P
├── priorityqueue/                   # [Contenedor] Cola de prioridades y Health Monitor BD (AD §4.5)
│   ├── admission.go  read_queue.go  write_queue.go
│   ├── data_node_switch.go          # Conmutador de nodo de datos
│   └── data_heartbeat_monitor.go    # Monitor de heartbeats de datos
├── datanode/                        # [Contenedor] Nodo de datos primario / pasivo (AD §4.6)
│   ├── partition_router.go          # Enrutador de partición (idViajero)
│   ├── cache/                       # InMemoryCacheAdapter
│   ├── primary/  readreplica/       # SQLite plano, sin ORM
│   ├── schema/                      # DDL .sql embebido
│   ├── operations/                  # Catálogo de operaciones nombradas (recurso → SQL)
│   ├── sync_emitter.go              # Emisor de sincronización periódica
│   └── heartbeat_emitter.go
└── stubs/                           # Sistemas externos simulados (proveedores, OAuth, catálogo)
```

**Regla de dependencias** (verificada por test desde la Fase 1): los paquetes `*/domain` y `*/application` solo importan biblioteca estándar, `src/ports`, `src/contracts/...` y el `domain` de su propio módulo. Nada de `modernc.org/sqlite`, `golang-jwt`, `x/crypto`, `net/http` ni adaptadores.

**Ruta de toda operación de datos** (no se colapsa, CLAUDE.md §4): `application` → puerto de repositorio del módulo → adapter del módulo → `PriorityQueuePort` (`priorityqueueclient`, emite E8 `OperacionEncolada`) → `PriorityQueueNode` → `DataNodeSwitch` → `DataNode` (`DataPort`) → `PartitionRouter` → caché / primaria / réplica.

**Directorio global ◇** (decisión de prototipo, a validar): login por email y acceso de invitados por `idItinerario` necesitan resolver `idViajero` antes de conocer el shard. Se usa una partición `directory` con las tablas `traveler_directory (email → idViajero)` e `itinerary_directory (idItinerario → idDueño)`; el resto de los datos se particiona por `hash(idViajero) mod N` (ADR11).

---

## Fase 1 — Esqueleto arquitectónico completo

**Objetivo:** Que exista y compile toda la estructura de AD (contenedores, componentes, entidades de dominio, puertos, DTOs y eventos), sin lógica de negocio. Cada handler responde `501 NO_IMPLEMENTADO` con el formato de error de §6.1. Deja listas las herramientas de build/ejecución.

**Componentes / interfaces involucrados:** todos los de AD §3.2 y §4.7 como tipos vacíos o con métodos que retornan `ErrNotImplemented`; los 11 puertos de §6.4; los eventos E1–E8; todos los endpoints de §6.2.

**ADRs aplicados:** ADR01, ADR02, ADR05 (forma del puerto), ADR11 (clave de partición en el modelo), y la trazabilidad de todos los demás mediante comentarios `// [ADR-XX]`.

**Tareas:**

- [x] `go.mod` (Go 1.26) con dependencias: `modernc.org/sqlite`, `golang.org/x/crypto`, `github.com/golang-jwt/jwt/v5`.
- [x] `Makefile` con `run`, `test`, `build`, `vet`, `clean` (borra `data/*.db`).
- [x] `Dockerfile` multi-stage + `docker-compose.yml` mínimo: un servicio `trotamundos`, puerto `8080`, volumen `trotamundos-data:/app/data` para los `.db`.
- [x] Crear el árbol de §0.3 y eliminar las carpetas placeholder en español de `src/` (están vacías) y `src/aca-va-la-implementacion.md`.
- [x] `src/config`: `RegionConfig` (regionId, puertos, cantidad de réplicas, cantidad de shards, intervalo de sondeo ≤ 2 min, heartbeat 10 s / 3 pérdidas, TTL access token 900 s) leído de variables de entorno con defaults.
- [x] **Entidades de dominio** (AD §2) en el `domain` de cada módulo, con comentarios de agregado/invariante:
  - [x] `authentication/domain`: `Traveler` (raíz de *Cuenta del viajero*), `RefreshToken`, `TravelerPreferences`, `LinkedEmailAccount`, `TrustedSender` ◇ (US03, sin lógica).
  - [x] `travelarrangements/domain`: `Trip` (raíz), `TravelArrangement` con `ArrangementType`, `Origin`, `Status`, `DeduplicationKey`, `Sender`; subtipos `Accommodation`, `Flight`, `RentalCar`, `Train`, `Bus` como implementaciones de `ArrangementDetail` y un `ArrangementTypeRegistry` (QA10).
  - [x] `permissions/domain`: `SharedItinerary` (raíz), `AccessPermission`, `AccessRequest`.
  - [x] `integrations/domain`: `ExternalProvider` (con `CircuitState`), `ProviderUpdate`.
  - [x] `recommendations/domain`: `Recommendation` (raíz), `RecommendationFeedback`.
- [x] **Contratos** `src/contracts`:
  - [x] `httpdto`: un struct por request/response de §6.2 con tags JSON exactos del contrato.
  - [x] `apierror`: `ErrorResponse{Codigo, Mensaje}` y constantes (`CREDENCIALES_INVALIDAS`, `REFRESH_REVOCADO_O_EXPIRADO`, `ACCESO_NO_AUTORIZADO`, `TOKEN_EXPIRADO`, `NO_IMPLEMENTADO`, …) + helper de escritura con código HTTP.
  - [x] `events`: `ProviderUpdateNormalized` (E1), `ProviderDegraded` (E2), `Heartbeat` (E3), `ResponsibilitySwap` (E4), `StateSync` (E5), `DataWritten` (E6), `AccessRequestCreated` (E7), `QueuedOperation` (E8).
- [x] **Puertos** `src/ports` (§6.4): `ExternalProviderPort`, `CircuitBreakerPort`, `MessageQueuePort`, `PriorityQueuePort`, `DataPort`, `TokenServicePort`, `OAuthEmailPort`, `ItineraryPermissionsPort`, `RecommenderPort`, `HistoryPort`, `HeartbeatPort`; más `EventBusPort` (E2, E7 internos) y `OwnerNotifierPort` (E7). Cada uno con su comentario `// [ADR-XX]`.
- [x] Puertos de repositorio propios de cada módulo en su `application` (ej. `TripRepository`, `TravelerRepository`, `SharedItineraryRepository`, `PreferencesRepository`).
- [x] Interfaces de servicios de aplicación por módulo (ej. `AuthenticationService`, `TravelArrangementsService`, `PermissionsService`) con métodos que devuelven `ErrNotImplemented`.
- [x] `replica/api`: router `net/http` con **todas** las rutas de §6.2 (patrones Go 1.22+, en español) apuntando a handlers stub `501`.
- [x] Structs vacíos de cada componente de infraestructura: `LoadBalancer`, `ReplicaHeartbeatMonitor`, `PriorityQueueNode`, `DataNodeSwitch`, `DataHeartbeatMonitor`, `DataNode`, `PartitionRouter`, `InMemoryCacheAdapter`, `InMemoryMessageQueue`, `PollingScheduler`, `ProviderCircuitBreaker`, `Broker`, `UpdateConsumer`, `OAuthManager`, `ConcurrentCollector`, `ContentBasedFilteringEngine`, `HITLFeedback`, stubs externos.
- [x] `cmd/trotamundos/main.go`: raíz de composición que instancia la región (LB → 2 réplicas) y levanta el servidor con los stubs `501`.
- [x] Test de arquitectura `src/architecture_test.go` (usa `go/build` / `go/parser` de la biblioteca estándar) que falla si un paquete `domain`/`application` importa algo fuera de la regla de dependencias.

**Criterio de verificación:**

- `make build` y `go vet ./...` sin errores.
- `go test ./...` pasa: test de arquitectura (regla de dependencias), test de que el router registra cada ruta de §6.2 y responde `501` con `{codigo, mensaje}`, tests de serialización de DTOs/eventos que verifican los nombres JSON exactos del contrato (ej. `idViaje`, `avisosProveedores`, `eventId`).
- `make run` levanta y `curl localhost:8080/dashboard` responde 501 pasando por el LB.

---

## Fase 2 — Región ejecutable: balanceo, ruta de datos, failover y autenticación

**Objetivo:** Dejar funcionando la infraestructura de la región de punta a punta y el primer caso de uso real: registro, login, refresh y logout con tokens de vida corta validados localmente por cualquier réplica. Toda lectura/escritura atraviesa cliente de cola → cola de prioridades → conmutador → nodo de datos → partición → caché/primaria/réplica. Incluye la detección de caídas por heartbeat y el intercambio primario/pasivo.

**Componentes / interfaces involucrados:** `LoadBalancer`, `ReplicaHeartbeatMonitor`, `heartbeat.Emitter`, `api`, `tokenvalidator`, `authentication`, `priorityqueueclient` (`PriorityQueuePort`), `PriorityQueueNode` (colas de lectura/escritura, `DataNodeSwitch`, `DataHeartbeatMonitor`), `DataNode` (`DataPort`, `PartitionRouter`, `InMemoryCacheAdapter`, primaria, réplica de lectura, `SyncEmitter`), `TokenServicePort`, `HeartbeatPort`. Eventos E3, E4, E5, E6, E8. Flujos AD 5.5 y 5.7.

**ADRs aplicados:** ADR08, ADR09, ADR10, ADR11, ADR12 (simulado), ADR14, ADR16, ADR21, ADR22.

**Tareas:**

- [ ] `datanode/schema`: DDL SQL plano por shard (`travelers`, `refresh_tokens`, `preferences`, `linked_email_accounts`, `trips`, `arrangements`, `shared_itineraries`, `access_permissions`, `access_requests`, `recommendations`, `recommendation_feedback`) y de la partición `directory`. Todas las tablas particionadas llevan `id_viajero`.
- [ ] `datanode/operations`: catálogo de operaciones nombradas (`recurso` de E8 → SQL + tipo LECTURA/ESCRITURA). Solo SQL plano vía `database/sql`.
- [ ] `PartitionRouter`: `hash(idViajero) mod N` y ruta especial a `directory` ◇ [ADR11].
- [ ] `InMemoryCacheAdapter`: lectura cache-first, miss → réplica de lectura, invalidación por clave al escribir [ADR10].
- [ ] Primaria + réplica de lectura: la escritura va a `primary-shard-N.db`, emite E6 `DataWritten` que replica a `replica-shard-N.db` e invalida la caché [ADR10].
- [ ] `PriorityQueueNode`: admisión, cola de lecturas (alta) y escrituras (baja), FIFO por tipo, despacho por un worker gorutina; devuelve resultado por canal (`Futuro<Resultado>`) [ADR09]. Documentar el riesgo de starvation en comentario.
- [ ] `priorityqueueclient`: implementa `PriorityQueuePort`, construye E8 con `idViajero` tomado del token (nunca del body) [ADR09, ADR11].
- [ ] Nodo pasivo: segundo `DataNode`; `SyncEmitter` periódico con `VACUUM INTO` y E5 [ADR21].
- [ ] `DataHeartbeatMonitor` + `DataNodeSwitch`: tras 3 pulsos perdidos redirige al pasivo, emite E4 y el nodo promovido activa su heartbeat y sincronización [ADR22].
- [ ] `heartbeat.Emitter` en cada réplica y `ReplicaHeartbeatMonitor` que retira/reincorpora réplicas del `LoadBalancer` [ADR22, ADR08].
- [ ] `LoadBalancer` round-robin sobre réplicas vivas con comentario `// [Prototipo] HTTP local simulando HTTPS/TLS` [ADR08, ADR12].
- [ ] `authentication/adapters`: `BcryptPasswordHasher` (`x/crypto/bcrypt`) y `JWTTokenServiceAdapter` (HS256, access 900 s, refresh persistido y revocable) que implementan puertos propios [ADR14, ADR16].
- [ ] `authentication/application`: `Register`, `Login`, `Refresh`, `Logout`, con email único (vía `directory`).
- [ ] `tokenvalidator`: middleware que valida firma/vigencia localmente, responde `401 TOKEN_EXPIRADO` / token inválido e inyecta `idViajero` en el contexto [ADR16].
- [ ] Handlers reales de `POST /registro`, `POST /login`, `POST /token/refresh`, `POST /logout` con payloads y códigos exactos de §6.2.
- [ ] Composición en `main.go`: primario + pasivo + nodo de cola + 2 réplicas + LB. Archivos `.db` en `./data` (volumen de docker-compose).

**Criterio de verificación** (`go test ./...`):

- Unit: `PriorityQueueNode` despacha lecturas antes que escrituras encoladas antes y mantiene FIFO dentro de cada tipo.
- Unit: `PartitionRouter` envía siempre al mismo shard el mismo `idViajero`; caché hit/miss/invalidación.
- Unit: `JWTTokenServiceAdapter` emite, valida, expira (reloj inyectable) y rechaza refresh revocado.
- Integración (httptest a través del LB, flujo 5.5): registro `201` → login `200` con `tokenType: "Bearer"` → request autenticado atendido por réplica A y luego por réplica B sin estado compartido → token vencido `401` → refresh `200` → logout `204` → refresh `401 REFRESH_REVOCADO_O_EXPIRADO`. Login con contraseña incorrecta `401 CREDENCIALES_INVALIDAS`.
- Integración (flujo 5.7, intervalos acelerados): detener una réplica → se retira tras 3 pulsos y el tráfico sigue `2xx`; detener el nodo primario → E4, el pasivo atiende y los datos sincronizados previos se leen correctamente.
- Test de arquitectura sigue en verde.

---

## Fase 3 — Dashboard SSR y CRUD manual con optimistic update

**Objetivo:** El viajero ve su dashboard organizado por viaje, renderizado en servidor con solo datos críticos, pide el detalle bajo demanda y crea/edita/elimina reservas manualmente con reflejo instantáneo y rollback visual. Las preferencias visuales (orden y tema) viven en LocalStorage.

**Componentes / interfaces involucrados:** Cliente web (`Vista`, `Runtime de cliente mínimo`, `Gestor de estado optimista`, `Gestor de preferencias visuales`), `api`, `tokenvalidator`, `ssr` (Renderizador SSR), `travelarrangements` (dashboard, altas manuales, deduplicación base, retiro de viajes completados), `priorityqueueclient`. Flujos AD 5.1 (primera mitad) y 5.2.

**ADRs aplicados:** ADR16, ADR17, ADR18, ADR19, ADR20 (+ ADR09, ADR10 heredados).

**Tareas:**

- [ ] `travelarrangements/application`: `GetDashboard` (solo datos críticos, viajes completados fuera de la vista — US03), `GetArrangementDetail`, `CreateArrangement`, `UpdateArrangement`, `DeleteArrangement`; validaciones → `422`, inexistente → `404`, arreglo de otro viajero → `404/403`.
- [ ] `travelarrangements/adapters`: repositorios sobre `PriorityQueuePort` (lecturas prioridad alta, escrituras baja).
- [ ] Gestión de viajes mínima ◇ para que `POST /arreglos` tenga un `idViaje` válido (seed determinístico de un viaje demo al registrarse; sin endpoint nuevo).
- [ ] `ssr`: plantillas `html/template` embebidas desde `src/client/web/templates`, payload reducido (sin `detalle`) [ADR17, ADR20].
- [ ] `GET /dashboard` con negociación de contenido: HTML para web, JSON de §6.2 con `Accept: application/json` (móvil ◇). Incluye `avisosProveedores` (vacío en esta fase).
- [ ] `GET /arreglos/{idArreglo}` (JSON o fragmento HTML), `POST /arreglos` `201`, `PUT /arreglos/{idArreglo}` `200`, `DELETE /arreglos/{idArreglo}` `204`.
- [ ] Shell web ◇ (`GET /` y página de ingreso) servida estática: el runtime hace login vía `POST /login`, guarda el token en memoria/sessionStorage y obtiene `GET /dashboard` con `Authorization: Bearer` para inyectar el HTML renderizado.
- [ ] `static/runtime.js` + `static/optimistic_state.js`: aplica el cambio local, muestra “guardando…”, envía `POST/PUT/DELETE`; en 2xx marca sincronizado, en no-2xx revierte y avisa [ADR18].
- [ ] `static/visual_preferences.js`: orden y tema en LocalStorage (con try/catch), aplicados antes de mostrar [ADR19].
- [ ] UI en español, simple y centrada en el viaje (QA11).

**Criterio de verificación** (`go test ./...`):

- Unit `travelarrangements/application` con repositorio fake: dashboard agrupa por viaje, excluye viajes completados, valida payloads.
- Integración por el LB: crear → aparece en `GET /dashboard` (JSON y HTML) → editar `200` → detalle `200` con `detalle` → eliminar `204` → `404`. Sin token `401`; arreglo de otro viajero no accesible.
- Test de SSR: el HTML contiene los títulos de los arreglos y **no** contiene campos de `detalle` (payload reducido).
- Test de que las escrituras del CRUD se encolaron como `ESCRITURA` y la lectura del dashboard como `LECTURA` (espía sobre el nodo de cola).
- Smoke QA02: 200 lecturas de `GET /dashboard` locales con p95 < 800 ms.
- Verificación manual documentada: `make run`, ingresar, crear un arreglo y ver el cambio instantáneo; forzar error (ej. id inexistente) y ver el rollback.

---

## Fase 4 — Ingesta de proveedores con circuit breaker y vinculación OAuth del correo

**Objetivo:** Implementar el flujo más crítico (QA01, QA15, QA14): vincular un correo con OAuth 2.0 simulado, sondear periódicamente correo, agencias y sistemas de viajes a través del puerto único, normalizar en el broker, encolar en la cola P2P y que el consumidor registre las reservas en el dashboard sin duplicarlas. Ante fallas, reintentos con espera progresiva, apertura del circuito y aviso al viajero.

**Componentes / interfaces involucrados:** `oauthmanager` (`OAuthEmailPort` ← `MockOAuth2AuthAdapter`), `PollingScheduler`, `ProviderCircuitBreaker` (`CircuitBreakerPort`), adapters `email`, `agency`, `travelsystem` (`ExternalProviderPort`), `providerport`, `Broker`, `InMemoryMessageQueue` (`MessageQueuePort`), `UpdateConsumer`, `travelarrangements` (deduplicación y avisos), `EventBusPort`, stubs `StubEmailProviderAPI` / `StubAgencyAPI` / `StubTravelSystemAPI`. Eventos E1, E2. Flujos AD 5.3 y 5.4.

**ADRs aplicados:** ADR04, ADR05, ADR06, ADR07, ADR12 (simulado), ADR15 (+ ADR09).

**Tareas:**

- [ ] `stubs`: tres APIs simuladas con formatos propios distintos (p. ej. correo con texto semiestructurado y remitente, agencia con JSON anidado, sistema de viajes con códigos IATA), datos determinísticos, la **misma reserva** presente en dos fuentes (para deduplicar) y modo de falla configurable (errores/timeouts).
- [ ] `MockOAuth2AuthAdapter`: `intercambiarCodigo`, `refrescar`, `revocar` determinísticos; códigos inválidos → error [ADR15].
- [ ] Cifrado de tokens OAuth en reposo con AES-GCM (biblioteca estándar `crypto/aes`) detrás de un puerto `TokenCipher`; nunca se guarda contraseña del correo [ADR15].
- [ ] `POST /correos` `201 { idCuenta, proveedor, estado: "VINCULADA" }` y `DELETE /correos/{idCuenta}` `204` (revoca y elimina el vínculo).
- [ ] Adapters por tipo que implementan `ExternalProviderPort.sondear(referencia, desde)` y traducen al modelo del puerto; errores `ProveedorNoDisponible`, `CredencialInvalida`, `RespuestaNoParseable` [ADR05].
- [ ] `providerport`: registro de adapters por tipo (agregar un tipo = registrar un adapter, QA10/QA14).
- [ ] `Broker`: normaliza a E1 `ActualizacionProveedorNormalizada` (incluye `claveDeduplicacion`, `generadoEnFuente`, `recibidoEn`, `origenRemitente`) y publica en la cola P2P. La lógica nunca recibe datos sin formatear [ADR06].
- [ ] `InMemoryMessageQueue`: `publicar`, `consumir`, `confirmar`; un único consumidor; mensajes no confirmados se re-entregan (tolerancia a caída del consumidor) [ADR04].
- [ ] `ProviderCircuitBreaker` por proveedor: estados CERRADO/ABIERTO/SEMIABIERTO, N reintentos con backoff progresivo, publica E2 `ProveedorDegradado` al abrir [ADR07].
- [ ] `PollingScheduler`: dispara sondeos cada ≤ 2 min (intervalo de `RegionConfig`) sobre cuentas vinculadas y proveedores; omite proveedores con circuito abierto [ADR05].
- [ ] `UpdateConsumer`: consume E1, delega en `travelarrangements` y confirma tras escritura exitosa [ADR04].
- [ ] `travelarrangements`: aplicar E1 (crear/modificar/cancelar), deduplicar por `claveDeduplicacion` fusionando información, marcar `origen` y `remitente` (US03), y exponer E2 como `avisosProveedores` en el dashboard.
- [ ] Registrar `recibidoEn` → visible para medir QA01.

**Criterio de verificación** (`go test ./...`):

- Unit adapters: cada formato stub se traduce correctamente; respuesta malformada → `RespuestaNoParseable`.
- Unit broker: produce E1 válido y con los nombres JSON del contrato.
- Unit circuit breaker: tras N fallas abre, publica E2, omite llamadas mientras está abierto, pasa a semiabierto y cierra tras éxito (reloj inyectable).
- Unit cola P2P: mensaje no confirmado se re-entrega; un único consumidor.
- Integración flujo 5.4: `POST /correos` con código válido `201`, tokens guardados cifrados (el valor en SQLite ≠ texto plano), `DELETE` `204` y llamada a `revocar` registrada en el mock; código inválido `4xx`.
- Integración flujo 5.3 (intervalo acelerado): vincular correo → sondeo → reservas aparecen en `GET /dashboard` con `origen` `EMAIL`/`AGENCIA`/`VIAJES` y `remitente`; la reserva presente en dos fuentes aparece **una sola vez**; latencia `generadoEnFuente → visible` < umbral escalado de QA01.
- Integración degradación: activar falla en `StubAgencyAPI` → circuito abierto → `avisosProveedores` muestra el aviso y el resto del dashboard sigue `200`.
- Test de arquitectura: `travelarrangements` no importa `integrations` (solo se comunican por la cola P2P y el bus de eventos).

---

## Fase 5 — Itinerarios compartidos y recomendaciones con human-in-the-loop

**Objetivo:** Completar los dos flujos restantes de AD §5: (a) compartir reservas en itinerarios cuyo acceso solo se concede con aprobación explícita del dueño (0 accesos no autorizados, QA09); (b) panel de recomendaciones cargado de forma asíncrona e independiente del dashboard, generado en < 3 s con recolección concurrente y filtrado basado en contenido, que usa el feedback del viajero en la siguiente recomendación.

**Componentes / interfaces involucrados:** `permissions` (`ItineraryPermissionsPort`), `OwnerNotifierPort` ← `InMemoryOwnerNotifier`, `EventBusPort` (E7), `travelarrangements` (lectura de arreglos referenciados), `recommendations` (`RecommenderPort`, `HistoryPort`, `ConcurrentCollector`, `ContentBasedFilteringEngine`, `HITLFeedback`), `StubProviderCatalog`, Cliente web (`Panel de recomendaciones asíncrono`). Flujos AD 5.6, 5.1 (segunda mitad) y 5.8.

**ADRs aplicados:** ADR13, ADR14, ADR16, ADR23, ADR24, ADR25 (+ ADR09, ADR11).

**Tareas — Itinerarios compartidos (US07 parcial):**

- [ ] `permissions/application`: crear (con `linkPublico` por defecto), eliminar, agregar/quitar arreglos propios, `puedeVer`, `solicitarAcceso`, `decidir`, `revocar` [ADR13].
- [ ] Registro en `itinerary_directory` para resolver el shard del dueño a partir de `idItinerario` ◇ [ADR11].
- [ ] Endpoints de la tabla de §6.2: `GET /itinerarios/{idItinerario}` (`200` o `403 { codigo: "ACCESO_NO_AUTORIZADO", puedeSolicitar: true }`), `POST /itinerarios`, `DELETE /itinerarios/{id}`, `POST|DELETE /itinerarios/{id}/arreglos[/{idArreglo}]`, `POST|GET /itinerarios/{id}/solicitudes`, `POST /itinerarios/{id}/solicitudes/{idSolicitud}/decision`, `DELETE /itinerarios/{id}/accesos/{idViajero}`.
- [ ] Publicar E7 `SolicitudAccesoCreada` en el bus; `InMemoryOwnerNotifier` la registra (canal real no definido en AD).
- [ ] Solo el dueño puede gestionar el itinerario, listar solicitudes y decidir (`403` en otro caso). Rechazo bloquea el acceso.

**Tareas — Recomendaciones (US08):**

- [ ] `StubProviderCatalog`: ítems de alojamiento, transporte y ruta con metadatos determinísticos (precio, destino, categoría, estrellas…).
- [ ] `ConcurrentCollector` (`HistoryPort`): lecturas en paralelo (gorutinas + `errgroup` propio o `sync.WaitGroup`) de historial por tabla (arreglos, viajes, feedback) y del perfil, unificadas después; cada lectura pasa por la cola de prioridades [ADR24].
- [ ] `ContentBasedFilteringEngine`: filtra/puntúa el catálogo por similitud de metadatos con historial y `TravelerPreferences` [ADR25].
- [ ] `HITLFeedback` (Analyzer + Updater): `ACEPTAR` / `RECHAZAR` / `AJUSTAR` actualizan `TravelerPreferences` vía escritura en cola [ADR25, QA18].
- [ ] `GET /recomendaciones` con `context.WithTimeout` de 3 s (QA17); `POST /recomendaciones/{idRecomendacion}/feedback` `200 { registrado: true }`; `PUT /perfil/preferencias` `200`.
- [ ] `static/recommendations_panel.js`: request independiente tras el primer render, estado de carga y estado de error sin bloquear las reservas [ADR23].

**Criterio de verificación** (`go test ./...`):

- Unit `permissions`: matriz de autorización (dueño, autorizado, pendiente, rechazado, revocado, anónimo) → solo dueño y autorizados ven.
- Integración flujo 5.6 por el LB: invitado `403 puedeSolicitar` → solicita `201 PENDIENTE` → notificación registrada para el dueño → dueño acepta `200 ACEPTADA` → invitado `200` → dueño desautoriza `204` → invitado `403`. Rama rechazo → `RECHAZADA` y sigue `403`. Un no-dueño no puede decidir ni listar solicitudes.
- Unit `ConcurrentCollector`: las consultas se ejecutan en paralelo (espía con barrera) y se unifican; falla parcial propagada.
- Unit `ContentBasedFilteringEngine`: ranking determinístico según preferencias.
- Integración flujos 5.1/5.8: `GET /recomendaciones` < 3 s → `POST feedback AJUSTAR { presupuestoMax: 120 }` → la **siguiente** `GET /recomendaciones` excluye ítems sobre ese presupuesto (QA18).
- Integración ADR23: con el recomendador forzado a fallar/demorar, `GET /recomendaciones` responde error/timeout mientras `GET /dashboard` sigue `200`.
- `go test ./...` completo en verde, incluido el test de arquitectura, como cierre del prototipo.
