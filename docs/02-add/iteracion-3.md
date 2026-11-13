# Iteración 3 - Escalabilidad, Rendimiento y Persistencia en "Trotamundos"

## PASO 2 - Objetivo de la Iteración

Escalar la arquitectura del sistema Trotamundos para soportar una alta concurrencia de usuarios y un volumen proyectado de 15 millones de cuentas, garantizando disponibilidad, escalabilidad y rendimiento de operaciones.

**Drivers a desarrollar en la iteración:**

* QA05 - Cantidad de cuentas
* QA06 - Actividad semanal
* QA07 - Picos de trafico
* ADR2 - Centralidad del Server (concern iteración-1)

## PASO 3 - Descomposición

La lógica de trotamundos es lo que está con mayor incertidumbre. Se decomponen los componentes internos del mismo sobre como realizar las operaciones (consultas y escrituras manuales) de los usuarios.

## PASO 4 - Aplicando conceptos de diseño a los drivers seleccionados

**Alternativas propuestas**

| Alternativa | Pros | Contras | Drivers relacionados |
| --- | --- | --- | --- |
| Load Balancer de usuarios | Ante una sobrecarga de un servidor, se puede mantener la disponibilidad y performance estable | Complejidad en la sincronizacion de sesiones | QA06, QA07 |
| Cola de Prioridad | Mejores tiempos de respuesta ante consultas al dashboard | Introduce intermediarios, puede retrasar las escrituras demasiado (starvation) | QA06, QA07 |
| Procesamiento secuencial sin colas | Sin capas intermedias, mejor performance en casos generales | No permite agregar concurrencia, bloqueo de mucho tiempo ante grandes escrituras | QA07 |
| Base de Datos Relacional + Caché | Soporta lecturas masivas, simplicidad en el modelo de datos (vs NoSQL) | Costo en replicación y almacenamiento de grandes volumenes de datos | QA05 |
| Base de Datos NoSQL | Flexibilidad y mejor escalamiento horizontal, soporte nativo de grandes volumenes de datos | Mayor curva de aprendizaje para el equipo, dificil realizar referencias | QA05 |


Para satisfacer tanto los QA sobre las operaciones de usuarios como los de escalabilidad en usuarios y en pos de dinamizar la arquitectura Cliente-Servidor descentralizando la lógica, se opta por un diseño que combina balanceo de carga y priorización de operaciones. Además se toma una base relacional para aprovechar su simplicidad y uso general y se lo combina con una caché.

Elegidos: "Load balancer de usuarios", "Cola de prioridad", "Base de datos relacional + Caché"

## PASO 5 - Instanciar elementos, asignar responsabilidades y definir interfaces

El load balancer actua como gateway al trafico de usuarios, distribuyendo dinamicamente en distintas instancias de servidores.

El servidor que trate los usuarios debe procesar su solicitud y encolarla en la cola de prioridad la cual clasifica como prioritarias las lecturas (son mas rápidas) y luego las escrituras manuales.

Para aprovechar la separación de lecturas y escrituras se coloca una cache para almacenar datos de consultas frecuentes, evitando acceder a la base en cada lectura. Para que esto funcione, las escrituras deben realizarse sobre una base primaria que mantenga consistente las replicas (Sobreescribiendo) y la caché (invalidando)


## PASO 6 - Documentación:

### Vistas
**Descomposición del sistema (D. Módulos):**

```mermaid
graph TD
    A((Trotamundos)) ---- B((Conexión APIs))
    A ---- C((Lógica Trotamundos))
    C ---- H((Manejo de usuarios))
    C ---- D((Operaciones del usuario))
    C ---- G((Presentación dashboard))
    D ---- E((Lectura de arreglos))
    D ---- F((Escritura manual))
    C ---- I((Funcionalidades adicionales))

```
**Leyenda (vista de módulos):** cada círculo es un módulo del sistema. Una línea indica "contiene a" (descomposición), el módulo de arriba contiene al de abajo.

**Arquitectura tratada en la iteracion (C&C):**

