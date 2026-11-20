# Iteración 7 - Sistema de recomendacion

## PASO 1 - Revisar el input
Como la user storie del sistema de recomendación no tiene medidas claras para poner diseñar sobre ella, se construyen QAs para el mismo. Esto se debería hacer la inicio de una round, por simplicidad ya que solo es un unico driver se hace previo a esta iteración.

**QA17 - Performance: Generacion de recomendaciones**

El sistema de recomendación debe generarlas en un tiempo apropiado

- Fuente: Viajero con historial
- Estimulo: Solicituda de recomandacion al cargar el dashboard
- Artefacto: Modulo de recomendaciones, historial
- Enterno: Tiempo de ejecucion, en modo normal
- Respuesta: El sistema devuelve en el dashboard las alternativas personalizadas de alojamiento, transporte y rutas
- Medida de respuesta: El tiempo de generacion de respuestas maximo es de 3 segundos

**QA18 - Evolvability: Retroalimentacion del usuario**

El sistema de recomendacion debe utilizar una optimizacion human-in-the-loop para ajustar los parametros

- Fuente: Viajero
- Estimulo: Acepta, rechaza o ajusta parametros de una recomendacion
- Artefacto: Modulo de recomendaciones, parametros y preferencias
- Enterno: Funcionamiento normal, en ejecucion
- Respuesta: El sistema utiliza el feedback del usuario para refinar futuras recomendaciones
- Medida: El feedback debe usarse para la proxima recomendacion que se realice


## PASO 2 - Objetivo de la Iteración
Diseñar la arquitectura del modulo de recomendaciones para cumplir los requisitos de performance y retroalimentacion

**Drivers a desarrollar en la iteración:**
- US08 - Sistema de recomendacion
- QA17 - Performance: Generación de recomendaciones
- QA18 - Evolvability: Retroalimentacion del usuario

## PASO 3 - Descomposición
El principal elemento a descomponer es el sistema de recomendaciones el cual debe interactuar y tener un buen flujo con el dashboard y el historial de usuario


## PASO 4 - Aplicando conceptos de diseño a los drivers seleccionados

| Alternativa             | Pros                                               | Contras |
|---|---|---|
| Asincronismo en la carga de la sección del dashboard | La carga del dashboard no depende de las recomendaciones que son mas lentas que el resto de elementos | El usuario ve que parte del dashboard está cargado y una parte falta |
| Concurrencia | La consultas para obtener el historial del usuario se realiza en simultaneo en las diferentes | Mayor carga en los servidores |
| Sistema de recomendacion de filtrado colaborativo | Descubre preferencias de forma temprana | Tiene cold start, depende de los gustos de otros usuarios |
| Sisteam de recomendacion basado en contenido | No depende de la actividad del resto de los usuarios, el input son metadatos del item sencillos de leer | Pocas diversidad en las recomendaciones, para que funcione bien los metadatos deben ser de calidad|

Se eligen las alternativas de asincronismo, concurrencia para asegurar la performance. El sistema de recomendacion se elige el basado en contenido por la simplicidad en la implementacion.

## PASO 5 - Instanciar elementos, asignar responsabilidades y definir interfaces

El dashboard muestra las recomendaciónes en un panel aparte al resto de arreglos del viajero. A diferencia del resto, esta puede tardar mas en generarse, al ser asincrono.
El modulo de recomendaciones utiliza un algoritmo basado en contenido y se alimenta de las sugerencias del usuario y el historial del mismo, para obtener la informacion se realizan consultas concurrentes para recolectar la misma en menos tiempo.
Las preferencias se deben almacenar en el perfil del usuario para personalizar las recomendaciones.
El modulo de recomendaciones posee una lista de arreglos provista por distintos proveedores, cada elemento tiene los metadatos suficientes para poder filtrar a los gustos de cada viajero.

## PASO 6 - Documentación

### Vistas

### ADRs generados
Las decisiones de esta iteración se documentan en los siguientes Registros:
- [ADR23 - Asíncronismo en las recomendaciones](../adr/23-AsincronismoRecomendaciones.md)
- [ADR24 - Concurrencia en la consulta del historial](../adr/24-ConcurrenciaConsultaHistorial.md)
- [ADR25 - Recomendacion basado en contenido](../adr/25-RecomendacionBasadaEnContenido.md)

## PASO 7 - Revisar la aplicación del método
La descomposición del sistema de recomendación y las alternativas planteadas fue suficiente para cumplir los atributos de calidad planteados.

# Contraste LLM
