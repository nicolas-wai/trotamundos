// Package ports contiene los puertos de AD §6.4 y los puertos de salida adicionales del prototipo.
// Solo describen contratos; el núcleo (domain/application) depende únicamente de estas interfaces.
// Concern de tecnologías genéricas: ningún puerto expone tipos de librerías concretas.
// [ADR-02][ADR-05] Es la frontera entre módulos y contenedores: Ports & Adapters, sin dependencias hacia adapters ni stubs.
package ports
