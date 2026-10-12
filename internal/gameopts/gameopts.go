// Package gameopts aplica los ajustes de rendimiento del jugador al fichero de
// arranque del juego (ProjectZomboid64.json), editando sus vmArgs.
//
// El launcher arranca el juego vía Steam (steam://rungameid/…), así que no puede
// pasarle argumentos a la JVM en la línea de comandos: la única vía fiable es
// editar el ProjectZomboid64.json que el juego lee al arrancar. Como lo
// reescribimos en cada lanzamiento, si Steam lo pisa (update / "Verificar
// integridad") el launcher lo vuelve a aplicar solo.
//
// Solo tocamos NUESTRAS líneas gestionadas (-Dpz.chunkBudgetMs, -Dpz.safeTowReconnect
// y, si el jugador lo configuró, -Xmx); el resto de vmArgs y del fichero se preserva intacto.
package gameopts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pzlauncher/internal/sysmem"
)

// jsonName es el fichero de arranque del cliente de 64 bits.
const jsonName = "ProjectZomboid64.json"

// chunkArgPrefix es el flag gestionado del fix anti-tirones (parche C11).
const chunkArgPrefix = "-Dpz.chunkBudgetMs="

// safeTowArgPrefix es el flag gestionado del fix de reenganche de remolque sin
// tirón (parche C18). El jar lo trae ACTIVADO por defecto; solo escribimos
// "-Dpz.safeTowReconnect=false" cuando el jugador decide desactivarlo.
const safeTowArgPrefix = "-Dpz.safeTowReconnect="

// maxGridArgPrefix es el flag gestionado del cap de ancho de grilla de chunks
// (parche C26). Baja la RAM que el server carga por este jugador. El jar usa su
// default (0 = vanilla) cuando el flag no está; solo lo escribimos si el jugador
// eligió un tope (>= 12).
const maxGridArgPrefix = "-Dpz.maxChunkGridWidth="

// Path devuelve la ruta al ProjectZomboid64.json dentro de la carpeta del juego.
func Path(gameDir string) string { return filepath.Join(gameDir, jsonName) }

// Apply reescribe los vmArgs del ProjectZomboid64.json según los ajustes:
//   - chunkBudgetMs > 0  -> fija "-Dpz.chunkBudgetMs=N". 0 -> quita el flag
//     (el juego usa el default del jar, que es 0 = vanilla).
//   - maxHeapMB > 0      -> fija "-Xmx<N>m" (reemplaza el que haya). 0 -> deja
//     el -Xmx del juego como esté (no lo gestionamos).
//   - safeTowOff == true -> fija "-Dpz.safeTowReconnect=false". false -> quita
//     el flag (el jar usa su default, que es true = fix activado).
//   - maxChunkGridWidth >= 12 -> fija "-Dpz.maxChunkGridWidth=N". 0 (o < 12) ->
//     quita el flag (el jar usa su default, 0 = vanilla por resolución).
//
// Preserva el resto del fichero (mainClass, classpath y demás vmArgs).
func Apply(gameDir string, chunkBudgetMs, maxHeapMB, maxChunkGridWidth int, safeTowOff bool) error {
	p := Path(gameDir)
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("leyendo %s: %w", jsonName, err)
	}

	// map genérico para no perder claves ni campos desconocidos del fichero.
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("%s con formato inesperado: %w", jsonName, err)
	}

	var args []string
	if raw, ok := root["vmArgs"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				args = append(args, s)
			}
		}
	}

	args = setVMArgs(args, chunkBudgetMs, maxHeapMB, maxChunkGridWidth, safeTowOff)

	// Volcar de vuelta como []any para el marshaling.
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = a
	}
	root["vmArgs"] = out

	buf, err := json.MarshalIndent(root, "", "\t")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, append(buf, '\n'), 0o644); err != nil {
		return fmt.Errorf("escribiendo %s: %w", jsonName, err)
	}
	return nil
}

// setVMArgs devuelve una copia de args con nuestras líneas gestionadas puestas al día.
func setVMArgs(args []string, chunkBudgetMs, maxHeapMB, maxChunkGridWidth int, safeTowOff bool) []string {
	out := make([]string, 0, len(args)+4)
	for _, a := range args {
		t := strings.TrimSpace(a)
		if strings.HasPrefix(t, chunkArgPrefix) {
			continue // quitamos el anterior; se re-añade abajo si corresponde
		}
		if strings.HasPrefix(t, safeTowArgPrefix) {
			continue // idem: lo regeneramos abajo solo si el jugador lo desactivó
		}
		if strings.HasPrefix(t, maxGridArgPrefix) {
			continue // idem: se re-añade abajo solo si el jugador fijó un tope
		}
		if maxHeapMB > 0 && strings.HasPrefix(t, "-Xmx") {
			continue // reemplazamos el -Xmx solo si el jugador lo configuró
		}
		out = append(out, a)
	}
	if maxHeapMB > 0 {
		out = append(out, fmt.Sprintf("-Xmx%dm", maxHeapMB))
	}
	if chunkBudgetMs > 0 {
		out = append(out, fmt.Sprintf("%s%d", chunkArgPrefix, chunkBudgetMs))
	}
	if maxChunkGridWidth >= 12 {
		out = append(out, fmt.Sprintf("%s%d", maxGridArgPrefix, maxChunkGridWidth))
	}
	if safeTowOff {
		out = append(out, safeTowArgPrefix+"false")
	}
	return out
}

// SuggestHeapMB sugiere un -Xmx (en MB) según la RAM física total, dejando
// margen para el SO. Devuelve 0 si no se pudo detectar la RAM. El cliente de PZ
// rara vez aprovecha más de 8 GB, así que ahí topamos.
func SuggestHeapMB() int {
	total := sysmem.TotalBytes()
	if total == 0 {
		return 0
	}
	gib := float64(total) / (1024 * 1024 * 1024)
	switch {
	case gib < 7.5: // equipos de ~8 GB: no ahogar al SO
		return 3072
	case gib < 15: // ~8-16 GB
		return 4096
	case gib < 31: // ~16-32 GB
		return 6144
	default: // 32 GB o más
		return 8192
	}
}

// TotalRAMGB devuelve la RAM física total redondeada a GB (0 si no se detectó).
func TotalRAMGB() int {
	total := sysmem.TotalBytes()
	if total == 0 {
		return 0
	}
	return int((float64(total)/(1024*1024*1024) + 0.5))
}
