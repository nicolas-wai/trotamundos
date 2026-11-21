package scheduler

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// DirectoryTargetSource [ADR-05][ADR-09] Enumera los objetivos de sondeo leyendo la partición directory ◇ por la
// cola de prioridades: cada cuenta de correo vinculada (un objetivo EMAIL) y, para cada viajero registrado, la
// agencia y el sistema de viajes del prototipo.
type DirectoryTargetSource struct {
	data                 ports.DataGateway
	agencyProviderID     string
	travelSystemProvider string
}

var _ TargetSource = (*DirectoryTargetSource)(nil)

// NewDirectoryTargetSource crea la fuente de objetivos.
func NewDirectoryTargetSource(data ports.DataGateway) *DirectoryTargetSource {
	return &DirectoryTargetSource{data: data, agencyProviderID: domain.DefaultAgencyProviderID, travelSystemProvider: domain.DefaultTravelSystemProviderID}
}

// Targets lista los objetivos vigentes.
func (d *DirectoryTargetSource) Targets(ctx context.Context) ([]Target, error) {
	var out []Target
	accounts, err := d.data.Read(ctx, "", operations.DirectoryLinkedAccountList, nil)
	if err != nil {
		return nil, err
	}
	for _, row := range accounts.Rows {
		provider, _ := row["proveedor"].(string)
		traveler, _ := row["id_viajero"].(string)
		account, _ := row["id_cuenta"].(string)
		out = append(out, Target{Type: domain.ProviderEmail, Ref: ports.ProviderReference{
			ProviderID: domain.ProviderIDFor(domain.ProviderEmail, provider), TravelerID: traveler, AccountID: account,
		}})
	}
	travelers, err := d.data.Read(ctx, "", operations.DirectoryTravelerList, nil)
	if err != nil {
		return nil, err
	}
	for _, row := range travelers.Rows {
		traveler, _ := row["id_viajero"].(string)
		out = append(out,
			Target{Type: domain.ProviderAgency, Ref: ports.ProviderReference{ProviderID: d.agencyProviderID, TravelerID: traveler}},
			Target{Type: domain.ProviderTravelSystem, Ref: ports.ProviderReference{ProviderID: d.travelSystemProvider, TravelerID: traveler}},
		)
	}
	return out, nil
}
