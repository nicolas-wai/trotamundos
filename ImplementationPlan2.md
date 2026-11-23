# ImplementationPlan2 — Cierre de desvíos respecto de los ADRs

> Rama de trabajo: `v3-cierre-desvios-adr` (sale de `main`). **No se mezcla con `main` sin revisión humana.**
> Continúa `ImplementationPlan.md` (fases 1 a 5, ya cerradas). Fuente de los desvíos: revisión de los ADRs de `docs/adr/` contra el código.
> Regla de trabajo (CLAUDE.md §5): **una fase por sesión**. Al cerrar una fase: `go test ./...` en verde, checkboxes `[x]` y se detiene para revisión humana.

---

## 0. Reglas que valen para todas las fases

### 0.1 Qué hacer con cada desvío al cerrar una fase

| Resultado | Qué se hace |
|---|---|
| El desvío **se solucionó** | Se marca como cerrado en el "Resumen de seguimiento" del final. |
| Se solucionó **pero apareció una limitación del entorno** (una PC, sin servidor, sin costo ni trámites) | Se marca como cerrado y se **anota la limitación** en la nota `> Estado (Fase N)`, con el motivo concreto y, si hay, números medidos. |
| **No se pudo** solucionar | Queda abierto en el seguimiento con el motivo. Si el motivo es del entorno, se anota como limitación. |

Además, en cada cierre de fase:

- [ ] Si se agrega un componente o paquete nuevo, actualizar `ArchitectureDesignV2.md` y el test de directorios (mapeo 1 a 1, CLAUDE.md §2). Todo cambio lleva el comentario `// [ADR-xx]` de trazabilidad.
- [ ] Escribir una nota `> Estado (Fase N)` al final de la fase con lo que se decidió, lo que se midió y lo que queda pendiente (mismo formato que `ImplementationPlan.md`).

### 0.2 Verificación común

- `go build ./...`, `go vet ./...`, `go test ./...` y `go test -race ./...` en verde.
- `go test ./src -run Architecture` en verde (las reglas de arquitectura).
- Los tests nuevos usan reloj y fallas inyectables (nada de `time.Sleep` largo ni de proveedores reales).

### 0.3 Alcance

| Fase | Contenido | Desvíos que cierra |
|---|---|---|
| 6 | Consistencia de datos y renovación de token | ADR-10 (copia falla), ADR-15 (refresh), ADR-21 (nodo caído) |
| 7 | Disponibilidad del sondeo y pantalla de itinerarios | ADR-04/21 (réplica 0), ADR-13 (pantalla web) |
| 8 | Seguridad y rendimiento | ADR-09/24/11 (un solo worker), ADR-12 (HTTPS), ADR-08 (prueba de carga) |

**Decididos como "aceptado" (no se tocan):** ADR-11 partición `directory`, ADR-20 SSR híbrido, ADR-01 cliente móvil, ADR-16 rotación de claves JWT.

**Orden dentro de la fase 8:** primero los workers, luego HTTPS y al final la prueba de carga, para medir el sistema ya terminado.

---

## Fase 6 — Consistencia de datos y renovación de token

**ADRs aplicados:** ADR-10, ADR-15, ADR-21 (+ ADR-22 para el monitor).
**Desvíos que cierra:** `10 BD relacional + caché`, `15 OAuth 2.0`, `21 Redundancia pasiva`.

### 6.1 ADR-10 — Si la copia a la base de lectura falla, no dejar caché vieja

Hoy `DataNode.Write` (`src/datanode/data_node.go`) escribe en la primaria, replica y recién después invalida la caché. Si `Replicate` falla, devuelve error **antes** de invalidar.

