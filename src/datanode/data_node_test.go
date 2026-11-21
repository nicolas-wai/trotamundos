package datanode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

func newNode(t *testing.T, id string, role Role) *DataNode {
	t.Helper()
	n, err := NewDataNode(Config{ID: id, Role: role, Dir: t.TempDir(), Shards: 3,
		Heartbeat: stubs.NewInMemoryHeartbeatChannel(time.Second, 3), HeartbeatInterval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(n.Close)
	return n
}

func TestPartitionRouterIsStableAndRoutesDirectoryApart(t *testing.T) {
	r := NewPartitionRouter(4)
	first, _ := r.ShardFor("viajero-1")
	for i := 0; i < 20; i++ {
		if s, _ := r.ShardFor("viajero-1"); s != first {
			t.Fatal("el mismo idViajero debe ir siempre al mismo shard")
		}
	}
	seen := map[int]bool{}
	for i := 0; i < 200; i++ {
		s, err := r.ShardFor(string(rune('a'+i%26)) + string(rune('A'+i/26)))
		if err != nil || s < 0 || s >= 4 {
			t.Fatalf("shard fuera de rango: %d %v", s, err)
		}
		seen[s] = true
	}
	if len(seen) != 4 {
		t.Fatalf("la distribución debería usar los 4 shards, usó %d", len(seen))
	}
	if p, _ := r.PartitionFor("", operations.ScopeDirectory); p != DirectoryPartition {
		t.Fatalf("directory debía ir a su partición, fue %s", p)
	}
	if _, err := r.PartitionFor("", operations.ScopeShard); err == nil {
		t.Fatal("un idViajero vacío no se puede enrutar a un shard")
	}
	if parts := r.Partitions(); len(parts) != 5 {
		t.Fatalf("4 shards + directory, hubo %v", parts)
	}
}

func write(t *testing.T, n *DataNode, id, res string, p map[string]any) {
	t.Helper()
	if _, err := n.Write(context.Background(), id, ports.DataCommand{Resource: res, Params: p}); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, n *DataNode, id, res string, p map[string]any) ports.DataResult {
	t.Helper()
	r, err := n.Read(context.Background(), id, ports.DataQuery{Resource: res, Params: p})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func traveler(id, email string) map[string]any {
	return map[string]any{"idViajero": id, "email": email, "encryptedPassword": "hash", "createdAt": "2026-01-01T00:00:00Z"}
}

// [ADR-10] Escritura → primaria → E6 → réplica de lectura e invalidación; lectura cache-first.
func TestWriteIsReplicatedAndReadIsCachedThenInvalidated(t *testing.T) {
	n := newNode(t, "n1", RoleActive)
	var e6 []events.DataWritten
	n.Subscribe(func(e events.DataWritten) { e6 = append(e6, e) })

	write(t, n, "v1", operations.TravelerCreate, traveler("v1", "a@x.com"))
	if len(e6) != 1 || e6[0].IDViajero != "v1" || e6[0].Entidad != "PERFIL" || e6[0].Version != 1 || len(e6[0].Claves) != 1 {
		t.Fatalf("E6 inesperado: %+v", e6)
	}

	q := map[string]any{"idViajero": "v1"}
	r := read(t, n, "v1", operations.TravelerFindByID, q) // miss → réplica
	if len(r.Rows) != 1 || r.Rows[0]["email"] != "a@x.com" {
		t.Fatalf("la réplica debía tener la escritura: %+v", r)
	}
	if h, m := n.Cache().Stats(); h != 0 || m != 1 {
		t.Fatalf("primer acceso: hits=%d misses=%d", h, m)
	}
	read(t, n, "v1", operations.TravelerFindByID, q) // hit
	if h, _ := n.Cache().Stats(); h != 1 {
		t.Fatalf("segundo acceso debía ser hit, hits=%d", h)
	}

	// Un refresh token del mismo viajero invalida la entrada cacheada del viajero (misma clave de partición).
	write(t, n, "v1", operations.RefreshTokenSave, map[string]any{"tokenId": "t1", "idViajero": "v1", "expiresAt": "2030-01-01T00:00:00Z"})
	read(t, n, "v1", operations.TravelerFindByID, q)
	if _, m := n.Cache().Stats(); m != 2 {
		t.Fatalf("tras escribir se esperaba un nuevo miss, misses=%d", m)
	}
}

func TestUniqueViolationIsConflictAndUnknownResourceFails(t *testing.T) {
	n := newNode(t, "n1", RoleActive)
	d := map[string]any{"email": "a@x.com", "idViajero": "v1"}
	write(t, n, "", operations.DirectoryTravelerReserve, d)
	_, err := n.Write(context.Background(), "", ports.DataCommand{Resource: operations.DirectoryTravelerReserve, Params: map[string]any{"email": "a@x.com", "idViajero": "v2"}})
	if !errors.Is(err, ports.ErrDataConflict) {
		t.Fatalf("se esperaba ErrDataConflict, hubo %v", err)
	}
	if _, err := n.Write(context.Background(), "v1", ports.DataCommand{Resource: "no.existe"}); !errors.Is(err, ports.ErrUnknownResource) {
		t.Fatalf("recurso desconocido: %v", err)
	}
	// una lectura no puede invocarse como escritura
	if _, err := n.Write(context.Background(), "v1", ports.DataCommand{Resource: operations.TravelerFindByID, Params: map[string]any{"idViajero": "v1"}}); !errors.Is(err, ports.ErrUnknownResource) {
		t.Fatalf("tipo equivocado: %v", err)
	}
	// parámetro faltante
	if _, err := n.Write(context.Background(), "v1", ports.DataCommand{Resource: operations.TravelerCreate, Params: map[string]any{}}); err == nil {
		t.Fatal("faltaban parámetros")
	}
}

func TestDifferentTravelersLiveInTheirOwnShardAndDirectoryIsSeparate(t *testing.T) {
	n := newNode(t, "n1", RoleActive)
	ids := []string{"v1", "v2", "v3", "v4", "v5", "v6"}
	for _, id := range ids {
		write(t, n, id, operations.TravelerCreate, traveler(id, id+"@x.com"))
		write(t, n, "", operations.DirectoryTravelerReserve, map[string]any{"email": id + "@x.com", "idViajero": id})
	}
	for _, id := range ids {
		if r := read(t, n, id, operations.TravelerFindByID, map[string]any{"idViajero": id}); len(r.Rows) != 1 {
			t.Fatalf("%s no encontrado", id)
		}
		if r := read(t, n, "", operations.DirectoryTravelerLookup, map[string]any{"email": id + "@x.com"}); len(r.Rows) != 1 || r.Rows[0]["id_viajero"] != id {
			t.Fatalf("directorio de %s: %+v", id, r)
		}
	}
}

// [ADR-21] SyncTo instala el estado de la primaria en el pasivo, que lo sirve tras promoverse.
func TestSyncToPassiveAndPromotion(t *testing.T) {
	active, passive := newNode(t, "a", RoleActive), newNode(t, "p", RolePassive)
	active.SetPeer(passive)
	write(t, active, "v1", operations.TravelerCreate, traveler("v1", "a@x.com"))
	write(t, active, "", operations.DirectoryTravelerReserve, map[string]any{"email": "a@x.com", "idViajero": "v1"})

	e5, err := active.SyncNow(context.Background())
	if err != nil || e5.Origen != "a" || e5.Destino != "p" || e5.VersionEstado < 2 {
		t.Fatalf("E5 inesperado: %+v %v", e5, err)
	}
	passive.Promote()
	if r := read(t, passive, "v1", operations.TravelerFindByID, map[string]any{"idViajero": "v1"}); len(r.Rows) != 1 {
		t.Fatalf("el pasivo debía leer los datos sincronizados: %+v", r)
	}
	if r := read(t, passive, "", operations.DirectoryTravelerLookup, map[string]any{"email": "a@x.com"}); len(r.Rows) != 1 {
		t.Fatal("el directorio también debía sincronizarse")
	}
	// y el pasivo promovido puede escribir con normalidad
	write(t, passive, "v2", operations.TravelerCreate, traveler("v2", "b@x.com"))
}

func TestStoppedNodeIsUnavailable(t *testing.T) {
	n := newNode(t, "n1", RoleActive)
	n.Stop()
	if _, err := n.Read(context.Background(), "v1", ports.DataQuery{Resource: operations.TravelerFindByID}); !errors.Is(err, ports.ErrDataNodeUnavailable) {
		t.Fatalf("lectura: %v", err)
	}
	if _, err := n.Write(context.Background(), "v1", ports.DataCommand{}); !errors.Is(err, ports.ErrDataNodeUnavailable) {
		t.Fatalf("escritura: %v", err)
	}
	if _, err := n.SyncNow(context.Background()); err == nil {
		t.Fatal("sin par no hay sincronización")
	}
}

func TestHeartbeatCarriesCurrentRole(t *testing.T) {
	hb := stubs.NewInMemoryHeartbeatChannel(10*time.Millisecond, 3)
	n, err := NewDataNode(Config{ID: "n", Role: RolePassive, Dir: t.TempDir(), Shards: 1, Heartbeat: hb, HeartbeatInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.Run(ctx)
	waitFor(t, func() bool { l, ok := hb.Last("n"); return ok && l.Rol == "PASIVO" && l.TipoNodo == "DATOS" })
	n.Promote()
	waitFor(t, func() bool { l, _ := hb.Last("n"); return l.Rol == "ACTIVO" })
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condición no cumplida a tiempo")
}
