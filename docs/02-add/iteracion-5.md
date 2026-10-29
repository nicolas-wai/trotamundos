# Iteración 5 - Presentación del sistema "Trotamundos"

## PASO 2 - Objetivo de la Iteración
Diseñar el módulo presentación al usuario (Web y móvil), asegurando una usabilidad apta para centralizar información y garantizando tiempos de respuesta óptimos. Tiene que seguir la arquitectura stateless planteado en el ADR16 y aprovechar los mecanismos optimizados del backend.

**Drivers a desarrollar en la iteración:**
- QA03 - Primer contenido mobile
- QA02 - Tiempo respuesta Web
- QA11 - UI Simple centrada en el viaje


## PASO 3 - Descomposición
Hay que refinar el módulo de presentación, los clientes Web y clientes Mobile. Se debe considerar que los clientes enviarán el token firmado en cada request y el esquema previamente planteado de backend genera consistencia eventual en el dashboard, pero los cambios locales se tendrán que presentar en el momento.

## PASO 4 - Aplicando conceptos de diseño a los drivers seleccionados

**Alternativas propuestas:**

| ALTERNATIVAS                                                    | PROS | CONTRAS |   |   |   |   |   |   |   |
|-----------------------------------------------------------------|------|---------|---|---|---|---|---|---|---|
| Reflejar los cambios manuales automáticamente                   |      |         |   |   |   |   |   |   |   |
| Solo reflejar en el dashboard lo que hay en la base de datos    |      |         |   |   |   |   |   |   |   |
| Enviar solo la informacion critica que mostrará en el dashboard |      |         |   |   |   |   |   |   |   |
| Enviar toda la información completa de cada arreglo de viaje    |      |         |   |   |   |   |   |   |   |
|                                                                 |      |         |   |   |   |   |   |   |   |
|                                                                 |      |         |   |   |   |   |   |   |   |
|                                                                 |      |         |   |   |   |   |   |   |   |
|                                                                 |      |         |   |   |   |   |   |   |   |
|                                                                 |      |         |   |   |   |   |   |   |   |


### 4.1 - Tiempos de respuesta


### 4.2 - UI Amigable


## PASO 5 - Instanciar elementos, asignar responsabilidades y definir interfaces
El módulo integraciones externas se estructura con Ports and Adapters que expone un único puerto de proveedores externos y un adapter por tipo de proveedor (email, agencias, sistemas de viajes).
En el módulo del puerto se incluye un middleware que recibe las actualizaciones de los sistemas externos y coloca estas actualizaciones en la cola para luego ser consumidos por la lógica central del sistema de forma asíncrona.


## PASO 6 - Documentación

### Vistas
**Diagrama de Componentes y Conectores:**

```mermaid
flowchart LR

    subgraph SRV ["Servidor monolito modular / Sistema propio"]
        subgraph INT ["Módulo Integraciones (Ports & Adapters)"]
            AD_EMAIL@{ shape: stadium, label: "Adapter Email" }
            AD_AGENCY@{ shape: stadium, label: "Adapter Agencias" }
            AD_TRAVEL@{ shape: stadium, label: "Adapter Viajes" }
            PORT["Puerto Único de Proveedores"]
        end
            QUEUE["Cola de Mensajes"]
        subgraph CORE ["Lógica Central"]
            VIA["Arreglos de viaje"]
        end
    end
    subgraph EXT_SYS ["Sistemas externos"]
        EXT_EMAIL["Proveedor de Email"]
        EXT_AGENCY["Agencias"]
        EXT_TRAVEL["Sistemas de Viajes"]
    end

    %% Relaciones externas hacia los adaptadores
    EXT_EMAIL <-->|"revisa/devuelve actualizaciones"| AD_EMAIL
    EXT_AGENCY <-->|"revisa/devuelve actualizaciones"| AD_AGENCY
    EXT_TRAVEL <-->|"revisa/devuelve actualizaciones"| AD_TRAVEL

    %% Relaciones internas de Ports & Adapters y la Cola
    AD_EMAIL --> PORT
    AD_AGENCY --> PORT
    AD_TRAVEL --> PORT
    PORT -->|"coloca actualizaciones"| QUEUE
    QUEUE -->|"consume de forma asíncrona"| VIA

    %% Estilos aplicados
    classDef core fill:#0000ff,color:#ffffff
    classDef external fill:#ffff00,color:#000000

    class ADPTR,AD_EMAIL,AD_AGENCY,AD_TRAVEL,PORT,QUEUE,VIA core
    class EXT_EMAIL,EXT_AGENCY,EXT_TRAVEL external
```

