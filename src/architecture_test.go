// Package src contiene el test de arquitectura del prototipo: verifica por análisis estático de imports
// que se respeta la regla de dependencias (concern de tecnologías genéricas) y el mapeo 1 a 1 con AD §3/§4.
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

func allowedImport(p, moduleDir string) bool {
	first, _, _ := strings.Cut(p, "/")
	if !strings.Contains(first, ".") { // biblioteca estándar
		return p != "net/http" && !strings.HasPrefix(p, "net/http/")
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
		"config", "contracts/httpdto", "contracts/apierror", "contracts/events", "ports",
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
