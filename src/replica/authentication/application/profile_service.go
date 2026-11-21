package application

import (
	"context"
	"fmt"
)

// ProfileService [ADR-25] Implementa TravelerProfileService: reemplaza los parámetros de recomendación del
// perfil (PUT /perfil/preferencias). Distintas de las preferencias visuales en LocalStorage [ADR-19].
type ProfileService struct{ preferences PreferencesRepository }

var _ TravelerProfileService = (*ProfileService)(nil)

// NewProfileService crea el servicio de perfil.
func NewProfileService(preferences PreferencesRepository) *ProfileService {
	return &ProfileService{preferences: preferences}
}

// UpdatePreferences reemplaza los parámetros de recomendación; las preferencias por categoría se conservan.
// El perfil aplica a la siguiente recomendación (QA-18).
func (s *ProfileService) UpdatePreferences(ctx context.Context, travelerID string, params map[string]any) error {
	if params == nil {
		return fmt.Errorf("%w: parametros es obligatorio", ErrValidation)
	}
	current, err := s.preferences.Get(ctx, travelerID)
	if err != nil {
		return err
	}
	current.TravelerID = travelerID
	current.RecommendationParams = params
	return s.preferences.Save(ctx, current)
}
