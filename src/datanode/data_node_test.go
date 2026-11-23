package datanode

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/contracts/resources"
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

	write(t, n, "v1", resources.TravelerCreate, traveler("v1", "a@x.com"))
	if len(e6) != 1 || e6[0].IDViajero != "v1" || e6[0].Entidad != "PERFIL" || e6[0].Version != 1 || len(e6[0].Claves) != 1 {
		t.Fatalf("E6 inesperado: %+v", e6)
	}

	q := map[string]any{"idViajero": "v1"}
	r := read(t, n, "v1", resources.TravelerFindByID, q) // miss → réplica
	if len(r.Rows) != 1 || r.Rows[0]["email"] != "a@x.com" {
		t.Fatalf("la réplica debía tener la escritura: %+v", r)
	}
	if h, m := n.Cache().Stats(); h != 0 || m != 1 {
		t.Fatalf("primer acceso: hits=%d misses=%d", h, m)
	}
	read(t, n, "v1", resources.TravelerFindByID, q) // hit
	if h, _ := n.Cache().Stats(); h != 1 {
		t.Fatalf("segundo acceso debía ser hit, hits=%d", h)
	}

	// Un refresh token del mismo viajero invalida la entrada cacheada del viajero (misma clave de partición).
	write(t, n, "v1", resources.RefreshTokenSave, map[string]any{"tokenId": "t1", "idViajero": "v1", "expiresAt": "2030-01-01T00:00:00Z"})
	read(t, n, "v1", resources.TravelerFindByID, q)
	if _, m := n.Cache().Stats(); m != 2 {
		t.Fatalf("tras escribir se esperaba un nuevo miss, misses=%d", m)
	}
}

func TestUniqueViolationIsConflictAndUnknownResourceFails(t *testing.T) {
	n := newNode(t, "n1", RoleActive)
	d := map[string]any{"email": "a@x.com", "idViajero": "v1"}
	write(t, n, "", resources.DirectoryTravelerReserve, d)
	_, err := n.Write(context.Background(), "", ports.DataCommand{Resource: resources.DirectoryTravelerReserve, Params: map[string]any{"email": "a@x.com", "idViajero": "v2"}})
	if !errors.Is(err, ports.ErrDataConflict) {
		t.Fatalf("se esperaba ErrDataConflict, hubo %v", err)
	}
	if _, err := n.Write(context.Background(), "v1", ports.DataCommand{Resource: "no.existe"}); !errors.Is(err, ports.ErrUnknownResource) {
		t.Fatalf("recurso desconocido: %v", err)
	}
	// una lectura no puede invocarse como escritura
	if _, err := n.Write(context.Background(), "v1", ports.DataCommand{Resource: resources.TravelerFindByID, Params: map[string]any{"idViajero": "v1"}}); !errors.Is(err, ports.ErrUnknownResource) {
		t.Fatalf("tipo equivocado: %v", err)
	}
	// parámetro faltante
	if _, err := n.Write(context.Background(), "v1", ports.DataCommand{Resource: resources.TravelerCreate, Params: map[string]any{}}); err == nil {
		t.Fatal("faltaban parámetros")
	}
}

func TestDifferentTravelersLiveInTheirOwnShardAndDirectoryIsSeparate(t *testing.T) {
	n := newNode(t, "n1", RoleActive)
	ids := []string{"v1", "v2", "v3", "v4", "v5", "v6"}
	for _, id := range ids {
		write(t, n, id, resources.TravelerCreate, traveler(id, id+"@x.com"))
		write(t, n, "", resources.DirectoryTravelerReserve, map[string]any{"email": id + "@x.com", "idViajero": id})
	}
	for _, id := range ids {
		if r := read(t, n, id, resources.TravelerFindByID, map[string]any{"idViajero": id}); len(r.Rows) != 1 {
			t.Fatalf("%s no encontrado", id)
		}
		if r := read(t, n, "", resources.DirectoryTravelerLookup, map[string]any{"email": id + "@x.com"}); len(r.Rows) != 1 || r.Rows[0]["id_viajero"] != id {
			t.Fatalf("directorio de %s: %+v", id, r)
		}
	}
}