- [x] Invalidar la caché **siempre** apenas la primaria confirma, antes de replicar.
- [x] Decidir y documentar qué responde `Write` cuando la primaria confirmó pero la réplica falló (recomendado: éxito, porque el dato ya está guardado; un error haría que el cliente reintente y duplique).
- [x] Guardar la copia fallida en una lista de **replicaciones pendientes** por partición y reintentarla en segundo plano (`ReplicationRetrier` dentro de `datanode`).
- [x] Mientras una partición tenga replicaciones pendientes, las lecturas de esa partición van a la primaria, para no devolver datos viejos.
- [x] Hacer que la réplica de lectura y la caché reciban el evento E6 `DatosEscritos` como suscriptores, como dice el AD, en vez de llamarse en línea dentro de `Write`. Debe seguir siendo síncrono para conservar read-your-writes. (Esto también deja de dejar a E6 sin suscriptores.)

**Criterio de verificación:**

- Test con una réplica que falla a propósito: `Write` no deja caché vieja, la lectura inmediata ve el dato nuevo y, al volver a funcionar la réplica, la pendiente se aplica y la lista queda vacía.
- Test: dos escrituras seguidas con la réplica caída se aplican en orden al recuperarse.

### 6.2 ADR-15 — Renovar el token del correo solo

Hoy `OAuthManager.AccessToken` (`src/replica/integrations/oauthmanager/oauth_manager.go`) devuelve el token guardado sin mirar si venció, y `OAuthEmailPort.Refresh` solo lo usan los tests.

- [x] Verificar si el token guardado incluye su vencimiento; si no, agregar `ExpiresAt` al conjunto de tokens del puerto y a `LinkedEmailAccount` (columna nueva en el esquema).
- [x] `MockOAuth2AuthAdapter`: los tokens vencen según el reloj inyectado; `Refresh` entrega tokens nuevos y determinísticos.
- [x] `AccessToken`: si el token venció (o falta poco), llama a `Refresh`, cifra y guarda los tokens nuevos y devuelve el nuevo. Si `Refresh` falla con credencial inválida, marca la cuenta como "requiere volver a vincular" y el planificador deja de sondearla sin contar la falla como caída del proveedor [ADR-07].
- [x] Evitar renovaciones simultáneas de la misma cuenta (un candado por cuenta).

**Criterio de verificación:**

- Test: con el reloj adelantado, `AccessToken` renueva una sola vez aunque lo llamen varias goroutines a la vez, y el siguiente sondeo usa el token nuevo.
- Test: `Refresh` rechazado → cuenta marcada para re-vincular, sin abrir el circuit breaker.
- `grep -rn "\.Refresh(" src | grep -v _test` muestra ahora al `OAuthManager` como llamador del puerto OAuth.

### 6.3 ADR-21 — Que el nodo caído vuelva como pasivo

Hoy `DataNode.Stop` pone `down = true` y nunca vuelve a `false`; tras una conmutación el sistema queda sin pasivo.

- [x] `DataNode.Recover()`: limpia `down`, reinicia su emisor de heartbeat y su contexto.
- [x] `DataNodeSwitch`: al detectar que el nodo que quedó pasivo vuelve a latir, primero le instala una **imagen completa** del activo (`VACUUM INTO`, el mismo mecanismo de `SyncEmitter`), y solo entonces lo declara pasivo válido. Mientras no se haya sincronizado, el sistema no lo cuenta como respaldo.
- [x] El nuevo activo reapunta `SetPeer` al nodo recuperado y retoma la sincronización periódica.
- [x] `DataHeartbeatMonitor.Check`: además de conmutar, reincorpora. Si caen ambos nodos, queda registrado un aviso en vez de no hacer nada.
- [x] Emitir un evento de reincorporación (junto a E4) para que quede trazado.

**Criterio de verificación:**

- Test de punta a punta: se detiene el activo, el pasivo toma el control, se escribe, el caído se recupera, recibe la imagen con los datos escritos mientras estaba caído y, si ahora cae el nuevo activo, el sistema conmuta de vuelta **sin perder datos**.
- Test: si el nodo recuperado todavía no recibió la imagen, el monitor no lo usa como destino de conmutación.

### Cierre de la fase 6

