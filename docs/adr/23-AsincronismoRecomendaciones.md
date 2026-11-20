---
estado: aceptada
fecha: 2026-11-20
fuente: Grupo
---

# Asincronismo en la carga del panel recomendaciones del Dashboard

## Contexto y Problema

Al incorporar el sistema de recomendaciones en el dashboard, surge una diferencia importante en los tiempos de respuesta: mientras que la carga principal de reservas debe resolverse rápidamente, la generación de recomendaciones requiere procesar el historial y las preferencias del viajero, pudiendo demorar hasta 3 segundos. Si el dashboard espera a que se generen las recomendaciones para mostrarse, el usuario percibirá lentitud al acceder a sus viajes, incumpliendo los tiempos de respuesta definidos para web y mobile.

## Decision Drivers

- US08 - Sistema de Recomendación
- QA17 - Performance: Generación de recomendaciones
- QA02 - Tiempo respuesta Web
- QA03 - Primer contenido mobile

## Opciones Consideradas

- Carga asíncrona del panel de recomendaciones en el Dashboard
- Carga síncrona de las recomendaciones junto con el resto del Dashboard

## Resultado de la decisión

Opción elegida: Carga asíncrona del panel de recomendaciones en el Dashboard, porque permite que el tiempo de generación de las recomendaciones no impacte en el flujo principal de carga. De esta manera el viajero puede ver sus reservas activas dentro de los tiempos esperados (QA02 y QA03) mientras el módulo de recomendaciones procesa y devuelve las sugerencias en un panel aparte dentro de su límite de 3 segundos (QA17).

### Consecuencias

- Bueno, porque la carga del dashboard no depende de las recomendaciones, que son más lentas que el resto de los elementos
- Bueno, porque el usuario puede consultar sus reservas en todo momento aunque el modulo de recomendaciones falle o se demore
- Malo, porque el usuario ve que una parte del dashboard ya cargó mientras el panel de recomendaciones todavía falta, genrando la necesidad de mostrar estados de carga en la interfaz

### Confirmación

Se deberá verificar durante la implementación y pruebas que el dashboard realice una petición independiente para el panel de recomendaciones, confirmando que las reservas se muestren en menos de 800ms en web y 1.4s en mobile sin quedar bloqueadas por la respuesta del módulo de recomendaciones.

## Pros y Cons de las opciones

### Carga asíncrona del panel de recomendaciones en el Dashboard

- Bueno, porque mantiene los tiempos de respuesta del dashboard principal (QA02/QA03 Positivo) y respeta el límite de recomendaciones (QA17 Positivo)
- Malo, porque genera una carga parcial visible en la pantalla y suma lógica en el cliente para manejar el estado de espera

### Carga síncrona de las recomendaciones junto con el resto del Dashboard

- Bueno, porque simplifica la implementación inicial al mostrar toda la pantalla en un solo paso
- Malo, porque el dashboard entero queda bloqueado esperando hasta 3 segundos por las recomendaciones (QA02/QA03 Negativo)