// [ADR-21] SyncTo instala el estado de la primaria en el pasivo, que lo sirve tras promoverse.
func TestSyncToPassiveAndPromotion(t *testing.T) {
	active, passive := newNode(t, "a", RoleActive), newNode(t, "p", RolePassive)
	active.SetPeer(passive)
	write(t, active, "v1", resources.TravelerCreate, traveler("v1", "a@x.com"))
	write(t, active, "", resources.DirectoryTravelerReserve, map[string]any{"email": "a@x.com", "idViajero": "v1"})

	e5, err := active.SyncNow(context.Background())
	if err != nil || e5.Origen != "a" || e5.Destino != "p" || e5.VersionEstado < 2 {
		t.Fatalf("E5 inesperado: %+v %v", e5, err)
	}
	passive.Promote()
	if r := read(t, passive, "v1", resources.TravelerFindByID, map[string]any{"idViajero": "v1"}); len(r.Rows) != 1 {
		t.Fatalf("el pasivo debía leer los datos sincronizados: %+v", r)
	}
	if r := read(t, passive, "", resources.DirectoryTravelerLookup, map[string]any{"email": "a@x.com"}); len(r.Rows) != 1 {
		t.Fatal("el directorio también debía sincronizarse")
	}
	// y el pasivo promovido puede escribir con normalidad
	write(t, passive, "v2", resources.TravelerCreate, traveler("v2", "b@x.com"))
}

