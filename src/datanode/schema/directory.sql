-- DDL de la partición global directory ◇ [ADR-11]: resuelve idViajero antes de conocer el shard.
CREATE TABLE IF NOT EXISTS traveler_directory (
    email TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS itinerary_directory (
    id_itinerario TEXT PRIMARY KEY,
    id_viajero_dueno TEXT NOT NULL
);
-- Cuentas de correo vinculadas: permite al planificador de sondeo enumerarlas sin recorrer los shards [ADR-05][ADR-11].
CREATE TABLE IF NOT EXISTS linked_account_directory (
    id_cuenta TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL,
    proveedor TEXT NOT NULL
);
-- Avisos de proveedores degradados (E2) expuestos como avisosProveedores en el dashboard [ADR-07].
-- id_viajero NULL = el proveedor falla para todos.
CREATE TABLE IF NOT EXISTS provider_notices (
    id_proveedor TEXT PRIMARY KEY,
    id_viajero TEXT,
    mensaje TEXT NOT NULL,
    proximo_intento TEXT NOT NULL
);
