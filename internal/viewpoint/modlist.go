package viewpoint

import (
	"os"
	"path/filepath"
	"strings"
)

// ModIDs son los mods que tienen que estar activos en el menú principal, en
// este orden (Viewpoint declara require=\ZombieBuddy).
//
// Por qué alcanza con el menú principal aunque el servidor no los tenga en su
// lista: ZombieBuddy carga el jar de un mod Java cuando ese mod está activo y
// aplica sus parches en memoria; al conectarse a un servidor el juego recarga
// la lista de mods, pero ZombieBuddy no deshace lo ya aplicado ("Restart the
// game to deactivate it", Loader.java). La parte de Viewpoint que importa (la
// cámara) es toda Java; sus dos Lua (opciones de FPS) sí dependen de la lista
// del servidor.
var ModIDs = []string{"ZombieBuddy", "Viewpoint"}

// DefaultModsFile es la lista de mods activos del menú principal.
func DefaultModsFile(zomboidDir string) string {
	return filepath.Join(zomboidDir, "mods", "default.txt")
}

const emptyDefaultMods = "VERSION = 1,\n\nmods\n{\n}\n\nmaps\n{\n}\n"

// ActiveMods devuelve los ids del bloque "mods { mod = X, }" de default.txt.
func ActiveMods(text string) []string {
	var out []string
	inMods := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "mods":
			inMods = true
		case inMods && t == "}":
			inMods = false
		case inMods && strings.HasPrefix(t, "mod"):
			if id, ok := modLineID(t); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

// modLineID extrae X de "mod = X," (con o sin "\" delante, como en B42).
func modLineID(t string) (string, bool) {
	k, v, ok := strings.Cut(t, "=")
	if !ok || strings.TrimSpace(k) != "mod" {
		return "", false
	}
	v = strings.TrimSuffix(strings.TrimSpace(v), ",")
	return strings.TrimPrefix(strings.TrimSpace(v), `\`), true
}

// AddMods inserta al principio del bloque "mods" los ids que falten y devuelve
// el texto nuevo y cuáles agregó. Respeta el resto del fichero tal cual.
func AddMods(text string, ids []string) (string, []string) {
	if strings.TrimSpace(text) == "" {
		text = emptyDefaultMods
	}
	have := map[string]bool{}
	for _, id := range ActiveMods(text) {
		have[id] = true
	}
	var missing []string
	for _, id := range ids {
		if !have[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return text, nil
	}
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(text, nl)
	for i := 0; i+1 < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "mods" && strings.TrimSpace(lines[i+1]) == "{" {
			var add []string
			for _, id := range missing {
				add = append(add, "    mod = "+id+",")
			}
			lines = append(lines[:i+2], append(add, lines[i+2:]...)...)
			return strings.Join(lines, nl), missing
		}
	}
	// Sin bloque "mods" reconocible: lo agregamos al final.
	block := nl + "mods" + nl + "{" + nl
	for _, id := range missing {
		block += "    mod = " + id + "," + nl
	}
	return text + block + "}" + nl, missing
}

// RemoveMods quita del bloque "mods" las líneas de esos ids.
func RemoveMods(text string, ids []string) string {
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	var out []string
	inMods := false
	for _, line := range strings.Split(text, nl) {
		t := strings.TrimSpace(line)
		switch {
		case t == "mods":
			inMods = true
		case inMods && t == "}":
			inMods = false
		case inMods:
			if id, ok := modLineID(t); ok && drop[id] {
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, nl)
}

// EnableMods activa ZombieBuddy y Viewpoint en default.txt y devuelve los ids
// que agregó (para poder quitar solo esos al desactivar). Hay que llamarlo con
// el juego cerrado: el juego reescribe default.txt al salir.
func EnableMods(zomboidDir string) ([]string, error) {
	p := DefaultModsFile(zomboidDir)
	data, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	text, added := AddMods(string(data), ModIDs)
	if len(added) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return added, os.WriteFile(p, []byte(text), 0o644)
}

// DisableMods quita de default.txt los ids indicados (los que agregó el launcher).
func DisableMods(zomboidDir string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	p := DefaultModsFile(zomboidDir)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(p, []byte(RemoveMods(string(data), ids)), 0o644)
}
