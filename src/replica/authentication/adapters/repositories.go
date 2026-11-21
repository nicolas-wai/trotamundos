package adapters

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/resources"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// QueueTravelerRepository [ADR-09][ADR-11] Implementa TravelerRepository sobre la cola de prioridades.
// Toda lectura/escritura sale como E8 (DataGateway → PriorityQueuePort); el email único se resuelve
// en la partición directory ◇ antes de tocar el shard del viajero.
type QueueTravelerRepository struct{ data ports.DataGateway }

var _ application.TravelerRepository = (*QueueTravelerRepository)(nil)

// NewQueueTravelerRepository crea el repositorio.
func NewQueueTravelerRepository(data ports.DataGateway) *QueueTravelerRepository {
	return &QueueTravelerRepository{data: data}
}

// Create reserva el email en el directorio y crea el viajero en su shard (compensa si el segundo paso falla).
func (r *QueueTravelerRepository) Create(ctx context.Context, t domain.Traveler) error {
	dir := map[string]any{"email": t.Email, "idViajero": t.TravelerID}
	if _, err := r.data.Write(ctx, "", resources.DirectoryTravelerReserve, dir); err != nil {
		if errors.Is(err, ports.ErrDataConflict) {
			return application.ErrEmailTaken
		}
		return err
	}
	_, err := r.data.Write(ctx, t.TravelerID, resources.TravelerCreate, map[string]any{
		"idViajero": t.TravelerID, "email": t.Email, "encryptedPassword": t.EncryptedPassword,
		"createdAt": t.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		_, _ = r.data.Write(ctx, "", resources.DirectoryTravelerRelease, dir) // compensación
		return err
	}
	return nil
}

// FindByEmail resuelve el idViajero en el directorio y lee el viajero de su shard.
func (r *QueueTravelerRepository) FindByEmail(ctx context.Context, email string) (domain.Traveler, error) {
	res, err := r.data.Read(ctx, "", resources.DirectoryTravelerLookup, map[string]any{"email": email})
	if err != nil {
		return domain.Traveler{}, err
	}
	if len(res.Rows) == 0 {
		return domain.Traveler{}, application.ErrTravelerNotFound
	}
	id, _ := res.Rows[0]["id_viajero"].(string)
	return r.FindByID(ctx, id)
}

// FindByID lee el viajero de su shard.
func (r *QueueTravelerRepository) FindByID(ctx context.Context, travelerID string) (domain.Traveler, error) {
	res, err := r.data.Read(ctx, travelerID, resources.TravelerFindByID, map[string]any{"idViajero": travelerID})
	if err != nil {
		return domain.Traveler{}, err
	}
	if len(res.Rows) == 0 {
		return domain.Traveler{}, application.ErrTravelerNotFound
	}
	row := res.Rows[0]
	created, _ := time.Parse(time.RFC3339Nano, str(row["created_at"]))
	return domain.Traveler{
		TravelerID: str(row["id_viajero"]), Email: str(row["email"]),
		EncryptedPassword: str(row["encrypted_password"]), CreatedAt: created,
	}, nil
}

// QueueRefreshTokenRepository [ADR-16] Implementa RefreshTokenRepository sobre la cola de prioridades.
type QueueRefreshTokenRepository struct{ data ports.DataGateway }

var _ application.RefreshTokenRepository = (*QueueRefreshTokenRepository)(nil)

// NewQueueRefreshTokenRepository crea el repositorio.
func NewQueueRefreshTokenRepository(data ports.DataGateway) *QueueRefreshTokenRepository {
	return &QueueRefreshTokenRepository{data: data}
}

// Save persiste el refresh token.
func (r *QueueRefreshTokenRepository) Save(ctx context.Context, t domain.RefreshToken) error {
	_, err := r.data.Write(ctx, t.TravelerID, resources.RefreshTokenSave, map[string]any{
		"tokenId": t.TokenID, "idViajero": t.TravelerID, "expiresAt": t.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	return err
}

// FindByID lee el refresh token del shard del viajero.
func (r *QueueRefreshTokenRepository) FindByID(ctx context.Context, travelerID, tokenID string) (domain.RefreshToken, error) {
	res, err := r.data.Read(ctx, travelerID, resources.RefreshTokenFind, map[string]any{"tokenId": tokenID, "idViajero": travelerID})
	if err != nil {
		return domain.RefreshToken{}, err
	}
	if len(res.Rows) == 0 {
		return domain.RefreshToken{}, application.ErrRefreshNotFound
	}
	row := res.Rows[0]
	exp, _ := time.Parse(time.RFC3339Nano, str(row["expires_at"]))
	revoked, _ := row["revoked"].(int64)
	return domain.RefreshToken{TokenID: str(row["token_id"]), TravelerID: str(row["id_viajero"]), ExpiresAt: exp, Revoked: revoked != 0}, nil
}

// Revoke marca el refresh token como revocado.
func (r *QueueRefreshTokenRepository) Revoke(ctx context.Context, travelerID, tokenID string) error {
	_, err := r.data.Write(ctx, travelerID, resources.RefreshTokenRevoke, map[string]any{"tokenId": tokenID, "idViajero": travelerID})
	return err
}

func str(v any) string {
	s, ok := v.(string)
	if !ok && v != nil {
		return fmt.Sprint(v)
	}
	return s
}

// QueueLinkedEmailAccountRepository [ADR-09][ADR-15] Implementa ports.LinkedEmailAccountPort sobre la cola de
// prioridades. Los tokens llegan ya cifrados y se guardan en base64; además registra la cuenta en el directorio ◇
// para que el planificador de sondeo la encuentre sin recorrer los shards.
type QueueLinkedEmailAccountRepository struct{ data ports.DataGateway }

var _ ports.LinkedEmailAccountPort = (*QueueLinkedEmailAccountRepository)(nil)

// NewQueueLinkedEmailAccountRepository crea el repositorio.
func NewQueueLinkedEmailAccountRepository(data ports.DataGateway) *QueueLinkedEmailAccountRepository {
	return &QueueLinkedEmailAccountRepository{data: data}
}

// Save guarda la cuenta en el shard del viajero y la registra en el directorio.
func (r *QueueLinkedEmailAccountRepository) Save(ctx context.Context, a domain.LinkedEmailAccount) error {
	_, err := r.data.Write(ctx, a.TravelerID, resources.LinkedAccountSave, map[string]any{
		"idCuenta": a.AccountID, "idViajero": a.TravelerID, "proveedor": a.Provider,
		"encryptedAccessToken":  base64.StdEncoding.EncodeToString(a.EncryptedAccessToken),
		"encryptedRefreshToken": base64.StdEncoding.EncodeToString(a.EncryptedRefreshToken),
		"estado":                string(a.Status),
	})
	if err != nil {
		return err
	}
	_, err = r.data.Write(ctx, "", resources.DirectoryLinkedAccountRegister, map[string]any{
		"idCuenta": a.AccountID, "idViajero": a.TravelerID, "proveedor": a.Provider,
	})
	return err
}

// FindByID lee la cuenta del shard del viajero.
func (r *QueueLinkedEmailAccountRepository) FindByID(ctx context.Context, travelerID, accountID string) (domain.LinkedEmailAccount, error) {
	res, err := r.data.Read(ctx, travelerID, resources.LinkedAccountFind, map[string]any{"idCuenta": accountID, "idViajero": travelerID})
	if err != nil {
		return domain.LinkedEmailAccount{}, err
	}
	if len(res.Rows) == 0 {
		return domain.LinkedEmailAccount{}, ports.ErrLinkedAccountNotFound
	}
	row := res.Rows[0]
	access, _ := base64.StdEncoding.DecodeString(str(row["encrypted_access_token"]))
	refresh, _ := base64.StdEncoding.DecodeString(str(row["encrypted_refresh_token"]))
	return domain.LinkedEmailAccount{
		AccountID: str(row["id_cuenta"]), TravelerID: str(row["id_viajero"]), Provider: str(row["proveedor"]),
		EncryptedAccessToken: access, EncryptedRefreshToken: refresh, Status: domain.EmailAccountStatus(str(row["estado"])),
	}, nil
}

// Delete borra la cuenta del shard y del directorio.
func (r *QueueLinkedEmailAccountRepository) Delete(ctx context.Context, travelerID, accountID string) error {
	res, err := r.data.Write(ctx, travelerID, resources.LinkedAccountDelete, map[string]any{"idCuenta": accountID, "idViajero": travelerID})
	if err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ports.ErrLinkedAccountNotFound
	}
	_, err = r.data.Write(ctx, "", resources.DirectoryLinkedAccountRemove, map[string]any{"idCuenta": accountID, "idViajero": travelerID})
	return err
}

// QueuePreferencesRepository [ADR-09][ADR-11][ADR-25] Implementa PreferencesRepository sobre la cola de prioridades.
// Las preferencias de recomendación viven en el shard del viajero, en columnas JSON.
type QueuePreferencesRepository struct{ data ports.DataGateway }

var _ application.PreferencesRepository = (*QueuePreferencesRepository)(nil)

// NewQueuePreferencesRepository crea el repositorio.
func NewQueuePreferencesRepository(data ports.DataGateway) *QueuePreferencesRepository {
	return &QueuePreferencesRepository{data: data}
}

// Get lee las preferencias (LECTURA, prioridad alta). Un viajero sin fila tiene preferencias vacías.
func (r *QueuePreferencesRepository) Get(ctx context.Context, travelerID string) (domain.TravelerPreferences, error) {
	res, err := r.data.Read(ctx, travelerID, resources.PreferencesGet, map[string]any{"idViajero": travelerID})
	if err != nil {
		return domain.TravelerPreferences{}, err
	}
	prefs := domain.TravelerPreferences{
		TravelerID: travelerID, RecommendationParams: map[string]any{}, AccommodationPreferences: map[string]any{},
		TransportPreferences: map[string]any{}, RoutePreferences: map[string]any{},
	}
	if len(res.Rows) == 0 {
		return prefs, nil
	}
	row := res.Rows[0]
	for col, dst := range map[string]*map[string]any{
		"recommendation_params": &prefs.RecommendationParams, "accommodation_preferences": &prefs.AccommodationPreferences,
		"transport_preferences": &prefs.TransportPreferences, "route_preferences": &prefs.RoutePreferences,
	} {
		decoded := map[string]any{}
		if raw := str(row[col]); raw != "" {
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				return domain.TravelerPreferences{}, fmt.Errorf("preferencias corruptas (%s): %w", col, err)
			}
		}
		*dst = decoded
	}
	return prefs, nil
}

// Save reemplaza las preferencias del viajero (ESCRITURA, prioridad baja).
func (r *QueuePreferencesRepository) Save(ctx context.Context, p domain.TravelerPreferences) error {
	encode := func(m map[string]any) (string, error) {
		if m == nil {
			m = map[string]any{}
		}
		raw, err := json.Marshal(m)
		return string(raw), err
	}
	params := map[string]any{"idViajero": p.TravelerID}
	for name, m := range map[string]map[string]any{
		"recommendationParams": p.RecommendationParams, "accommodationPreferences": p.AccommodationPreferences,
		"transportPreferences": p.TransportPreferences, "routePreferences": p.RoutePreferences,
	} {
		raw, err := encode(m)
		if err != nil {
			return err
		}
		params[name] = raw
	}
	_, err := r.data.Write(ctx, p.TravelerID, resources.PreferencesSave, params)
	return err
}
