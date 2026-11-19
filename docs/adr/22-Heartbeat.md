---
estado: aceptada
fecha: 2026-11-19
fuente: Grupo
---

# Healthchecks para réplicas del backend y base de datos

## Contexto y Problema

Con el patrón de despliegue definido en el [ADR21](./21-Deployment.md), el sistema necesita detectar cuándo un nodo cae para redistribuir la carga o conmutar a la instancia pasiva. Sin un mecanismo de detección, la redundancia no alcanza para cumplir [QA04](../01-drivers/ArchitecturalDrivers.md#escenarios-de-atributos-de-calidad).

El alcance del problema es el monitoreo de salud entre el balanceador y las réplicas del monolito modular, y entre el nodo de la cola de prioridades y la base de datos.

## Decision Drivers

- QA04 - Inactividad no planificada

## Opciones Consideradas

- Heartbeat
- Ping/echo

## Resultado de la decisión

Opción elegida: Heartbeat, porque permite controlar el tiempo de detección de una caída mediante la configuración del timeout de forma centralizada y alivia el uso de red frente a ping/echo, al generar tráfico adicional en una sola dirección.

Se decidió aplicarlo en ambos puntos:

- **Backend:** el balanceador de carga monitorea los heartbeats producidos por las réplicas. La frecuencia de pulsos y la tolerancia a heartbeats perdidos se configura de manera tal que garantice detectar una caída real por debajo del minuto, respentando medida de respuesta en [QA04](../01-drivers/ArchitecturalDrivers.md#escenarios-de-atributos-de-calidad). Ante una caída, solo se redistribuye la carga a las demás réplicas hasta que la caída se recupere (recuperación es responsabilidad del clúster).
- **Datos:** el nodo de la cola de prioridades monitorea los heartbeats de ambas réplicas de nodo de datos. Ante un fallo, redirige el tráfico a la instancia de redundancia pasiva, convirtiéndose esta en primaria y activando ella misma un heartbeat y sincronizaciones. A esto le llamamos intercambio de responsabilidades. La réplica principal, entonces, pasa a considerarse pasiva.

### Consecuencias

- Bueno, porque el tiempo de detección se controla de forma centralizada con el timeout del balanceador.
- Bueno, porque el tráfico es unidireccional y consume menos red que ping/echo.
- Bueno, porque la réplica o la base de datos es proactiva al informar su estado.
- Neutral, porque existe riesgo de saturación si no hay sincronización entre los heartbeats.
- Neutral, porque el tiempo de reconocimiento de una caída es variable.
- Neutral, hay que ajustar frecuencia y timeout para no declarar caído a un nodo ante una demora puntual.

### Confirmación

Para confirmar el cumplimiento de este ADR se debe verificar que:

- Al apagar una réplica del backend, el balanceador la detecta en menos de un minuto y deja de enviarle tráfico, redistribuyéndolo entre las restantes.
- Al cortar los heartbeats del nodo de datos primario, el nodo de la cola de prioridades redirige el tráfico a la instancia pasiva y la notifica del intercambio de responsabilidades. Posteriormente la nueva instancia primaria comienza a enviar sincronizaciones a la pasiva (una vez que pueda recibirlas)
- Los tiempos medidos cumplen la medida de respuesta de QA04.

## Pros y Cons de las opciones

### Ping/echo

- Bueno, porque el control queda centralizado en el balanceador/cola.
- Bueno, porque se integra fácilmente con el balanceador.
- Malo, porque el tráfico es bidireccional.
- Malo, porque es sensible a la latencia.