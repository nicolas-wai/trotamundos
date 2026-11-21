-- DDL de la partición global directory ◇ [ADR-11]: resuelve idViajero antes de conocer el shard.
CREATE TABLE IF NOT EXISTS traveler_directory (
    email TEXT PRIMARY KEY,
    id_viajero TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS itinerary_directory (
    id_itinerario TEXT PRIMARY KEY,
    id_viajero_dueno TEXT NOT NULL
);
