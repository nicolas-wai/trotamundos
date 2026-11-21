-- DDL de cada shard [ADR-10][ADR-11]. Todas las tablas particionadas llevan id_viajero.
CREATE TABLE IF NOT EXISTS travelers (
    id_viajero TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    encrypted_password TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_id TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_viajero ON refresh_tokens(id_viajero);
CREATE TABLE IF NOT EXISTS preferences (
    id_viajero TEXT PRIMARY KEY,
    recommendation_params TEXT NOT NULL DEFAULT '{}',
    accommodation_preferences TEXT NOT NULL DEFAULT '{}',
    transport_preferences TEXT NOT NULL DEFAULT '{}',
    route_preferences TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS linked_email_accounts (
    id_cuenta TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL,
    proveedor TEXT NOT NULL,
    encrypted_access_token BLOB,
    encrypted_refresh_token BLOB,
    estado TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS trips (
    id_viaje TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL,
    nombre TEXT NOT NULL,
    destino TEXT NOT NULL,
    fecha_inicio TEXT NOT NULL,
    fecha_fin TEXT NOT NULL,
    estado TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS arrangements (
    id_arreglo TEXT PRIMARY KEY,
    id_viaje TEXT NOT NULL,
    id_viajero TEXT NOT NULL,
    tipo TEXT NOT NULL,
    titulo TEXT NOT NULL,
    origen TEXT NOT NULL,
    remitente TEXT,
    estado TEXT NOT NULL,
    detalle TEXT NOT NULL DEFAULT '{}',
    clave_deduplicacion TEXT,
    fecha_inicio TEXT,
    fecha_fin TEXT,
    generado_en_fuente TEXT,
    recibido_en TEXT,
    actualizado_en TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_arrangements_viajero ON arrangements(id_viajero);
CREATE TABLE IF NOT EXISTS shared_itineraries (
    id_itinerario TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL,
    nombre TEXT NOT NULL,
    link_publico TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS access_permissions (
    id_itinerario TEXT NOT NULL,
    id_viajero TEXT NOT NULL,
    id_viajero_autorizado TEXT NOT NULL,
    estado TEXT NOT NULL,
    PRIMARY KEY (id_itinerario, id_viajero_autorizado)
);
CREATE TABLE IF NOT EXISTS access_requests (
    id_solicitud TEXT PRIMARY KEY,
    id_itinerario TEXT NOT NULL,
    id_viajero TEXT NOT NULL,
    id_viajero_solicitante TEXT NOT NULL,
    estado TEXT NOT NULL,
    fecha_solicitud TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS recommendations (
    id_recomendacion TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL,
    categoria TEXT NOT NULL,
    metadatos_item TEXT NOT NULL DEFAULT '{}',
    generada_en TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS recommendation_feedback (
    id_feedback TEXT PRIMARY KEY,
    id_recomendacion TEXT NOT NULL,
    id_viajero TEXT NOT NULL,
    accion TEXT NOT NULL,
    parametros_ajustados TEXT NOT NULL DEFAULT '{}',
    fecha TEXT NOT NULL
);
