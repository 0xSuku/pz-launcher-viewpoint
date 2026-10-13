package gameopts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func countPrefix(args []string, prefix string) int {
	n := 0
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			n++
		}
	}
	return n
}

func TestSetVMArgs(t *testing.T) {
	base := []string{"-Djava.awt.headless=false", "-Xmx3072m", "-Dpz.chunkBudgetMs=99"}

	// chunk>0 reemplaza el flag previo (sin duplicar); heap=0 deja -Xmx intacto;
	// safeTowOff=false NO escribe el flag (el jar usa su default = true).
	got := setVMArgs(base, 4, 0, 0, false)
	if countPrefix(got, chunkArgPrefix) != 1 || !contains(got, "-Dpz.chunkBudgetMs=4") {
		t.Fatalf("chunk no quedó en 4 único: %v", got)
	}
	if !contains(got, "-Xmx3072m") {
		t.Fatalf("heap=0 no debía tocar -Xmx: %v", got)
	}
	if !contains(got, "-Djava.awt.headless=false") {
		t.Fatalf("se perdió un arg no gestionado: %v", got)
	}
	if countPrefix(got, safeTowArgPrefix) != 0 {
		t.Fatalf("safeTowOff=false no debía escribir el flag: %v", got)
	}

	// chunk=0 quita el flag (vuelve a vanilla); heap>0 reemplaza -Xmx;
	// safeTowOff=true escribe "-Dpz.safeTowReconnect=false" una sola vez.
	got = setVMArgs(base, 0, 6144, 0, true)
	if countPrefix(got, chunkArgPrefix) != 0 {
		t.Fatalf("chunk=0 debía quitar el flag: %v", got)
	}
	if countPrefix(got, "-Xmx") != 1 || !contains(got, "-Xmx6144m") {
		t.Fatalf("heap no quedó en 6144 único: %v", got)
	}
	if countPrefix(got, safeTowArgPrefix) != 1 || !contains(got, "-Dpz.safeTowReconnect=false") {
		t.Fatalf("safeTowOff=true no quedó en false único: %v", got)
	}

	// safeTowOff vuelve a false -> se elimina el flag previo (no se arrastra).
	got = setVMArgs(got, 0, 0, 0, false)
	if countPrefix(got, safeTowArgPrefix) != 0 {
		t.Fatalf("safeTowOff=false debía quitar el flag previo: %v", got)
	}

	// maxChunkGridWidth>=12 escribe el flag una sola vez (parche C26).
	got = setVMArgs(base, 0, 0, 16, false)
	if countPrefix(got, maxGridArgPrefix) != 1 || !contains(got, "-Dpz.maxChunkGridWidth=16") {
		t.Fatalf("grid no quedó en 16 único: %v", got)
	}
	// < 12 (incluye 0) NO escribe el flag y quita el previo.
	got = setVMArgs(got, 0, 0, 0, false)
	if countPrefix(got, maxGridArgPrefix) != 0 {
		t.Fatalf("grid<12 debía quitar el flag previo: %v", got)
	}
}

func TestApplyPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	src := `{
	"mainClass": "zombie/gameStates/MainScreenState",
	"classpath": ["java/.", "java/projectzomboid.jar"],
	"vmArgs": ["-Dzomboid.steam=1", "-Xmx3072m"]
}`
	if err := os.WriteFile(filepath.Join(dir, jsonName), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Apply(dir, 5, 4096, 0, true); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, jsonName))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if root["mainClass"] != "zombie/gameStates/MainScreenState" {
		t.Fatalf("se perdió mainClass: %v", root["mainClass"])
	}
	if _, ok := root["classpath"]; !ok {
		t.Fatalf("se perdió classpath")
	}
	var args []string
	for _, v := range root["vmArgs"].([]any) {
		args = append(args, v.(string))
	}
	if !contains(args, "-Dpz.chunkBudgetMs=5") || !contains(args, "-Xmx4096m") {
		t.Fatalf("no se aplicaron los flags: %v", args)
	}
	if !contains(args, "-Dpz.safeTowReconnect=false") {
		t.Fatalf("no se aplicó el flag de remolque: %v", args)
	}
	if !contains(args, "-Dzomboid.steam=1") {
		t.Fatalf("se perdió un vmArg previo: %v", args)
	}
}

func TestSetAgentArg(t *testing.T) {
	base := []string{"-Djava.awt.headless=true", "-Xmx6144m"}

	// on: una sola línea, al principio; el resto intacto y en orden.
	got := setAgentArg(base, true)
	if got[0] != AgentArg || countPrefix(got, AgentArg) != 1 || len(got) != 3 {
		t.Fatalf("el agente no quedó único al principio: %v", got)
	}
	// Idempotente, y absorbe una variante con parámetros que estuviera más abajo.
	got = setAgentArg(append(got, AgentArg+"=verbosity=1"), true)
	if got[0] != AgentArg || countPrefix(got, AgentArg) != 1 {
		t.Fatalf("se duplicó el agente: %v", got)
	}
	// off: lo quita y no toca nada más.
	got = setAgentArg(got, false)
	if countPrefix(got, AgentArg) != 0 || !contains(got, "-Xmx6144m") || len(got) != 2 {
		t.Fatalf("off no quitó solo el agente: %v", got)
	}
}

func TestSetAgentSurvivesApply(t *testing.T) {
	dir := t.TempDir()
	src := `{ "mainClass": "zombie/gameStates/MainScreenState", "vmArgs": ["-Dzomboid.steam=1"] }`
	if err := os.WriteFile(filepath.Join(dir, jsonName), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasAgent(dir) {
		t.Fatal("un json vanilla no carga el agente")
	}
	if err := SetAgent(dir, true); err != nil {
		t.Fatal(err)
	}
	// Los ajustes de rendimiento se reaplican en cada Jugar: no deben borrar el agente.
	if err := Apply(dir, 4, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if !HasAgent(dir) {
		t.Fatal("Apply se llevó el agente")
	}
	if err := SetAgent(dir, false); err != nil {
		t.Fatal(err)
	}
	if HasAgent(dir) {
		t.Fatal("SetAgent(false) no quitó el agente")
	}
}
