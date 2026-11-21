// Package src contiene el test de arquitectura del prototipo: verifica por análisis estático de imports
// que se respeta la regla de dependencias (concern de tecnologías genéricas, también de forma transitiva),
// las fronteras entre contenedores y entre módulos del monolito (ADR-02) y el mapeo 1 a 1 con AD §3/§4.
package src

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/nicolas-wai/trotamundos/src"

// checkCoreImports verifica los imports de los paquetes "domain" y "application" bajo root.
// Regla: solo biblioteca estándar (sin net/http), src/ports, src/contracts/... y el domain del propio módulo.
// Devuelve una lista de violaciones.
func checkCoreImports(root string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		dir := filepath.Dir(path)
		layer := filepath.Base(dir)
		if layer != "domain" && layer != "application" {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(dir))
		if err != nil {
			return err
		}
		moduleDir := filepath.ToSlash(rel) // p. ej. replica/authentication
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !allowedImport(p, moduleDir) {
				violations = append(violations, fmt.Sprintf("%s importa %q", filepath.ToSlash(path), p))
			}
		}
		return nil
	})
	return violations, err
}

// isForbiddenStdlib: paquetes de la biblioteca estándar que atan al núcleo a una tecnología concreta (HTTP, SQL).
func isForbiddenStdlib(p string) bool {
	return p == "net/http" || strings.HasPrefix(p, "net/http/") || p == "database/sql" || strings.HasPrefix(p, "database/sql/")
}

func allowedImport(p, moduleDir string) bool {
	first, _, _ := strings.Cut(p, "/")
	if !strings.Contains(first, ".") { // biblioteca estándar
		return !isForbiddenStdlib(p)
	}
	switch {
	case p == modulePath+"/ports":
		return true
	case strings.HasPrefix(p, modulePath+"/contracts/"):
		return true
	case p == modulePath+"/"+moduleDir+"/domain":
		return true
	}
	return false
}

func TestCoreDoesNotDependOnInfrastructure(t *testing.T) {
	violations, err := checkCoreImports(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("violaciones de la regla de dependencias:\n%s", strings.Join(violations, "\n"))
	}
}

// El checker debe detectar imports prohibidos: guarda contra un test que siempre pasa.
func TestCheckerDetectsForbiddenImports(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "replica", "demo", "application")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package application

