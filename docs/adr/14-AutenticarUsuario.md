---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Autenticación del usuario mediante email y contraseña con manejo de Sesiones/Tokens

## Contexto y Problema

Al abordar la confianza y seguridad del usuario en el sistema (Iteración 4), surge la necesidad de controlar de forma estricta quién accede a la plataforma, qué datos se muestran y qué transferencias ocurren entre los usuarios. Dado que Trotamundos manejara información sensible, es necesario establecer una barrera inicial que verifique la identidad del usuario antes de otorgarle acceso a los recursos del sistema y permitir el uso de conexiones externas.

## Decision Drivers

- QA08
- QA09

## Opciones Consideradas

- Autenticar usuario
- Detectar intruso (Monitoreo de comportamiento malicioso)

## Resultado de la decisión

Opción elegida: Autenticar usuario (Email y contraseña), porque es un método rápido de validar que asegura la identidad del usuario antes de otorgar acceso.

### Consecuencias

- Bueno, porque valida rápidamente la identidad y protege los endpoints sensibles (itinerarios y configuraciones de email) contra accesos anónimos.
- Bueno, porque es un estándar de la industria fácil de integrar y sin complejidad tecnica excesiva.
- Malo, porque requiere almacenar de manera segura y cifrada las credenciales en la base de datos de usuarios.

### Confirmación

Para confirmar el correcto cumplimiento de este ADR, se deberá verificar mediante pruebas de acceso que:
- Un usuario pueda autenticarse exitosamente.
- Solo los usuarios con sesión activa tengan acceso a su información, bloqueando cualquier intento de acceso no logueado.

## Pros y Cons de las opciones

### Detectar intruso

- Bueno, porque permite rechazar amenazas a tiempo evaluando comportamientos sospechosos o anómalos dentro de la plataforma.
- Malo, porque genera un overhead por chequeo ante cada acceso, impactando negativamente en la latencia del sistema.
- Malo, porque agrega la necesidad de mantener una base de datos con firmas de comportamiento malicioso, sumando una complejidad arquitectonica innecesaria para el alcance actual. 

## Más información

En el ADR16 se detalla la implementación y políticas de los tokens de vida corta utilizados en este proceso de autenticación.