- [x] Marcar como cerrados ADR-10, ADR-15 y ADR-21 en el seguimiento.
- [x] Limitaciones esperables a anotar: la "recuperación" del nodo se simula con `Recover()` dentro del mismo proceso (no hay reinicio real de máquina); los reintentos de replicación viven en memoria y se pierden si muere el proceso.
- [x] `go test ./...` completo y nota `> Estado (Fase 6)`.

> Estado (Fase 6): `go build`, `go vet`, `go test ./...` y `go test -race ./...` en verde, y `go test ./src -run Architecture` en verde. Decisiones a revisar:
>
> - **ADR-10, qué responde `Write` si la réplica falla:** éxito. La primaria ya confirmó y un error haría reintentar al cliente y duplicar. La copia queda en el `ReplicationRetrier` (`datanode/replication_retrier.go`), en orden por partición, y se reintenta cada `HeartbeatInterval` (o con `FlushReplication`). Mientras haya pendientes, `Read` de esa partición va a la primaria. `InstallSnapshot` vacía los pendientes de la partición.
> - **E6 con suscriptores internos:** `cacheInvalidator` y luego `readReplicaReplicator` (`data_written_subscribers.go`), síncronos y en ese orden. Para probar la falla, `Config.ReplicationFault` inyecta errores en la copia.
> - **ADR-15:** `OAuthManager.AccessToken` renueva si falta menos de 1 min (o si el vencimiento es desconocido), con un candado por cuenta y relectura tras tomarlo (una sola renovación con N goroutines). `ExpiresAt` se guarda en `linked_email_accounts.expires_at` (columna nueva). Si el proveedor rechaza la renovación, la cuenta pasa a `REQUIERE_VINCULAR` y sale del directorio (el planificador deja de sondearla); devuelve `ports.ErrEmailRelinkRequired`, que el adapter trata como `ErrInvalidCredential` (no abre el circuito). Una falla transitoria devuelve `ErrProviderUnavailable` y sí cuenta como caída. `MockOAuth2AuthAdapter.WithClock` hace vencer los tokens con el reloj inyectado.
> - **ADR-21:** `DataNode.Recover()` + `Run` reinicia heartbeat y reintentos tras `Stop`. `DataNodeSwitch` lleva `passiveReady`: tras conmutar o si el pasivo cae pasa a falso, y el monitor no conmuta a un pasivo no listo. Al volver a latir, `Reincorporate` instala la imagen completa (`SyncNow`, fuera del lock) y recién entonces emite E4b `PassiveRejoined` (nuevo, documentado en `ArchitectureDesignV2.md`). Si caen ambos, o el pasivo no está listo, el monitor deja un aviso por episodio (`Region.DataAlerts()`). `ports.SyncTarget` pasó a `src/ports` para que `priorityqueue` no importe `datanode`.
> - **Pendiente / limitaciones (limitaciones del entorno):** la recuperación se simula con `Recover()` en el mismo proceso; los reintentos de replicación viven en memoria.
> - **Datos de corridas anteriores:** el esquema agregó la columna `expires_at`; `CREATE TABLE IF NOT EXISTS` no migra archivos `.db` viejos, hay que borrar `data/` antes de volver a correr el binario.
> - **Sin medir:** no hay números de rendimiento en esta fase.

---

## Fase 7 — Disponibilidad del sondeo y pantalla de itinerarios

**ADRs aplicados:** ADR-04, ADR-21, ADR-13 (+ ADR-18 para la actualización optimista en la pantalla).
**Desvíos que cierra:** `04, 21 Cola P2P y backend activo`, `13 Informar al dueño`.

### 7.1 ADR-04/21 — Relevo automático del sondeo y la cola

Hoy solo la réplica 0 tiene consumidor y planificador (`RunIntegrations: i == 0`, `src/region/region.go`). Si cae, nadie más lo hace.

