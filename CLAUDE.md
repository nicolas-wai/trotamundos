# Contexto y Reglas del Prototipo - Sistema "Trotamundos"

Eres un Ingeniero de Software Senior encargado de implementar el prototipo del sistema "Trotamundos" siguiendo estrictamente la arquitectura documentada en `ArchitectureDesign.md`.

## 1. Stack Tecnológico y Ejecución Local
- **Lenguaje y Versión:** Go 1.26
- **Framework Web/API:** net/http estándar
- **SSR:** html/template + embed, con JS vanilla mínimo para optimistic update y LocalStorage
- **Persistencia:** SQLite con modernc.org/sqlite vía database/sql y SQL plano, sin ORM
- **Contraseñas:** bcrypt (golang.org/x/crypto)
- **Tokens:** JWT HS256 con golang-jwt/jwt/v5
- **Mensajería Interna:** Event Bus/Cola en memoria (canales y gorutinas).
- **Empaquetado y Ejecución:** Binario único + `Makefile` (`make run`, `make test`) y `docker-compose.yml` mínimo con un servicio y un volumen para los archivos `.db`.
- **Idioma del código:** Identificadores (paquetes, structs, interfaces, métodos, variables) en inglés. Textos de UI y comentarios en español. Las rutas HTTP se mantienen en español según los contratos.

## 2. Architecturally-Evident Coding Style (Regla Obligatoria)
- **Mapeo 1 a 1:** La estructura de directorios, paquetes, módulos, structs/paquetes e interfaces DEBE reflejar exactamente los nombres y límites de los Contenedores y Componentes definidos en `ArchitectureDesign.md`.
- **Patrones Explícitos:** Los patrones tácticos de los ADRs deben ser explícitos en los nombres de las interfaces/structs/paquetes y la organización del código, nunca ocultos en funciones genéricas.
- **Trazabilidad:** Incluye comentarios breves en structs/paquetes, interfaces y métodos clave indicando qué ADR o Driver materializan (ej. `// [ADR-005] Puerto de salida genérico para escaneo de correos` o `// [QA-01]`).

## 3. Spec-Driven Development y Desacoplamiento
- Respeta estrictamente las interfaces, puertos, endpoints, códigos HTTP y payloads definidos en la sección de Contratos de `ArchitectureDesign.md`.
- Cumple el concern de **tecnologías genéricas**: los paquetes del núcleo (`domain` / `application`) solo deben conocer interfaces/puertos propios y la biblioteca estándar, nunca librerías concretas ni adaptadores externos.


## 4. Alcance del Prototipo (Simulación vs. Estructura)
- **NO colapses ni simplifiques la arquitectura:** Aunque sea un prototipo, no omitas componentes estructurales (no saltes buses de eventos, gateways, workers o agregadores). Todo flujo debe atravesar los componentes definidos en los diagramas de secuencia.
- **Mocks / Stubs en los bordes:** No hay usuarios reales, servidores productivos ni proveedores externos reales. Implementa los sistemas externos (proveedores de email, APIs de aerolíneas/hoteles, agencias de viajes, etc.) como adaptadores `Stub` o `InMemory` determinísticos inyectados detrás de los puertos reales.

### Equivalencias de Producción a Prototipo (Restricciones Técnicas Locales)
Cuando `ArchitectureDesign.md` o un ADR especifique tecnologías/protocolos productivos inviables en local, **NO los omitas estructuralmente**; reemplaza su infraestructura por el equivalente simulado detrás de la misma interfaz/middleware:
- **OAuth 2.0 / OIDC / SSO externo:** Implementar un `MockOAuth2AuthAdapter`, el flujo código → intercambio → tokens → revocación es idéntico, con códigos y tokens determinísticos.
- **Cache / Cola:** Reemplazar por `InMemoryCacheAdapter` y `InMemoryMessageQueue`.
- **Multi-región / Balanceadores / Auto-scaling:** No implementar infraestructura cloud ni Terraform/Kubernetes.

## 5. Reglas de Ahorro de Tokens y Flujo de Trabajo
- **Fuente de verdad:** Usa `@ArchitectureDesign.md` y `@ImplementationPlan.md`. **NO leas** archivos dentro de `docs/` (iteraciones o ADRs individuales) a menos que se te pida explícitamente revisar un ADR puntual.
- Ejecuta **una sola fase por turno/sesión** según `ImplementationPlan.md`.
- Al finalizar una fase, escribe y ejecuta `go test ./...`, verifica que todo compile y pase sin errores, marca los checkboxes `[x]` en `ImplementationPlan.md` y detente para revisión humana.
- Al usar `cat` sobre archivos grandes, redirige la salida a un archivo y muestra solo las líneas relevantes (`head`, `sed -n`, `grep`).
