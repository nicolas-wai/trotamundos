---
estado: aceptada
fecha: 2026-10-29
fuente: Grupo
---

# Conexiones seguras

## Contexto y Problema

Dado de que el usuario puede llegar a acceder a fuentes desconocidas e inseguras, la idea es darle la mejor experiencia de usuario, por lo que es fundamental que confíe en el servicio que se ofrece

## Decision Drivers

- QA08- Security: Acceso a email

## Opciones Consideradas

- Encriptar datos
- Integridad de mensaje

## Resultado de la decisión

Opción elegida: Encriptar datos, mediante el uso de https, lo que va a chequear las credenciales para garantizar la seguridad de los datos

### Consecuencias

- Bueno, porque protege la información en tránsito
- Bueno, porque protege la información de la base de datos (Afecta QA09)
- Bueno, porque el usuario reconoce fácilmente que sus datos están seguros
- Malo, porque añade complejidad por el cifrado y descifrado de los mensajes
- Malo, porque requiere una gestión de claves

### Confirmación

No permite conectarse mediante http, sino que requiere conexión mediante https. Las responses del curl al sitio devuelven mensajes en protocolo https.

## Pros y Cons de las opciones

### Integridad de mensaje

- Bueno, porque asegura que los mensajes e itinerarios no sean alterados en tránsito
- Malo, porque aumenta la latencia en la integridad del mensaje
- Malo, porque suma complejidad en la gestión de claves/firmas
