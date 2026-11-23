// Package resources contiene los nombres de recurso del contrato E8 OperacionEncolada (AD §6.3).
// [ADR-09][ADR-10] Es el vocabulario compartido entre la réplica (que encola operaciones por nombre) y el
// nodo de datos (que resuelve cada nombre a SQL plano en su catálogo de operaciones). Así la réplica no
// conoce los internos del nodo de datos.
package resources

// Nombres de recursos de Autenticación (Fase 2).
const (
	DirectoryTravelerReserve = "directory.traveler.reserve"
	DirectoryTravelerLookup  = "directory.traveler.lookup"
	DirectoryTravelerRelease = "directory.traveler.release"
	TravelerCreate           = "traveler.create"
	TravelerFindByID         = "traveler.find_by_id"
	RefreshTokenSave         = "refresh_token.save"
	RefreshTokenFind         = "refresh_token.find"
	RefreshTokenRevoke       = "refresh_token.revoke"
)

// Nombres de recursos de Arreglos de viaje (Fase 3).
const (
	TripCreate                     = "trip.create"
	TripList                       = "trip.list"
	TripFindByID                   = "trip.find_by_id"
	ArrangementListSummaries       = "arrangement.list_summaries"
	ArrangementFindByID            = "arrangement.find_by_id"
	ArrangementFindByDeduplication = "arrangement.find_by_deduplication_key"
	ArrangementSave                = "arrangement.save"
	ArrangementDelete              = "arrangement.delete"
)

// Nombres de recursos de la Fase 4 (vinculación de correo, ingesta de proveedores y avisos).
const (
	DirectoryTravelerList          = "directory.traveler.list"
	DirectoryLinkedAccountRegister = "directory.linked_account.register"
	DirectoryLinkedAccountList     = "directory.linked_account.list"
	DirectoryLinkedAccountRemove   = "directory.linked_account.remove"
	DirectoryProviderNoticeSave    = "directory.provider_notice.save"
	DirectoryProviderNoticeList    = "directory.provider_notice.list"
	DirectoryProviderNoticeClear   = "directory.provider_notice.clear"
	LinkedAccountSave              = "linked_account.save"
	LinkedAccountFind              = "linked_account.find"
	LinkedAccountDelete            = "linked_account.delete"
)

// Nombres de recursos de la Fase 5: itinerarios compartidos (Permisos) y recomendaciones.
const (
	DirectoryItineraryRegister = "directory.itinerary.register"
	DirectoryItineraryResolve  = "directory.itinerary.resolve"
	DirectoryItineraryRemove   = "directory.itinerary.remove"
	ItineraryCreate            = "itinerary.create"
	ItineraryFind              = "itinerary.find"
	ItineraryListByOwner       = "itinerary.list_by_owner" // Fase 7: "Mis itinerarios" del dashboard [ADR-13]
	ItineraryDelete            = "itinerary.delete"
	ItineraryArrangementAdd    = "itinerary.arrangement.add"
	ItineraryArrangementRemove = "itinerary.arrangement.remove"
	ItineraryArrangementList   = "itinerary.arrangement.list"
	ItineraryArrangementsClear = "itinerary.arrangement.clear"
	AccessPermissionSave       = "access_permission.save"
	AccessPermissionList       = "access_permission.list"
	AccessPermissionsClear     = "access_permission.clear"
	AccessRequestCreate        = "access_request.create"
	AccessRequestList          = "access_request.list"
	AccessRequestFindItinerary = "access_request.find_itinerary"
	AccessRequestUpdateStatus  = "access_request.update_status"
	AccessRequestsClear        = "access_request.clear"
	PreferencesGet             = "preferences.get"
	PreferencesSave            = "preferences.save"
	RecommendationFeedbackSave = "recommendation_feedback.save"
	RecommendationFeedbackList = "recommendation_feedback.list"
)
