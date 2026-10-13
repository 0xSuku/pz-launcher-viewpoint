// Package steam localiza la carpeta de INSTALACIÓN de un juego de Steam
// (donde está projectzomboid.jar), no la de datos del usuario (logs/partidas).
//
// Steam no expone una variable de entorno con esa ruta, pero se puede deducir:
//  1. La carpeta de Steam (según el SO: registro en Windows, rutas conocidas en
//     Linux/Mac).
//  2. Las bibliotecas (posiblemente en varios discos) desde libraryfolders.vdf.
//  3. En cada biblioteca, appmanifest_<appid>.acf da el "installdir"; la carpeta
//     final es <biblioteca>/steamapps/common/<installdir>.
package steam

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reInstallDir = regexp.MustCompile(`(?i)"installdir"\s+"([^"]+)"`)
	rePath       = regexp.MustCompile(`(?i)"path"\s+"([^"]+)"`)
)

// FindGameDir devuelve la carpeta de instalación del juego con el AppID dado
// (Project Zomboid = 108600) y true si la encontró.
func FindGameDir(appID int) (string, bool) {
	seen := map[string]bool{}
	for _, root := range steamRoots() {
		for _, lib := range libraryPaths(root) {
			if seen[lib] {
				continue
			}
			seen[lib] = true
			if dir, ok := gameInLibrary(lib, appID); ok {
				return dir, true
			}
		}
	}
	return "", false
}

// WorkshopItemDir devuelve la carpeta de un ítem de Workshop ya descargado
// (<biblioteca>/steamapps/workshop/content/<appid>/<itemID>) y true si existe.
// Steam lo baja a la biblioteca donde está el juego, que puede ser otro disco.
func WorkshopItemDir(appID int, itemID string) (string, bool) {
	if itemID == "" {
		return "", false
	}
	seen := map[string]bool{}
	for _, root := range steamRoots() {
		for _, lib := range libraryPaths(root) {
			if seen[lib] {
				continue
			}
			seen[lib] = true
			dir := filepath.Join(lib, "steamapps", "workshop", "content", strconv.Itoa(appID), itemID)
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir, true
			}
		}
	}
	return "", false
}

// LocalConfigFiles devuelve los localconfig.vdf de cada cuenta de Steam que
// usó esta PC (<steam>/userdata/<id>/config/localconfig.vdf). Ahí guarda Steam
// las "Opciones de lanzamiento" de cada juego. Solo para leer.
func LocalConfigFiles() []string {
	var out []string
	seen := map[string]bool{}
	for _, root := range steamRoots() {
		matches, _ := filepath.Glob(filepath.Join(root, "userdata", "*", "config", "localconfig.vdf"))
		for _, m := range matches {
			// El registro suele dar la misma carpeta de Steam por varias vías.
			if key := strings.ToLower(filepath.Clean(m)); !seen[key] {
				seen[key] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// gameInLibrary mira si una biblioteca contiene el juego y devuelve su carpeta.
func gameInLibrary(lib string, appID int) (string, bool) {
	acf := filepath.Join(lib, "steamapps", "appmanifest_"+strconv.Itoa(appID)+".acf")
	data, err := os.ReadFile(acf)
	if err != nil {
		return "", false
	}
	m := reInstallDir.FindSubmatch(data)
	if m == nil {
		return "", false
	}
	dir := filepath.Join(lib, "steamapps", "common", string(m[1]))
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, true
	}
	return "", false
}

// libraryPaths lee las bibliotecas declaradas (la propia carpeta de Steam más
// las de libraryfolders.vdf, que pueden estar en otros discos).
func libraryPaths(steamRoot string) []string {
	libs := []string{steamRoot}
	candidates := []string{
		filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"),
		filepath.Join(steamRoot, "config", "libraryfolders.vdf"),
	}
	for _, vdf := range candidates {
		data, err := os.ReadFile(vdf)
		if err != nil {
			continue
		}
		for _, m := range rePath.FindAllSubmatch(data, -1) {
			// En el VDF las barras invertidas van escapadas ("\\").
			libs = append(libs, strings.ReplaceAll(string(m[1]), `\\`, `\`))
		}
	}
	return libs
}
