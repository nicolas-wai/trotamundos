# Iteración 5 - Presentación del sistema "Trotamundos"

## PASO 2 - Objetivo de la Iteración

Diseñar el módulo presentación, asegurando una usabilidad apta para centralizar información y garantizando tiempos de respuesta óptimos. Tiene que seguir la arquitectura stateless planteado en el ADR16 y aprovechar los mecanismos optimizados del backend.

**Drivers a desarrollar en la iteración:**

- QA03 - Primer contenido mobile
- QA02 - Tiempo respuesta Web
- QA11 - UI Simple centrada en el viaje

## PASO 3 - Descomposición

Hay que refinar el módulo de presentación. Se debe considerar que los clientes enviarán el token firmado en cada request y el esquema previamente planteado de backend genera consistencia eventual en el dashboard, pero los cambios locales se presentarán en el momento para acortar el tiempo de respuesta.

## PASO 4 - Aplicando conceptos de diseño a los drivers seleccionados

**Alternativas propuestas:**

| Alternativa                                                                       | Pros                                                                                    | Contras                                                                               | Drivers relacionados        |
| --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | --------------------------- |
| Cliente SPA                                                                       | Compatibilidad con server stateless, alivia lógica del servidor, añade responsividad    | Aumenta tiempo de carga inicial                                                       | QA02, QA03 (Negativo)       |
| Server Side Rendering basado en templates                                         | Carga inicial rápida, menor complejidad en cliente                                      | Reduce responsividad, delega lógica al servidor                                       | QA03, QA02                  |
| Solo reflejar en el dashboard lo que hay en la base de datos                      | Consistencia clara                                                                      | El usuario ve el dashboard con delay mientras la base se actualiza                    | QA02, QA03 (Negativo), QA11 |
| Enviar solo la informacion critica que mostrará en el dashboard (Reducir payload) | Reduce el trafico de red, acelera el renderizado                                        | Posteriormente, el usuario tendrá que realizar llamadas adicionales para los detalles | QA02, QA03, QA11            |
| Reflejar cambios manuales automáticamente (Optimistic Update)                     | Experiencia mas fluida, evita que el cliente piense que su operación no funcionó        | Complejidad para manejar rollbacks ante falla en el backend                           | QA02, QA03                  |
| Enviar toda la información completa de cada arreglo de viaje                      | No requiere recargar la pagina para ver detalles                                        | Aumenta el tamaño del payload                                                         | QA02, QA03 (Negativo)       |
| Maintain user model                                                               | El sistema muestra una experiencia mas personalizada en base a sus preferencias previas | Requiere permisos del cliente para almacenar sus preferencias                         | QA11                        |

### 4.1 - Tiempos de respuesta

Elegimos las alternativas "Reflejar los cambios manuales automáticamente" y "Enviar solo la información crítica" para atacar el tiempo de respuesta en el cliente web y la primera visualización de contenido en el cliente. Ambas tácticas apuntan a no depender de todos los datos que contiene el backend. Las otras opciones quedan descartadas por degradar el performance del sistema en el cliente.

### 4.2 - UI Amigable

Elegimos la táctica "Maintain user model" para lograr una UI adaptada al usuario, centrada en el viaje. Se implementa con lógica client-side

### 4.3 - Diseño del cliente
Elegimos Server Side Rendering basado en templates para el cliente web. A esto lo complementamos con funcionalidad mínima en el cliente que permita implementar la táctica de (4.1 y 4.2)


## PASO 5 - Instanciar elementos, asignar responsabilidades y definir interfaces

El cliente web recibe el dashboard renderizado por el servidor. El cual mostrará, inicialmente, solo los datos esenciales para la interfaz principal. Posteriormente se podrá interactuar con la interfaz para ver más detalles (via request al servidor y recarga parcial)

Además, si el usuario crea o actualiza un arreglo de viaje manualmente, se debe ver reflejado instantaneamente sin esperar la latencia del backend/base de datos. Esto se logrará gracias a lógica interna del cliente (Optimistic Update)

Como el servidor planteado es stateless, la interfaz personalizada al usuario (Como orden de elementos, modo claro/oscuro, etc.) son almacenadas como cookies del lado del cliente (Tipo LocalStorage).

## PASO 6 - Documentación

### Vistas

**Descomposición del sistema (D. Módulos):**
```mermaid
graph TD
    A((TROTAMUNDOS)) ---- B((Conexiones Externas))
        B ---- BA((APIs Agencias))
        B ---- BB((APIs Correo))

    A --- C((Núcleo de Negocio))
        C ---- D(("Dashboard (UI Viajero) "))
            D ---- G((Presentación de reservas))
                G --- GA((Cliente Web))
                G --- GB((Cliente Móvil))
            D ---- E((Importación manual))
            D ---- F((Funcionalidad Secundaria *))
    C ---- J((Autenticación))
        J ---- K((Control de Permisos))
        J ---- H((Gestión de perfiles de usuario))

```

