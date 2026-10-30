---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Personalización de interfaz mediante almacenamiento en cliente (LocalStorage)

## Contexto y Problema

Durante el diseño se definió que el servidor funcionara por medio de una arquitectura stateless (sin estado) para maximizar el rendimiento. Sin embargo, para ofrecer una interfaz amigable, el sistema debe recordar las preferencias visuales del usuario (modo claro/oscuro o el orden de visualización).

## Decision Drivers

- QA11

## Opciones Consideradas

- Maintain user model en el cliente (LocalStorage)
- Almacenar las preferencias de presentación visual en la base de datos (Backend)

## Resultado de la decisión

Opción elegida: Maintain user model en el cliente (LocalStorage / AsyncStorage), porque permite adaptar la interfaz de manera instantánea en el dispositivo del usuario sin trafico de red adicional. Ademas al tener las preferencias en la memoria del dispositivo, el renderizado es inmediato y favorece una interfaz centrada en el viaje y amigable, sin sacrificar los tiempos de respuesta.

### Consecuencias

- Bueno, porque el sistema muestra una experiencia más personalizada basada en preferencias
- Bueno, porque permite no almacenar ni tener que consultar datos puramente orientados a la estética, manteniendo la arquitectura stateless
- Malo, porque la personalización queda atada al dispositivo físico

### Confirmación

Para confirmar esta decisión, durante el desarrollo se deberá probar que:
- Los cambios de interfaz (como el modo oscuro o el orden de los viajes) se guarden en el dispositivo del usuario.
- Al abrir la app, la pantalla se dibuje usando estas preferencias al instante, antes de pedir cualquier dato al backend.

## Pros y Cons de las opciones

### Almacenar las preferencias de presentación visual en la base de datos (Backend)

- Bueno, porque el usuario tendría exactamente la misma experiencia visual sincronizada sin importar desde donde ingresa al sistema
- Malo, porque obliga a descaragr informacion adicional antes de mostrar el Dashboard, perjudicando el tiempo de despliegue inicial

