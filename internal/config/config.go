// Package config lee/escribe un fichero de configuración opcional situado junto
// al ejecutable (pzlauncher.json). Permite al usuario personalizar cosas como la
// ruta de la carpeta de datos de Zomboid, que normalmente se autodetecta.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config son los ajustes persistentes del launcher.
type Config struct {
	// GameDir sobrescribe la carpeta de INSTALACIÓN del juego autodetectada
	// (…\steamapps\common\ProjectZomboid). Vacío = autodetectar vía Steam.
	GameDir string `json:"game_dir,omitempty"`

	// ZomboidDir sobrescribe la carpeta de datos de Zomboid autodetectada
	// (%UserProfile%\Zomboid). Vacío = autodetectar.
	ZomboidDir string `json:"zomboid_dir,omitempty"`

	// PerfConfigured indica que el jugador ya pasó por "Ajustes de rendimiento"
	// al menos una vez (para no insistir y para saber que los valores de abajo
	// son intencionales aunque sean 0).
	PerfConfigured bool `json:"perf_configured,omitempty"`

	// ChunkBudgetMs es el presupuesto por frame (ms) del fix anti-tirones (C11).
	// 0 = desactivado (vanilla). Se escribe como -Dpz.chunkBudgetMs en el json
	// del juego.
	ChunkBudgetMs int `json:"chunk_budget_ms,omitempty"`

	// MaxHeapMB es el -Xmx (MB) a fijar en el json del juego. 0 = no gestionar
	// (dejar el valor del juego como esté).
	MaxHeapMB int `json:"max_heap_mb,omitempty"`

	// MaxChunkGridWidth capa el ancho de la grilla de chunks del cliente (parche
	// C26). El cliente le pide al server exactamente este ancho, así que bajarlo
	// reduce la RAM que el server carga por este jugador (y algo la del cliente),
	// SIN los chunks negros que causaba capar del lado del server. 0 = sin tope
	// (vanilla: el ancho lo decide la resolución). Se escribe como
	// -Dpz.maxChunkGridWidth; el jar lo fuerza impar con piso 13. Rango 12..20.
	MaxChunkGridWidth int `json:"max_chunk_grid_width,omitempty"`

	// SafeTowReconnectOff desactiva el fix de reenganche de remolque sin tirón
	// (parche C18). El jar viene con el fix ACTIVADO por defecto, así que el
	// valor-cero (false) = fix activo y no escribimos nada en el json del juego.
	// Solo si el jugador lo pone en true escribimos "-Dpz.safeTowReconnect=false"
	// para volver al comportamiento vanilla (diagnóstico).
	SafeTowReconnectOff bool `json:"safe_tow_reconnect_off,omitempty"`

	// Viewpoint activa el mod Project Viewpoint: al Jugar se instala ZombieBuddy
	// en la carpeta del juego y se añade -agentlib:zbNative al json del juego
	// (ver internal/viewpoint).
	Viewpoint bool `json:"viewpoint,omitempty"`

	// ViewpointManualUpdates desactiva la actualización automática de
	// ZombieBuddy al Jugar. Al revés para que el valor-cero (false) = automática.
	ViewpointManualUpdates bool `json:"viewpoint_manual_updates,omitempty"`

	// ViewpointAgentOwned recuerda que la línea -agentlib:zbNative del json del
	// juego la puso el launcher. Solo así la quitamos al desactivar Viewpoint: si
	// la puso el instalador oficial de ZombieBuddy, no es nuestra.
	ViewpointAgentOwned bool `json:"viewpoint_agent_owned,omitempty"`

	// ViewpointAddedMods son los mods (ZombieBuddy/Viewpoint) que el launcher
	// activó en Zomboid\mods\default.txt. Al desactivar Viewpoint se quitan solo
	// esos: si el jugador ya los tenía activos a mano, no los tocamos.
	ViewpointAddedMods []string `json:"viewpoint_added_mods,omitempty"`
}

func file(baseDir string) string {
	return filepath.Join(baseDir, "pzlauncher.json")
}

// Load lee la config junto al ejecutable. Si no existe o está mal formada,
// devuelve una config vacía (comportamiento por defecto).
func Load(baseDir string) Config {
	var c Config
	data, err := os.ReadFile(file(baseDir))
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	return c
}

// Save escribe la config junto al ejecutable.
func Save(baseDir string, c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file(baseDir), append(data, '\n'), 0o644)
}