- [x] Todas las réplicas construyen el módulo de integraciones, pero solo la **líder** ejecuta consumidor y planificador.
- [x] Elección de líder simple y sin consenso distribuido: es líder la réplica viva de menor identificador, según el `HeartbeatPort` de réplicas (nombre sugerido: `IntegrationsLeaderElector`, dentro de `src/replica/integrations/`).
- [x] Cuando una réplica deja de ser líder o cae, se cancela el contexto de su consumidor y planificador. **Hoy ese contexto es el raíz de la región**, así que `replicaCancels` no los detiene: hay que atarlos al contexto de la réplica.
- [x] Nunca dos consumidores a la vez (la cola respeta `ErrSingleConsumer`). Documentar el pequeño tramo entre que la líder cae y el heartbeat la declara caída (hasta ~40 s, como en el heartbeat de réplicas).
- [x] El `since` de cada objetivo hoy vive solo en la memoria de la réplica 0. Una líder nueva volvería a leer todo: comprobar que la deduplicación por clave (`deduplicationKey`) hace inofensivo ese segundo sondeo, o mover el `since` a un lugar compartido de la región.
- [x] Los avisos E2 (proveedor degradado) deben seguir llegando al dashboard después del relevo.

**Criterio de verificación:**

- Test: se detiene el heartbeat de la réplica 0, pasa el tiempo (reloj inyectado), la réplica 1 se vuelve líder y sigue ingiriendo un correo nuevo de un proveedor falso.
- Test: si la réplica 0 vuelve, no hay dos consumidores simultáneos y no se duplican arreglos.
- Test: los mensajes publicados sin consumir mientras no había líder se procesan al llegar la nueva (la cola los retiene).

### 7.2 ADR-13 — Pantalla web de itinerarios compartidos

Hoy el dueño solo puede llamar a `GET /itinerarios/{id}/solicitudes`. No hay pantalla (no hay referencias a "itinerar" en `src/client/web`), y no existe un endpoint para listar los itinerarios propios.

- [x] Resolver cómo la pantalla obtiene los itinerarios del dueño: **preferir** incluirlos en los datos del dashboard sin agregar rutas. Si hace falta un endpoint nuevo, actualizar primero el contrato en `ArchitectureDesignV2.md` (§6.2) y los DTO de `src/contracts/httpdto`.
- [x] Plantilla y JS en `src/client/web`: sección "Mis itinerarios" con las solicitudes pendientes de cada uno, botones **Aceptar** y **Rechazar** (`POST .../decision`), y lista de accesos con **Revocar**.
- [x] Crear itinerario y agregar o quitar arreglos desde la pantalla.
- [x] Vista del invitado: si recibe `403 puedeSolicitar: true`, mostrar el botón "Solicitar acceso".
- [x] Actualización optimista con rollback al decidir una solicitud, igual que en ADR-18.
- [x] Las solicitudes nuevas aparecen sin recargar la página (consulta periódica corta desde el JS; el canal de entrega externo sigue siendo la limitación ya anotada).
- [x] Textos de la interfaz en español.

**Criterio de verificación:**

- Tests de handlers y de plantillas para la nueva sección.
- Test de navegador (`make test-e2e`, mismo patrón de `src/client/web/browser_e2e_test.go`): el dueño ve una solicitud pendiente, la acepta, y el invitado pasa a ver el itinerario; rama de rechazo y de revocación.

### Cierre de la fase 7

- [x] Marcar como cerrados ADR-04/21 y ADR-13 en el seguimiento.
- [x] Limitación esperable: la elección de líder usa los heartbeats en memoria del mismo proceso; no es consenso distribuido real.
- [x] `go test ./...`, `make test-e2e` y nota `> Estado (Fase 7)`.