- Amarillo: Sistemas externos
- Rectángulos azules: Módulos del sistema
- Píldoras azules: Adaptadores de puertos de nuestro sistema

La vista busca mostrar los módulos involucrados en la implementación de "ports and adapters" y "cola asíncrona" dentro del servidor.

**Diagrama de secuencias:**
```mermaid
sequenceDiagram
    autonumber
    actor Email as Sistema Email
    actor Agencia as Sistema Agencia
    actor Viajes as Sistema Viajes

    box Módulo Integraciones Externas (Ports & Adapters)
        participant EmailAdapter as Adapter Email
        participant AgenciaAdapter as Adapter Agencias
        participant ViajesAdapter as Adapter Viajes
        participant Puerto as Puerto Proveedores
        participant Middleware as Middleware Actualizaciones
    end

    box Infraestructura & Sistema Core
        participant Cola as Cola de Mensajes (Queue)
        participant Core as Lógica Central
    end

    %% Entrada de datos desde proveedores
    alt Notificación desde Email
        Email->>EmailAdapter: Notificar actualización
        activate EmailAdapter
        EmailAdapter->>Puerto: Normalizar evento al modelo del puerto
        activate Puerto
    else Notificación desde Agencias
        Agencia->>AgenciaAdapter: Notificar actualización
        activate AgenciaAdapter
        AgenciaAdapter->>Puerto: Normalizar evento al modelo del puerto
    else Notificación desde Sistemas de Viajes
        Viajes->>ViajesAdapter: Notificar actualización
        activate ViajesAdapter
        ViajesAdapter->>Puerto: Normalizar evento al modelo del puerto
    end

    %% Procesamiento del Middleware dentro del módulo del Puerto
    Puerto->>Middleware: Transferir evento de actualización
    activate Middleware
    Middleware->>Cola: Publicar actualización en la cola (Enqueue)
    activate Cola
    Cola-->>Middleware: Confirmación de publicación (ACK)
    deactivate Cola
    Middleware-->>Puerto: Procesado correctamente
    deactivate Middleware

    Puerto-->>EmailAdapter: Respuesta del puerto (OK)
    deactivate Puerto
    EmailAdapter-->>Email: Confirmación de recepción (202 Accepted)
    deactivate EmailAdapter

    %% Consumo Asíncrono por la Lógica Central
    Note over Cola, Core: Procesamiento Asíncrono

    loop Consumo continuo
        Core->>Cola: Escuchar / Polling de nuevos mensajes
        activate Cola
        Cola-->>Core: Entregar evento de actualización
        deactivate Cola
    end
```

En este diagrama buscamos dar una vista dinámica del consumo de servicios externos y posterior formateo y publicación interna hasta llegar a la lógica central.

### ADRs generados
Las decisiones de esta iteración se documentan en los siguientes Registros:
- ADR03 - Uso de Publisher-Subscriber (rechazado)
- ADR04 - Uso de "cola asincrona punto a punto" para actualizaciones
- ADR05 - Uso de estilo Ports & Adapters

## PASO 7 - Revisar la aplicación del método

Se refinó el módulo más crítico en performance e interoperabilidad con terceros. Se concentra toda la comunicación en este modulo, aislando el resto del sistema de los mismos. Falta definir con qué intervalo de tiempo hará las consultas el middleware para asegurar los tiempos requeridos por los escenarios. Consideramos que lo mas critico próximo es el ámbito de la escalabilidad del sistema.