func TestStoppedNodeIsUnavailable(t *testing.T) {
	n := newNode(t, "n1", RoleActive)
	n.Stop()
	if _, err := n.Read(context.Background(), "v1", ports.DataQuery{Resource: resources.TravelerFindByID}); !errors.Is(err, ports.ErrDataNodeUnavailable) {
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

// [ADR-10] La réplica falla a propósito: no queda caché vieja, la lectura ve el dato nuevo desde la primaria
// y al recuperarse la réplica la pendiente se aplica.
func TestFailedReplicationLeavesNoStaleCacheAndIsRetried(t *testing.T) {
	var failing atomic.Bool
	n, err := NewDataNode(Config{ID: "n", Role: RoleActive, Dir: t.TempDir(), Shards: 2,
		ReplicationFault: func(operations.ReplicationRecord) error {
			if failing.Load() {
				return errors.New("réplica caída")
			}
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	write(t, n, "v1", resources.LinkedAccountSave, map[string]any{"idCuenta": "c1", "idViajero": "v1", "proveedor": "gmail", "encryptedAccessToken": "inicial", "encryptedRefreshToken": "r", "estado": "VINCULADA", "expiresAt": ""})
	q := map[string]any{"idCuenta": "c1", "idViajero": "v1"}
	if r := read(t, n, "v1", resources.LinkedAccountFind, q); r.Rows[0]["encrypted_access_token"] != "inicial" { // queda cacheado
		t.Fatalf("lectura inicial: %+v", r)
	}

	failing.Store(true)
	// Write responde éxito aunque la copia falle: la primaria ya confirmó (helper write exige err == nil).
	write(t, n, "v1", resources.LinkedAccountSave, map[string]any{"idCuenta": "c1", "idViajero": "v1", "proveedor": "gmail", "encryptedAccessToken": "nuevo", "encryptedRefreshToken": "r", "estado": "VINCULADA", "expiresAt": ""})
	if n.PendingReplications() != 1 {
		t.Fatalf("debía quedar 1 replicación pendiente, hay %d", n.PendingReplications())
	}
	if r := read(t, n, "v1", resources.LinkedAccountFind, q); r.Rows[0]["encrypted_access_token"] != "nuevo" {
		t.Fatalf("la lectura inmediata debía ver el dato nuevo (sin caché vieja): %+v", r)
	}

	n.FlushReplication(context.Background()) // la réplica sigue caída: no se aplica
	if n.PendingReplications() != 1 {
		t.Fatal("con la réplica caída la pendiente debía seguir en la lista")
	}
	failing.Store(false)
	n.FlushReplication(context.Background())
	if n.PendingReplications() != 0 {
		t.Fatalf("la lista debía quedar vacía, hay %d", n.PendingReplications())
	}
	res, err := n.replica.Read(context.Background(), mustPartition(t, n, "v1"), mustOp(t, n, resources.LinkedAccountFind), ports.DataQuery{Resource: resources.LinkedAccountFind, Params: q})
	if err != nil || res.Rows[0]["encrypted_access_token"] != "nuevo" {
		t.Fatalf("la réplica debía tener el dato: %+v %v", res, err)
	}
}

// [ADR-10] Dos escrituras con la réplica caída se aplican en orden al recuperarse.
func TestPendingReplicationsApplyInOrder(t *testing.T) {
	var failing atomic.Bool
	failing.Store(true)
	n, err := NewDataNode(Config{ID: "n", Role: RoleActive, Dir: t.TempDir(), Shards: 2,
		ReplicationFault: func(operations.ReplicationRecord) error {
			if failing.Load() {
				return errors.New("réplica caída")
			}
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	failing.Store(false)
	write(t, n, "v1", resources.LinkedAccountSave, map[string]any{"idCuenta": "c1", "idViajero": "v1", "proveedor": "gmail", "encryptedAccessToken": "inicial", "encryptedRefreshToken": "r", "estado": "VINCULADA", "expiresAt": ""})
	failing.Store(true)
	write(t, n, "v1", resources.LinkedAccountSave, map[string]any{"idCuenta": "c1", "idViajero": "v1", "proveedor": "gmail", "encryptedAccessToken": "uno", "encryptedRefreshToken": "r", "estado": "VINCULADA", "expiresAt": ""})
	write(t, n, "v1", resources.LinkedAccountSave, map[string]any{"idCuenta": "c1", "idViajero": "v1", "proveedor": "gmail", "encryptedAccessToken": "dos", "encryptedRefreshToken": "r", "estado": "VINCULADA", "expiresAt": ""})
	if n.PendingReplications() != 2 {
		t.Fatalf("pendientes: %d", n.PendingReplications())
	}
	failing.Store(false)
	n.FlushReplication(context.Background())
	res, _ := n.replica.Read(context.Background(), mustPartition(t, n, "v1"), mustOp(t, n, resources.LinkedAccountFind), ports.DataQuery{Params: map[string]any{"idCuenta": "c1", "idViajero": "v1"}})
	if n.PendingReplications() != 0 || res.Rows[0]["encrypted_access_token"] != "dos" {
		t.Fatalf("el último valor debía ser dos: %+v pendientes=%d", res, n.PendingReplications())
	}
}

func mustPartition(t *testing.T, n *DataNode, id string) string {
	t.Helper()
	p, err := n.router.PartitionFor(id, operations.ScopeShard)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustOp(t *testing.T, n *DataNode, res string) operations.Operation {
	t.Helper()
	op, err := n.catalog.Resolve(res, operations.Read)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

// [ADR-21] Recover limpia down y Run vuelve a emitir heartbeat.
func TestRecoverRestartsHeartbeat(t *testing.T) {
	hb := stubs.NewInMemoryHeartbeatChannel(10*time.Millisecond, 3)
	n, err := NewDataNode(Config{ID: "n", Role: RolePassive, Dir: t.TempDir(), Shards: 1, Heartbeat: hb, HeartbeatInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.Run(ctx)
	waitFor(t, func() bool { _, ok := hb.Last("n"); return ok })
	n.Stop()
	waitFor(t, func() bool { return hb.Observe("n") == ports.NodeDead })
	n.Recover()
	waitFor(t, func() bool { return hb.Observe("n") == ports.NodeAlive })
	if _, err := n.Read(context.Background(), "v1", ports.DataQuery{Resource: resources.TravelerFindByID, Params: map[string]any{"idViajero": "v1"}}); err != nil {
		t.Fatalf("tras Recover el nodo debía atender: %v", err)
	}
}