> Estado (Fase 7): `go build`, `go vet`, `go test ./...`, `go test -race ./...`, `go test ./src -run Architecture` y `make test-e2e` en verde. Decisiones a revisar:
>
> - **ADR-04/21, relevo del sondeo:** todas las réplicas arman el módulo de integraciones; `IntegrationsLeaderElector` (`replica/integrations/leader_elector.go`, dentro del paquete `integrations`, sin directorio nuevo) decide líder a la réplica viva de menor id según el heartbeat de réplicas y reevalúa cada `HeartbeatInterval`. `Replica.RunIntegrations` lanza consumidor y planificador con el contexto de la líder y los cancela al perder el liderazgo; `Region.startReplica` los ata al contexto de la réplica, así `StopReplica` los detiene (antes colgaban del contexto raíz). Se agregó `Region.RecoverReplica` para simular el regreso. Todas las réplicas escuchan E2, pero solo la líder lo registra (una vez por región).
> - **Tramo sin líder y solapamiento:** hasta ~3 pulsos (~40 s con pulso de 10 s) nadie consume ni sondea; la cola retiene los mensajes. Si la líder nueva arranca antes de que la anterior libere, `Consume` devuelve `ErrSingleConsumer` y `RunWithRetry` reintenta. Un mensaje en vuelo sin `Ack` cuando la líder cede se re-entrega a los 30 s (los tests lo acortan).
> - **`since`:** se dejó en la memoria de cada planificador. La líder nueva relee todo y la deduplicación por clave lo hace inofensivo (`TestIntegrationsLeaderReturnsWithoutDuplicates`: sin duplicados tras el relevo). El circuit breaker de la líder nueva arranca cerrado.
> - **ADR-13, pantalla:** los itinerarios del dueño viajan en `GET /dashboard` (campo `itinerarios` del JSON y sección "Mis itinerarios compartidos" del HTML), sin rutas nuevas; contrato actualizado en `ArchitectureDesignV2.md` §6.2 y `httpdto.OwnedItinerary`. Nueva lectura `itinerary.list_by_owner` (shard del dueño) y `ListOwnedItineraries` en Permisos; si Permisos falla, el dashboard se entrega sin esa sección. El JS vive en `static/itineraries_panel.js`: aceptar/rechazar y revocar son optimistas con rollback (ADR-18), las solicitudes nuevas llegan con consulta cada 5 s a `GET /itinerarios/{id}/solicitudes` (el e2e la baja a 150 ms con `window.TROTAMUNDOS_ITINERARY_POLL_MS`), y la vista del invitado ofrece "Solicitar acceso" ante `403 puedeSolicitar: true`.
> - **Alcance elegido:** el dueño ve a los viajeros por `idViajero` (no hay email en el contrato de permisos). Crear un itinerario desde la pantalla lo crea vacío y las reservas se agregan después con el selector.
> - **Tests:** `leader_elector_test.go` (reloj inyectado), `region/leader_failover_test.go` (relevo, regreso sin duplicados, mensajes retenidos sin líder), `TestDashboardIncludesOwnedItinerariesWithRequestsAndAccesses`, render SSR y `client/web/itineraries_e2e_test.go` (aceptar + revocar, rechazar, rollback ante 500). `TestCrudWritesAndDashboardReadsAreQueuedWithTheirKind` ahora espera 4 lecturas en el dashboard.
> - **Pendiente / limitaciones (limitaciones del entorno):** la elección de líder usa heartbeats en memoria del mismo proceso, no consenso real. La entrega de E7 por un canal externo sigue siendo la limitación ya anotada.
> - **Sin medir:** no hay números de rendimiento en esta fase.

---

## Fase 8 — Seguridad y rendimiento

**ADRs aplicados:** ADR-09, ADR-24, ADR-11, ADR-12, ADR-08.
**Desvíos que cierra:** `09, 24, 11 Cola de prioridad, consultas concurrentes, sharding`, `12 HTTPS`, `08 Load balancer`.

### 8.1 ADR-09/24/11 — Varios workers en la cola de prioridades

Hoy `PriorityQueueNode.work` (`src/priorityqueue/node.go`) es una sola goroutine que ejecuta una operación a la vez en toda la región. La prioridad ordena pero no da paralelismo.

