---
estado: aceptada
fecha: 2026-11-20
fuente: Grupo
---

# Recomendación basada en contenido

## Contexto y Problema

El sistema de recomendaciones debe sugerir alternativas de alojamientos, transportes y rutas personalizadas para cada viajero a partir de su historial, sus preferencias y el feedback que brinde sobre sugerencias anteriores (human-in-the-loop).
## Decision Drivers

- US08 - Sistema de Recomendación
- QA17 - Performance: Generación de recomendaciones
- QA18 - Evolvability: Retroalimentación del usuario

## Opciones Consideradas

- Sistema de recomendación basado en contenido
- Sistema de recomendación por filtrado colaborativo

## Resultado de la decisión

Opción elegida: Sistema de recomendación basado en contenido, porque no depende de la actividad de otros usuarios y resulta más simple de implementar para el prototipo. Al trabajar directamente con los metadatos de los arreglos provistos por los proveedores y las preferencias almacenadas en el perfil del viajero, permite incorporar de forma directa el feedback del usuario para refinar la próxima recomendación (QA18).

### Consecuencias

- Bueno, porque funciona sin necesidad de contar con un gran volumen de interacciones de otros usuarios
- Bueno, porque facilita ajustar las recomendaciones inmediatamente a partir de las preferencias y el feedback del viajero
- Malo, porque ofrece poca diversidad en las recomendaciones al enfocarse solo en elementos similares al historial previo
- Malo, porque la calidad de las sugerencias depende directamente de que los metadatos provistos por los proveedores sean completos y precisos

### Confirmación

Se deberá verificar durante la implementación que el módulo de recomendaciones consuma los metadatos de la lista de arreglos de los proveedores junto con el historial y perfil de preferencias del viajero, y que al recibir una interacción del usuario (aceptacion, rechazo o ajuste de parametros), estos cambios se reflejen en la siguiente recomendacion generada.

## Pros y Cons de las opciones

### Sistema de recomendación por filtrado colaborativo

- Bueno, porque descubre preferencias emergentes y ofrece mayor diversidad en las sugerencias
- Malo, porque presenta el problema de cold start y requiere un gran volumen de actividad de otros usuarios para un buen funcionamiento