**Vista estructural (C&C)** - Componentes involucrados en el renderizado y presentación:
```mermaid
flowchart TD
    subgraph Client ["Cliente Web (Navegador)"]
        subgraph UI_Layer ["Capa de Interfaz de Usuario"]
            View["Vistas Renderizadas (DOM)"]
            ClientRuntime["Lógica de Cliente Minimal<br/>(Maintain User Model)"]
        end

        OptState["Gestor de Estado Optimista<br/>(en memoria)"]
        PrefMgr["Gestor de Preferencias"]
        LocalStorage[("LocalStorage / Cookies<br/>(Preferencias UI: Modo, Orden)")]

    end

    subgraph WebServer ["Servidor Web (Stateless)"]
        SSREngine["Módulo SSR"]
        AuthMiddleware["Módulo Autenticación<br/>(Validación Token Firmado)"]
        APIEndpoints["Controladores API / Endpoints"]
    end

    subgraph CoreSystem ["Sistema Core"]
        CoreBackend[("Backend Core & Persistencia<br/>(Consistencia Eventual)")]
    end

    %% --- CONECTORES Y FLUJOS DE DATOS ---

    %% Renderizado Inicial (SSR)
    View -->|"HTTPS GET /dashboard"| SSREngine
    SSREngine -->|"HTML + Datos Críticos Esenciales"| View

    %% Manejo de Preferencias Locales
    ClientRuntime -->|"Aplica orden/tema"| PrefMgr
    PrefMgr <-->|"Lectura/Escritura sincrónica (Web Storage API)"| LocalStorage
    PrefMgr -.->|"Modifica DOM (UI Amigable)"| View

    %% Actualizaciones Optimistas (Creación/Edición Manual)
    View -->|"Eventos de usuario (Crear/Editar)"| ClientRuntime
    ClientRuntime -->|"Actualización local inmediata"| OptState
    OptState -->|"Renderizado instantáneo"| View

    ClientRuntime -->|"HTTPS POST/PUT + Bearer Token Firmado"| AuthMiddleware

    %% Obtención de Datos Detallados (Recarga Parcial)
    View -->|"HTTPS GET /detalles + Bearer Token Firmado"| AuthMiddleware

    %% Procesamiento Server
    AuthMiddleware --> APIEndpoints
    APIEndpoints --> CoreBackend
```

**Vista comportamental (Secuencia)** - Flujo de datos entre el cliente y servidor:

```mermaid
sequenceDiagram
    actor U as Viajero
    
    box Cliente (Navegador)
        participant UI as Vista / DOM
        participant CR as Runtime Cliente
        participant LS as LocalStorage / Cookies
    end

    box Servidor & Backend
        participant WS as Servidor Web (SSR / Auth)
        participant Core as Backend Core
    end

    %% ESCENARIO 1: Carga Inicial de Dashboard
    Note over U, Core: 1. Carga Inicial de Dashboard (Payload Reducido via SSR)
    U->>UI: Solicita ingreso a /dashboard
    UI->>WS: GET /dashboard
    WS->>Core: Consultar solo información crítica esencial del viaje
    Core-->>WS: Retorna dataset mínimo
    WS-->>UI: Responde con HTML renderizado (Payload Reducido)

    UI->>LS: Consultar preferencias (Tema, Ordenación)
    LS-->>UI: Retorna estado de personalización local
    UI-->>U: Renderiza primera vista del Dashboard (Mínima Latencia)

    %% ESCENARIO 2: Creación/Edición Manual (Optimistic Update)
    Note over U, Core: 2. Creación Manual de Arreglo (Respuesta Instantánea + Confirmación Backend)
    U->>UI: Crea o edita un arreglo de viaje manualmente
    UI->>CR: Evento de envío de formulario

    %% Paso Optimista Local (Instantáneo)
    CR->>CR: Aplicar cambios en Estado Optimista local
    CR->>UI: Renderizar nuevo/modificado viaje en pantalla
    UI-->>U: Cambio visible al instante (Latencia percibida ~0ms)

    %% Comunicación Asíncrona con Backend
    CR->>WS: POST HTTPS [Header: Authorization Bearer Token Firmado]
    activate WS
    WS->>WS: Validar firma e integridad del Token
    WS->>Core: Procesar modificación del itinerario
    activate Core
    Core-->>WS: Devuelve HTTP-status-code
    deactivate Core


    %% Confirmación o Reversión de Estado
    alt HTTP-status-code == 2xx
        WS-->>CR: Confirmación exitosa del servidor
        CR->>CR: Consolidar estado local como "Sincronizado"
        CR->>UI: (Opcional) Remueve indicador de "Guardando..."
    else HTTP-status-code != 2xx
        WS-->>CR: Confirmación exitosa del servidor
        deactivate WS
        CR->>CR: Revertir cambios en Estado Optimista
        CR->>UI: Eliminar cambio de la vista y renderizar estado previo
        UI-->>U: Mostrar notificación: "No se pudo sincronizar el cambio. Reintentando..."
    end
    

```
En esta vista mostramos la carga inicial del sitio sumado al camino crítico de modificar/crear una reserva manualmente. Por último se muestran algunos elementos de cómo implementar la actualización optimista y posibles soluciones a rollbacks por fallo de esta.

### ADRs generados

Las decisiones de esta iteración se documentan en los siguientes Registros:

- ADR20 - Uso de Server Side Rendering
- ADR17 - Payload reducido en la interfaz
- ADR18 - Reflejar actualizaciones manuales
- ADR19 - Personalizacion en LocalStorage
- ADR20 - Renderizado SSR

## PASO 7 - Revisar la aplicación del método

Se refinó el módulo de presentación para cliente web, asegurando la compatibilidad con decisiones previas como el server stateless y las tácticas de optimizacion de consultas a la base de datos. Se cubrieron correctamente los drivers de usabilidad y respuesta al cliente.

Queda pendiente definir como tratar rollbacks si falla en red o backend (El cliente ya vió su cambio, pero no persistió).

El diseño actual ya aborda los drivers prioritarios de negocio, consideramos que no son necesarias más iteraciones para descomponer, podría realizarse una iteración extra para definir puntos sueltos centrados en elementos específicos.