- [x] Investigar primero cuánto paralelismo permite SQLite en esta PC: hoy hay **una conexión por archivo** (`primary`, `readreplica`). Con una sola conexión por archivo, el paralelismo real es entre shards. Evaluar WAL y varias conexiones para lecturas en la réplica. Anotar el resultado medido en la nota de estado.
- [x] Un **worker por partición** (cada shard y la partición `directory`), cada uno con su propio par de colas de lectura (prioridad 1) y escritura (prioridad 2). El orden de las escrituras de una misma partición se conserva.
- [x] Dar a la cola la forma de saber la partición de una operación (un puerto pequeño que exponga el `PartitionRouter` del nodo de datos), sin que `priorityqueue` importe `datanode` (regla de contenedores).
- [x] Opcional si la medición lo justifica: workers extra solo de lectura por partición.
- [x] Auditar el código que asumía un único worker (los repositorios no usaban locks porque había un único worker): agregar sincronización donde haga falta y ejecutar todo con `-race`.
- [x] Mantener el comportamiento actual: si el request se cancela, la operación igual se ejecuta (`Await` solo deja de esperar), y la conmutación de nodo (`DataNodeSwitch`) funciona con los workers en marcha.
- [x] Actualizar el comentario de starvation en `PriorityQueueNode` (sigue sin envejecimiento de prioridad, aceptado en ADR-09).

**Criterio de verificación:**

- Test con barrera: dos operaciones de shards distintos se ejecutan **al mismo tiempo**; dos del mismo shard se ejecutan en orden, con la lectura antes que la escritura.
- Test del `ConcurrentCollector`: con una demora inyectada en el nodo de datos, el tiempo total se acerca al de la lectura más lenta y no a la suma (hoy se serializan al ejecutar).
- `go test -race ./...` sin carreras.

### 8.2 ADR-12 — HTTPS con certificado autofirmado

Hoy solo existe HTTP. El ADR exige que no se pueda conectar por http.

- [x] Generar al arrancar (o reutilizar de `data/tls/`) un certificado autofirmado con la biblioteca estándar (`crypto/x509`), válido para `localhost` y `127.0.0.1`.
- [x] El balanceador y las réplicas sirven con `ListenAndServeTLS`. **No se abre ningún puerto http.** El `ReverseProxy` del balanceador confía solo en ese certificado (un `x509.CertPool`, sin `InsecureSkipVerify`).
- [x] Cabecera `Strict-Transport-Security` en las respuestas.
- [x] Actualizar `main.go`, `Makefile`, `docker-compose.yml` y `README.md` (URL `https://localhost:8080`, aviso del navegador la primera vez). Los tests de navegador aceptan el certificado propio.
- [x] Quitar los comentarios `// [Prototipo] HTTP local simulando HTTPS/TLS` de `main.go`, `balancer.go`, `router.go` y `provider_apis.go` donde ya no apliquen.
- [x] Anotar en `ImplementationPlan.md` §0.2 que la fila de HTTPS fue reemplazada por este plan.

**Limitaciones esperables (registrar como limitación del entorno):**

- Las conexiones a proveedores son stubs en proceso, sin red: no hay TLS que aplicar ahí.
- Los archivos `.db` siguen sin cifrar: `modernc.org/sqlite` no trae cifrado en reposo y las alternativas (SQLCipher) exigen cgo y otro driver. Evaluarlo antes de dar la limitación por cierta.
- El certificado autofirmado no lo respalda una autoridad real, por lo que el navegador avisa.

**Criterio de verificación:**

- Test: una petición `http://` al puerto del balanceador falla (el servidor no habla http); una `https://` con el certificado propio responde `200`.
- Test: una petición https con un certificado distinto es rechazada por el balanceador hacia la réplica.
- `go test ./...` y `make test-e2e` en verde con el sistema sobre https.

### 8.3 ADR-08 — Prueba de carga de 500 req/s

