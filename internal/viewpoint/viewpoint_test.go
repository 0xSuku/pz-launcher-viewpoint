package viewpoint

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeJar(t *testing.T, path, version string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("META-INF/MANIFEST.MF")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("Manifest-Version: 1.0\r\nImplementation-Version: " + version + "\r\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

// fakeItem arma <item>/mods/ZombieBuddy/libs con un dll y un jar de la versión dada.
func fakeItem(t *testing.T, item, version string) string {
	t.Helper()
	libs := filepath.Join(item, "mods", "ZombieBuddy", "libs")
	if err := os.MkdirAll(libs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libs, dll), []byte("dll "+version), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJar(t, filepath.Join(libs, jar), version)
	return libs
}

func TestJarVersion(t *testing.T) {
	dir := t.TempDir()
	writeJar(t, filepath.Join(dir, jar), "2.3.4")
	if v := JarVersion(filepath.Join(dir, jar)); v != "2.3.4" {
		t.Fatalf("versión: %q", v)
	}
	if v := JarVersion(filepath.Join(dir, "no-existe.jar")); v != "" {
		t.Fatalf("un jar inexistente no tiene versión: %q", v)
	}
}

func TestInstallThenUpdateFollowsWorkshop(t *testing.T) {
	game, item := t.TempDir(), t.TempDir()
	libs := fakeItem(t, item, "1.0.0")

	if s := StateOf(game, ""); s != NoSource {
		t.Fatalf("sin nada: %s", s)
	}
	if s := StateOf(game, libs); s != NotInstalled {
		t.Fatalf("con Workshop pero sin copiar: %s", s)
	}
	if err := Install(game, libs); err != nil {
		t.Fatal(err)
	}
	if s := StateOf(game, libs); s != UpToDate {
		t.Fatalf("tras instalar: %s", s)
	}
	if s := StateOf(game, ""); s != InstalledNoSource {
		t.Fatalf("instalado sin Workshop: %s", s)
	}

	// Steam actualiza el ítem de Workshop.
	fakeItem(t, item, "1.1.0")
	if s := StateOf(game, libs); s != UpdateAvailable {
		t.Fatalf("tras update de Workshop: %s", s)
	}

	// Un .new viejo del autoactualizador desharía la copia: Install lo borra.
	os.WriteFile(filepath.Join(game, pendingJar), []byte("viejo"), 0o644)
	if err := Install(game, libs); err != nil {
		t.Fatal(err)
	}
	if s := StateOf(game, libs); s != UpToDate {
		t.Fatalf("tras actualizar: %s", s)
	}
	if v := JarVersion(filepath.Join(game, jar)); v != "1.1.0" {
		t.Fatalf("versión instalada: %q", v)
	}
	if _, err := os.Stat(filepath.Join(game, pendingJar)); err == nil {
		t.Fatal("quedó un ZombieBuddy.jar.new viejo")
	}
}

func TestLibsNeedBothFiles(t *testing.T) {
	item := t.TempDir()
	libs := fakeItem(t, item, "1.0.0")
	if _, ok := libsIn(item); !ok {
		t.Fatal("con los dos ficheros debía encontrarlo")
	}
	os.Remove(filepath.Join(libs, dll))
	if _, ok := libsIn(item); ok {
		t.Fatal("sin el dll no está completo")
	}
}

func TestLaunchOptionsLoadAgent(t *testing.T) {
	withAgent := `"UserLocalConfigStore"
{
	"Software" { "Valve" { "Steam" { "apps" {
		"108600"
		{
			"LaunchOptions"		"-agentlib:zbNative --"
		}
	} } } }
}`
	plain := `"108600" { "LaunchOptions"		"-Xmx6144m -Xms6144m --" }`
	if !launchOptionsLoadAgent(withAgent) {
		t.Fatal("no detectó el agente en las opciones de lanzamiento")
	}
	if launchOptionsLoadAgent(plain) {
		t.Fatal("opciones sin agente detectadas como con agente")
	}
}