```mermaid
flowchart TD
    %% Clientes externos (no es un componente propio en ejecución, va con estilo neutro)
    WebUI["Web App / Mobile App"]

    %% Balanceador de carga
    LB["Load Balancer"]

    %% Recuadro exclusivo para agrupar las instancias stateless
    subgraph SRV ["Instancias stateless del servidor"]
        S1["Backend Instance 1<br/>(Trotamundos API)"]
        S2["Backend Instance 2<br/>(Trotamundos API)"]
        Sn["Backend Instance N<br/>(Auto-scaling)"]
    end

    %% Cola de priorización
    Queue["Priority Queue<br/>---<br/>1. Prioridad Alta: Lecturas (Read FIFO)<br/>2. Prioridad Baja: Escrituras (Write FIFO)"]

    %% Almacenes de datos (todos en forma de cilindro)
    Cache[("Caché")]
    DB_Primary[("Primary DB<br/>(Escrituras)")]
    DB_Replica[("DB Read Replicas<br/>(Lecturas secundarias)")]

    %% Flujos y Conexiones (Líneas continuas = sincrónicas)
    WebUI -->|"HTTPS"| LB
    LB -->|"Distribución de Tráfico"| S1
    LB -->|"Distribución de Tráfico"| S2
    LB -->|"Distribución de Tráfico"| Sn

    S1 & S2 & Sn -->|"Encola operaciones (Lecturas/Escrituras)"| Queue

    Queue -->|"Lecturas"| Cache
    Queue -->|"Escrituras"| DB_Primary

    %% Líneas punteadas (Cache miss e invalidación)
    DB_Primary -.->|"Invalida datos cambiados"| Cache
    Cache -.->|"Cache Miss"| DB_Replica

    %% Línea doble (Replicación)
    DB_Primary ==>|"Replicación de Datos"| DB_Replica

    %% Aplicación de estilos según leyenda
    classDef propio fill:#0044cc,stroke:#002266,stroke-width:2px,color:#ffffff;
    classDef externo fill:#2b2b2b,stroke:#666666,stroke-width:1px,color:#ffffff;

    class WebUI externo;
    class LB,S1,S2,Sn,Queue,Cache,DB_Primary,DB_Replica propio;
```
**Leyenda (vista C&C):**
- Cajas azules: componentes en ejecución del sistema propio.
- Cilindros: almacenes de datos.
- Línea doble: replicación de datos de la primaria a las réplicas.
- Línea punteada: fallo de caché que va a una réplica, e invalidación de caché tras una escritura.
- El recuadro agrupa las instancias idénticas del servidor, que es stateless.

Se muestran los cambios arquitectónicos realizados en la iteración (distribución de tráfico, encolamiento con priorización y estrategia de persistencia)

### ADRs generados

- [ADR08 - Balanceo de carga](../adr/08-LoadBalancer.md)
- [ADR09 - Lecturas y escrituras: caché primero, pools separados y admisión](../adr/09-ColaPrioridad.md)
- [ADR10 - Uso de BBDD relacional cacheada](../adr/10-DBRelacionalCache.md)
- [ADR11 - Particionado por idViajero](../adr/11-ShardingDBporViajero.md)

## PASO 7 - Revisar la aplicación del método

En esta segunda iteración, se trataron temas de escalabilidad como prioritarios. Los tacticas seleccionadas ayudan a satisfacer los 3 QA seleccionados, cada tactica se adapta para poder cumplir con el QA que beneficia sin perjudicar fuertemente al resto.

En el diagrama de descomposicion se ve abordado la parte principal de operaciones que puede hacer el usuario sobre la base de datos. Para tener en cuenta, aun no se trató como asegurar la latencia tanto a web como mobile, aunque las decisiones de esta iteración afecten positivamente los requerimientos de performance de estos escenarios.

# Contraste LLM - Iteración 3 (Fuera del método)

El LLM optó por elegir más drivers sobre el performance (Web y Mobile), tiene sentido su elección porque forma parte de la velocidad con la que llegan los datos. Nosotros no lo tomamos pensando esa parte unicamente como parte del front-end. (Incluso, el LLM en el paso 7 dice que el escenario QA04, no lo abordó correctamente y QA03 quedo parcialmente cubierto)

El LLM pensó en la misma idea de que las lecturas son prioritarias para cumplir las medidas de respuesta de los drivers, al igual que la idea de tener réplicas + balanceador de carga (Lo llamó "Service-Based: servicios de grano grueso y sin estado, con balanceador y autoscaling", pero es la misma idea).

Acerca de la priorización de lecturas por sobre escrituras, nosotros usamos una cola de prioridad por tipo de operación y FIFO. El LLM usó el estilo CQRS, el cual separa las lecturas y escrituras en dos modelos distintos, es la misma idea que planteamos nosotros, pero le dió el nombre al estilo.

El LLM sugirió particionar la base de datos por viajeroID, el equipo no sabía que esto se podía hacer pero es una lógica para asegurar que un usuario solo consulte a única replica.

ADRs a agregar:
Parte del ADR-06 de división de réplicas por idViajero