Hoy solo hay un smoke de 200 lecturas **secuenciales** con p95 < 800 ms (`src/region/arrangements_test.go`, `TestQA02DashboardReadLatencySmoke`).

- [x] Generador de carga en Go, sin dependencias nuevas, con etiqueta `load` (`make test-load`): lanza pedidos a ritmo fijo de 500 por segundo (modelo abierto, con ticker) contra `GET /dashboard` por el balanceador, con usuarios y datos de prueba cargados antes.
- [x] Mide p50, p95, p99, máximo y tasa de error, y los escribe en un resumen.
- [x] Objetivo: p95 < 800 ms con 500 req/s sostenidos durante al menos 30 s, sobre el sistema ya con HTTPS y varios workers.
- [x] Probar también mezcla de lecturas y escrituras (por ejemplo 90/10) para ver el efecto de la prioridad.
- [x] Si no se cumple: perfilar con `pprof`, corregir los cuellos de botella razonables, volver a medir. Si queda por el hardware (núcleos, disco, SQLite de un solo escritor por archivo), **anotarlo como limitación** con los números medidos y la máquina usada.
- [x] Registrar en la nota de estado los resultados y los comandos para repetir la prueba.

**Criterio de verificación:**

- `make test-load` corre en esta PC y deja un resumen con p95 y tasa de error. El resultado queda en la nota `> Estado (Fase 8)`.
- La prueba no forma parte de `make test` (es lenta y depende de la máquina).

### Cierre de la fase 8

- [x] Marcar como cerrados ADR-09/24/11, ADR-12 y ADR-08 en el seguimiento.
- [x] Revisar que no queden desvíos abiertos: solo los que el grupo decidió aceptar (ADR-11 `directory`, ADR-20, ADR-01, ADR-16).
- [x] `go test ./...`, `go test -race ./...`, `make test-e2e`, `make test-load` y nota `> Estado (Fase 8)`.

---