import (
	"net/http"
	"modernc.org/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"` + modulePath + `/replica/other/domain"
	"` + modulePath + `/replica/demo/adapters"
	"fmt"
	"` + modulePath + `/ports"
	"` + modulePath + `/replica/demo/domain"
)
`
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	violations, err := checkCoreImports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 6 {
		t.Fatalf("se esperaban 6 violaciones, hubo %d: %v", len(violations), violations)
	}
}

// Mapeo 1 a 1 con los contenedores y componentes de AD §3 y §4 (ImplementationPlan §0.3).
func TestDirectoryLayoutMatchesArchitecture(t *testing.T) {
	required := []string{
		"config", "contracts/httpdto", "contracts/apierror", "contracts/events", "contracts/resources", "httpresponse", "ports",
		"client/web/templates", "client/web/static",
		"loadbalancer", "messagequeue", "priorityqueue",
		"datanode/cache", "datanode/primary", "datanode/readreplica", "datanode/schema", "datanode/operations",
		"stubs",
		"replica/api", "replica/tokenvalidator", "replica/ssr", "replica/updateconsumer",
		"replica/priorityqueueclient", "replica/heartbeat",
		"replica/authentication/domain", "replica/authentication/application", "replica/authentication/adapters",
		"replica/permissions/domain", "replica/permissions/application", "replica/permissions/adapters",
		"replica/travelarrangements/domain", "replica/travelarrangements/application", "replica/travelarrangements/adapters",
		"replica/integrations/domain", "replica/integrations/scheduler", "replica/integrations/circuitbreaker",
		"replica/integrations/providerport", "replica/integrations/broker", "replica/integrations/oauthmanager",
		"replica/integrations/adapters/email", "replica/integrations/adapters/agency", "replica/integrations/adapters/travelsystem",
		"replica/recommendations/domain", "replica/recommendations/collector",
		"replica/recommendations/engine", "replica/recommendations/hitl",
	}
	var missing []string
	for _, dir := range required {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			missing = append(missing, dir)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("faltan directorios de la arquitectura: %v", missing)
	}
}

// Arreglos de viaje e Integraciones externas solo se comunican por la cola P2P y el bus de eventos (ADR-04, ADR-07):
// ningún archivo de travelarrangements puede importar integrations.
func TestTravelArrangementsDoesNotImportIntegrations(t *testing.T) {
	var violations []string
	err := filepath.WalkDir("replica/travelarrangements", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); strings.HasPrefix(p, modulePath+"/replica/integrations") {
				violations = append(violations, fmt.Sprintf("%s importa %q", filepath.ToSlash(path), p))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("travelarrangements no puede depender de integrations:\n%s", strings.Join(violations, "\n"))
	}
}

// Los puertos de salida de proveedores no deben conocer a sus adapters concretos ni a los stubs.
func TestPortsDoNotImportStubsOrAdapters(t *testing.T) {
	entries, _ := os.ReadDir("ports")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("ports", e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(p, modulePath+"/stubs") || strings.Contains(p, "/adapters") {
				t.Errorf("ports/%s importa %q", e.Name(), p)
			}
		}
	}
}

// importsOfDir devuelve los imports (sin duplicados) de los .go de un directorio, sin contar los _test.go.
func importsOfDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// checkTransitiveCore recorre, desde cada paquete domain/application bajo root, todo el grafo de imports del
// propio módulo (mod) y reporta si alguna ruta llega a una tecnología concreta: HTTP, SQL o una librería de
// terceros. Cierra el hueco de checkCoreImports, que solo ve imports directos. [Concern: tecnologías genéricas]
func checkTransitiveCore(root, mod string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if layer := filepath.Base(path); layer != "domain" && layer != "application" {
			return nil
		}
		start := mod + "/" + filepath.ToSlash(mustRel(root, path))
		visited := map[string]bool{start: true}
		type step struct{ pkg, chain string }
		queue := []step{{start, start}}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			dir := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(cur.pkg, mod+"/")))
			imports, err := importsOfDir(dir)
			if err != nil {
				return err
			}
			for _, p := range imports {
				first, _, _ := strings.Cut(p, "/")
				switch {
				case strings.HasPrefix(p, mod+"/"):
					if !visited[p] {
						visited[p] = true
						queue = append(queue, step{p, cur.chain + " -> " + p})
					}
				case !strings.Contains(first, ".") && !isForbiddenStdlib(p):
					// stdlib permitida
				default:
					violations = append(violations, cur.chain+" -> "+p)
				}
			}
		}
		return nil
	})
	sort.Strings(violations)
	return violations, err
}

func mustRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		panic(err)
	}
	return rel
}

// El núcleo (domain/application) no debe alcanzar HTTP, SQL ni librerías externas ni de forma indirecta
// (p. ej. importando un contrato que a su vez importe net/http).
func TestCoreHasNoTransitiveTechnologyDependencies(t *testing.T) {
	violations, err := checkTransitiveCore(".", modulePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("dependencias tecnológicas transitivas en el núcleo:\n%s", strings.Join(violations, "\n"))
	}
}

// Guarda contra un test transitivo que siempre pasa: el checker debe ver net/http detrás de un contrato.
func TestTransitiveCheckerDetectsIndirectHTTP(t *testing.T) {
	const mod = "example.com/m"
	root := t.TempDir()
	write := func(rel, src string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("replica/demo/application/a.go", "package application\nimport _ \""+mod+"/contracts/errs\"\n")
	write("contracts/errs/errs.go", "package errs\nimport _ \"net/http\"\n")
	write("replica/ok/domain/d.go", "package domain\nimport _ \"fmt\"\n")
	violations, err := checkTransitiveCore(root, mod)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || !strings.HasSuffix(violations[0], "-> net/http") {
		t.Fatalf("se esperaba 1 violación indirecta por net/http, hubo: %v", violations)
	}
}

// nonTestGoFiles lista los .go de producción bajo dir (relativo a src).
func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func importsOfFile(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		out = append(out, p)
	}
	return out
}

// ADR-02: los módulos del monolito solo se conocen por el domain del otro (o por puertos en src/ports).
// Excepciones explícitas, tomadas de las flechas de AD §4.4:
//   - Control de permisos → Arreglos de viaje (PERM --> VIA): permissions/adapters consulta el repositorio de arreglos.
func TestModulesDoNotImportEachOthersInternals(t *testing.T) {
	modules := []string{"authentication", "permissions", "travelarrangements", "integrations", "recommendations"}
	exceptions := map[string]string{
		"replica/permissions/adapters": modulePath + "/replica/travelarrangements/application",
	}
	var violations []string
	for _, mod := range modules {
		for _, path := range nonTestGoFiles(t, "replica/"+mod) {
			pkgDir := filepath.ToSlash(filepath.Dir(path))
			for _, p := range importsOfFile(t, path) {
				for _, other := range modules {
					if other == mod {
						continue
					}
					prefix := modulePath + "/replica/" + other
					if p != prefix && !strings.HasPrefix(p, prefix+"/") {
						continue
					}
					if p == prefix+"/domain" || exceptions[pkgDir] == p {
						continue
					}
					violations = append(violations, fmt.Sprintf("%s importa %q", filepath.ToSlash(path), p))
				}
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("módulos acoplados a internals de otro módulo (usar su domain o un puerto en src/ports):\n%s", strings.Join(violations, "\n"))
	}
}

// Los puertos compartidos solo pueden referirse al domain de los módulos, nunca a su application ni a sus adapters.
func TestPortsOnlyImportModuleDomains(t *testing.T) {
	for _, path := range nonTestGoFiles(t, "ports") {
		for _, p := range importsOfFile(t, path) {
			if strings.HasPrefix(p, modulePath+"/replica/") && !strings.HasSuffix(p, "/domain") {
				t.Errorf("%s importa %q: los puertos solo pueden usar domain", filepath.ToSlash(path), p)
			}
		}
	}
}

// Los mocks y stubs de borde solo los ensambla la raíz de composición (región y main); ningún contenedor de
// producción (réplica, LB, colas, datos) puede importarlos.
func TestStubsOnlyImportedByCompositionRoot(t *testing.T) {
	var violations []string
	for _, path := range nonTestGoFiles(t, ".") {
		rel := filepath.ToSlash(path)
		if strings.HasPrefix(rel, "region/") || strings.HasPrefix(rel, "stubs/") {
			continue
		}
		for _, p := range importsOfFile(t, path) {
			if p == modulePath+"/stubs" {
				violations = append(violations, fmt.Sprintf("%s importa %q", rel, p))
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("solo region puede ensamblar stubs:\n%s", strings.Join(violations, "\n"))
	}
}

// Fronteras entre contenedores (AD §3): se comunican por src/ports y src/contracts, nunca importándose.
// Solo region (raíz de composición) y cmd conocen a todos.
func TestContainersDoNotImportEachOther(t *testing.T) {
	containers := []string{"replica", "datanode", "priorityqueue", "loadbalancer", "messagequeue"}
	var violations []string
	for _, c := range containers {
		for _, path := range nonTestGoFiles(t, c) {
			for _, p := range importsOfFile(t, path) {
				for _, other := range append([]string{"region"}, containers...) {
					if other == c {
						continue
					}
					prefix := modulePath + "/" + other
					if p == prefix || strings.HasPrefix(p, prefix+"/") {
						violations = append(violations, fmt.Sprintf("%s importa %q", filepath.ToSlash(path), p))
					}
				}
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("contenedores acoplados entre sí (usar src/ports o src/contracts):\n%s", strings.Join(violations, "\n"))
	}
}