> Estado (Fase 8): `go build`, `go vet`, `go test ./...`, `go test -race ./...`, `go test ./src -run Architecture`, `make test-e2e` (sobre https) y `make test-load` en verde. Decisiones a revisar:
>
> - **ADR-09/24/11, workers:** `PriorityQueueNode` tiene un carril (`lane`, `admission.go`) por partición (cada shard y `directory`) más uno de respaldo para operaciones cuya partición no se resuelve (el nodo de datos devuelve el error real). Cada carril: su par `ReadQueue`/`WriteQueue`, un worker de escrituras (despacha lecturas primero y conserva el orden de las escrituras de la partición) y `DefaultReadWorkers = 4` workers de solo lectura (opcional del plan, necesario para el criterio del `ConcurrentCollector`: sus 4 lecturas van al mismo shard). La cola conoce la partición por el puerto nuevo `ports.PartitionResolver` (`DataNode` y `DataNodeSwitch` lo implementan), sin importar `datanode`. No hay paquetes nuevos.
> - **Lo que apareció al paralelizar:** (1) un worker de solo lectura podía llevarse la señal de despertar de una escritura y dejarla sin atender (deadlock en los tests de `region`); ahora cada carril tiene `wakeWriter` y `wakeReaders`. (2) Con lecturas concurrentes a una escritura, una lectura lenta podía dejar en caché un dato viejo; `DataNode` usa un `partitionGuard` por partición (generación + candado) y vuelve a invalidar al terminar `Write`. Los repositorios siguen sin locks (cada operación es una sentencia por archivo, una conexión por archivo); `-race` limpio.
> - **SQLite, medido en esta PC:** una conexión con 1 lector 9.600 consultas/s; una conexión con 4 lectores 9.200/s (no escala); WAL con 4 conexiones y 4 lectores 36.400/s (~3,8x). **No se aplicó** WAL ni más conexiones en la réplica de lectura porque la prueba de carga ya da p95 de unos 2 ms; queda como mejora si hiciera falta. El paralelismo real hoy es entre particiones y, por la caché, entre lecturas.
> - **ADR-12, HTTPS:** `loadbalancer/tls.go` (dentro del contenedor del balanceador, sin paquete nuevo): `LoadOrCreateTLS` genera o reutiliza `data/tls/cert.pem` y `key.pem` (ECDSA P-256, `localhost`, `127.0.0.1` y `::1`, 1 año, clave 0600). `main.go` sirve balanceador y réplicas con `ListenAndServeTLS`; no se abre ningún puerto http. El `ReverseProxy` usa `WithBackendTLS` (un `x509.CertPool`, sin `InsecureSkipVerify`) y el balanceador agrega `Strict-Transport-Security`. Un `http://` al puerto TLS no se sirve (Go responde 400 sin contenido de la aplicación). Se actualizaron `README.md` (nueva sección "Cómo correr"), los comentarios `[Prototipo]` y `ImplementationPlan.md` §0.2. `docker-compose.yml` y el `Dockerfile` no cambian (mismo puerto 8080, ahora https). Los tests de navegador aceptan el certificado propio (`IgnoreHttpsErrors`). `.db` sin cifrar: evaluado, `modernc.org/sqlite` no cifra y SQLCipher exige cgo y otro driver; quedó como limitación.
> - **ADR-08, carga:** `src/region/load_test.go` (etiqueta `load`, `make test-load`, resumen en `bin/load-summary.txt`). Ticker a 500 req/s (modelo abierto, latencia desde el instante programado), 30 s por escenario tras 3 s de calentamiento, https, 3 shards, 2 réplicas, 30 usuarios con 5 arreglos cada uno. Resultado en AMD Ryzen 5 150 (12 núcleos), Go 1.26: **solo lecturas p50 1,1 ms, p95 1,8 ms, p99 2,0 ms, 0 % de errores; mezcla 90/10 p50 1,5 ms, p95 2,6 ms, p99 3,0 ms (lecturas p95 2,6 ms, escrituras p95 2,2 ms), 0 % de errores.** Objetivo (p95 < 800 ms) cumplido, no hizo falta `pprof` (`LOAD_CPUPROFILE` queda disponible). Repetir: `make test-load` (variables `LOAD_RATE`, `LOAD_DURATION`, `LOAD_USERS`, `LOAD_WRITE_PCT`, `LOAD_P95_MS`). Las lecturas salen casi todas de la caché: no es una medida de SQLite en frío.
> - **Tests nuevos:** `priorityqueue/partition_workers_test.go` (barrera entre shards, orden en un mismo shard, solapamiento de lecturas con demora inyectada en el nodo de datos —misma forma que el `ConcurrentCollector`: el total se acerca a la lectura más lenta, no a la suma—, partición sin resolver, escrituras no desatendidas), `loadbalancer/tls_test.go` (http no se sirve, https propio responde 200 con una sola cabecera HSTS, réplica con otro certificado rechazada, reutilización del certificado en disco).
> - **Datos de corridas anteriores:** `data/tls/` se crea solo; `make clean` lo borra.
> - **Pendiente / limitaciones (limitaciones del entorno):** certificado autofirmado (el navegador avisa), sin TLS hacia proveedores (son stubs), `.db` sin cifrar, medición local y no de producción.

---

## Resumen de seguimiento

| Desvío | Fase | Estado |
|---|---|---|
| ADR-10 la copia falla y no se limpia la caché | 6 | [x] |
| ADR-15 el refresh de OAuth no se usa | 6 | [x] |
| ADR-21 el nodo caído no vuelve como pasivo | 6 | [x] |
| ADR-04/21 sondeo solo en la réplica 0 | 7 | [x] |
| ADR-13 falta la pantalla web de itinerarios | 7 | [x] |
| ADR-09/24/11 un solo worker | 8 | [x] |
| ADR-12 HTTPS | 8 | [x] |
| ADR-08 prueba de carga | 8 | [x] |
