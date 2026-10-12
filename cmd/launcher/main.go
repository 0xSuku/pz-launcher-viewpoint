// Command launcher es el actualizador/lanzador multiplataforma de Project Zomboid.
//
// Flujo al arrancar:
//  1. Descarga el manifiesto (manifest.json).
//  2. Si el manifiesto anuncia una versión del launcher más nueva, se
//     autoactualiza y se relanza.
//  3. Localiza la carpeta de INSTALACIÓN del juego (autodetección vía Steam; si
//     no, pregunta y la recuerda) y sincroniza sus ficheros (p. ej.
//     projectzomboid.jar) por hash, mostrando progreso y tiempo estimado.
//  4. Pregunta al usuario qué hacer: Jugar / Limpiar logs / Salir.
//  5. Lanza el juego vía Steam (arranca Steam si está cerrado).
//
// El ejecutable NO necesita estar dentro de la carpeta del juego: detecta dónde
// está instalado. Muestra el progreso en una ventana nativa (zenity); si no hay
// entorno gráfico, cae a registrar el progreso por consola.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"pzlauncher/internal/config"
	"pzlauncher/internal/gamelaunch"
	"pzlauncher/internal/gameopts"
	"pzlauncher/internal/manifest"
	"pzlauncher/internal/paths"
	"pzlauncher/internal/selfupdate"
	"pzlauncher/internal/steam"
	"pzlauncher/internal/ui"
	"pzlauncher/internal/updater"
	"pzlauncher/internal/zomboid"
)

const pzAppID = 108600 // Project Zomboid en Steam

// Valores inyectados en tiempo de compilación con -ldflags:
//
//	go build -ldflags "-X main.version=1.0.0 -X main.manifestURL=https://..."
var (
	version     = "0.0.0-dev"
	manifestURL = "https://github.com/patagoniapz/pz-launcher/releases/latest/download/manifest.json"
)

func main() {
	log.SetFlags(0)

	var (
		noLaunch    = flag.Bool("no-launch", false, "actualizar pero no arrancar el juego")
		noSelf      = flag.Bool("no-self-update", false, "no autoactualizar el launcher")
		showVer     = flag.Bool("version", false, "mostrar la versión y salir")
		detect      = flag.Bool("detect", false, "mostrar la carpeta del juego detectada y salir")
		urlOverride = flag.String("manifest", "", "URL del manifiesto (sobrescribe el valor por defecto)")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("pzlauncher " + version)
		return
	}

	if *detect {
		if dir, ok := steam.FindGameDir(pzAppID); ok {
			fmt.Println(dir)
		} else {
			fmt.Println("No se detectó la carpeta de Project Zomboid.")
		}
		return
	}

	// La URL del manifiesto se puede sobrescribir por flag o por variable de
	// entorno (útil para probar contra un servidor local antes de publicar).
	url := manifestURL
	if env := os.Getenv("PZL_MANIFEST_URL"); env != "" {
		url = env
	}
	if *urlOverride != "" {
		url = *urlOverride
	}

	// Limpia un posible "<exe>.old" dejado por una autoactualización anterior.
	selfupdate.CleanupOld()

	ui.SetVersion(version) // la versión aparece en el título de las ventanas
	rep := ui.New()
	defer rep.Close()

	exeDir, err := paths.ExecutableDir()
	if err != nil {
		ui.ShowError("No se pudo determinar la carpeta del launcher: " + err.Error())
		os.Exit(1)
	}
	cfg := config.Load(exeDir)
	log.Printf("pzlauncher %s | carpeta del exe: %s", version, exeDir)

	rep.Stage("Comprobando actualizaciones…")
	m, err := manifest.Fetch(url)
	offline := err != nil
	var gameDir string
	if offline {
		// Sin conexión no debería impedir jugar: avisamos y seguimos con un
		// manifiesto mínimo para poder lanzar el juego igualmente.
		log.Printf("[aviso] no se pudo obtener el manifiesto: %v", err)
		m = offlineManifest()
		// Resolver la carpeta del juego igual (best-effort) para que ajustes y
		// autoaplicado funcionen sin conexión. Sin exit si falla.
		if dir, derr := resolveGameDir(exeDir, &cfg, pzAppID); derr == nil {
			gameDir = dir
		}
	} else {
		if !*noSelf {
			relaunched, err := selfupdate.MaybeUpdate(version, m, rep, "")
			if err != nil {
				log.Printf("[aviso] fallo autoactualizando el launcher: %v", err)
			} else if relaunched {
				// El nuevo binario ya está en marcha; este proceso debe salir.
				return
			}
		}

		appID := m.SteamAppID
		if appID == 0 {
			appID = pzAppID
		}
		gameDir, err = resolveGameDir(exeDir, &cfg, appID)
		if err != nil {
			ui.ShowError(err.Error())
			os.Exit(1)
		}
		log.Printf("carpeta del juego: %s", gameDir)

		if err := updater.SyncFiles(gameDir, m.Files, rep); err != nil {
			ui.ShowError("Error actualizando ficheros: " + err.Error())
			os.Exit(1)
		}
	}

	// Cerramos el diálogo de progreso antes de preguntar la acción.
	rep.Close()

	if *noLaunch {
		return
	}

	instruction := "Project Zomboid está listo."
	if offline {
		instruction = "Sin conexión (no se comprobaron actualizaciones)."
	}
	// El contenido (bajo el título) adelanta la última novedad de un vistazo.
	content := "¿Qué quieres hacer?"
	if len(m.PatchNotes) > 0 {
		content = "Última novedad: " + m.PatchNotes[0].Title + "\n\n¿Qué quieres hacer?"
	}

	hasNotes := len(m.PatchNotes) > 0
	for {
		switch ui.AskAction(instruction, content, hasNotes) {
		case ui.ActionQuit:
			return
		case ui.ActionPlay:
			launchGame(gameDir, m, cfg)
			return
		case ui.ActionCleanLogs:
			// Limpia y vuelve a mostrar el diálogo (no arranca el juego).
			cleanLogs(exeDir, &cfg)
		case ui.ActionOpenLogs:
			// Abre la carpeta de logs y vuelve a mostrar el diálogo.
			openLogsFolder(exeDir, &cfg)
		case ui.ActionSettings:
			// Configura rendimiento y vuelve a mostrar el diálogo.
			configurePerf(exeDir, gameDir, &cfg)
		case ui.ActionPatchNotes:
			// Muestra las novedades y vuelve a mostrar el diálogo.
			showPatchNotes(m)
		}
	}
}

// showPatchNotes muestra la lista de parches/novedades (en español) que viajan
// en el manifiesto. Se presenta en un único cuadro de información, que sí admite
// varias líneas y crece con el contenido.
func showPatchNotes(m *manifest.Manifest) {
	if m == nil || len(m.PatchNotes) == 0 {
		ui.Info("No hay novedades disponibles (sin conexión o aún no publicadas).")
		return
	}
	var b strings.Builder
	b.WriteString("Novedades y parches aplicados\n")
	for _, n := range m.PatchNotes {
		b.WriteString("\n• ")
		if n.ID != "" {
			b.WriteString(n.ID + " — ")
		}
		b.WriteString(n.Title)
		if n.Date != "" {
			b.WriteString("  (" + n.Date + ")")
		}
		if n.Desc != "" {
			b.WriteString("\n   " + n.Desc)
		}
		b.WriteString("\n")
	}
	ui.Info(b.String())
}

// resolveGameDir localiza la carpeta de instalación del juego con esta prioridad:
//  1. Variable de entorno PZL_GAME_DIR.
//  2. Valor guardado en la config (pzlauncher.json).
//  3. El propio exe ya está dentro de la carpeta del juego.
//  4. Autodetección vía Steam.
//  5. Preguntar al usuario (y recordar la elección).
func resolveGameDir(exeDir string, cfg *config.Config, appID int) (string, error) {
	if env := os.Getenv("PZL_GAME_DIR"); env != "" {
		return env, nil
	}
	if cfg.GameDir != "" && dirExists(cfg.GameDir) {
		return cfg.GameDir, nil
	}
	if hasGameExe(exeDir) {
		return exeDir, nil
	}
	if dir, ok := steam.FindGameDir(appID); ok {
		return dir, nil
	}
	if picked, ok := ui.PickFolder("Selecciona la carpeta de instalación de Project Zomboid"); ok {
		cfg.GameDir = picked
		_ = config.Save(exeDir, *cfg)
		return picked, nil
	}
	return "", errors.New("No se encontró la carpeta de instalación de Project Zomboid.")
}

// hasGameExe indica si dir parece la carpeta del juego (contiene su ejecutable).
func hasGameExe(dir string) bool {
	for _, name := range []string{"ProjectZomboid64.exe", "ProjectZomboid64"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// resolveZomboidDir localiza la carpeta de DATOS de Zomboid (logs, partidas…).
// Si no se encuentra, pide al usuario que la seleccione y recuerda la elección.
// Devuelve ("", false) si no se pudo determinar. Esta es la misma carpeta que
// se usa tanto para limpiar logs como para abrirla en el explorador.
func resolveZomboidDir(exeDir string, cfg *config.Config) (string, bool) {
	dir := zomboid.Resolve(cfg.ZomboidDir)
	if zomboid.Exists(dir) {
		return dir, true
	}
	picked, ok := ui.PickFolder("Selecciona tu carpeta Zomboid (donde están los logs)")
	if !ok {
		return "", false
	}
	cfg.ZomboidDir = picked
	_ = config.Save(exeDir, *cfg)
	return picked, true
}

// cleanLogs borra los logs del cliente de la carpeta de datos de Zomboid.
func cleanLogs(exeDir string, cfg *config.Config) {
	dir, ok := resolveZomboidDir(exeDir, cfg)
	if !ok {
		ui.ShowError("No se encontró la carpeta de Zomboid; no se limpiaron logs.")
		return
	}

	removed, err := zomboid.CleanLogs(dir)
	if err != nil {
		ui.ShowError("Error limpiando logs: " + err.Error())
		return
	}
	log.Printf("Logs limpiados: %d elemento(s) en %s", removed, dir)
	if removed == 0 {
		ui.Info("No había logs que limpiar.")
	} else {
		ui.Info(fmt.Sprintf("Logs limpiados: %d elemento(s).", removed))
	}
}

// openLogsFolder abre, en el explorador de archivos del sistema, la carpeta de
// datos de Zomboid de ESTE usuario (la misma que se usa para limpiar los logs).
func openLogsFolder(exeDir string, cfg *config.Config) {
	dir, ok := resolveZomboidDir(exeDir, cfg)
	if !ok {
		ui.ShowError("No se encontró la carpeta de Zomboid.")
		return
	}
	if err := zomboid.OpenDir(dir); err != nil {
		ui.ShowError("No se pudo abrir la carpeta:\n" + dir + "\n\n" + err.Error())
	}
}

func launchGame(gameDir string, m *manifest.Manifest, cfg config.Config) {
	applyGameOpts(gameDir, cfg)
	if err := gamelaunch.Launch(gameDir, m); err != nil {
		ui.ShowError(err.Error())
		os.Exit(1)
	}
}

// applyGameOpts reescribe los ajustes de rendimiento en el ProjectZomboid64.json
// antes de cada lanzamiento (self-heal si Steam pisó el fichero). Solo actúa si
// el jugador ya pasó por los ajustes; si no, no toca nada (comportamiento del
// juego intacto).
func applyGameOpts(gameDir string, cfg config.Config) {
	if gameDir == "" || !cfg.PerfConfigured {
		return
	}
	if err := gameopts.Apply(gameDir, cfg.ChunkBudgetMs, cfg.MaxHeapMB, cfg.MaxChunkGridWidth, cfg.SafeTowReconnectOff); err != nil {
		log.Printf("[aviso] no se pudieron aplicar los ajustes de rendimiento: %v", err)
	}
}

// configurePerf pregunta al jugador el fix anti-tirones y la memoria, guarda la
// elección en pzlauncher.json y la aplica al ProjectZomboid64.json del juego.
func configurePerf(exeDir, gameDir string, cfg *config.Config) {
	if gameDir == "" {
		ui.ShowError("No se encontró la carpeta del juego; no puedo aplicar ajustes.")
		return
	}
	if _, err := os.Stat(gameopts.Path(gameDir)); err != nil {
		ui.ShowError("No encontré ProjectZomboid64.json en:\n" + gameDir)
		return
	}

	// 1) Fix anti-tirones (chunk budget, parche C11). La explicación va en un
	// cuadro aparte (multilínea); el campo de entrada solo lleva una etiqueta
	// corta porque recorta los textos largos.
	chunkHelp := "Tirones al moverse (fix anti-stutter)\n\n" +
		"Reparte la carga del mapa en varios frames para suavizar los tironcitos " +
		"al desplazarte o entrar a zonas nuevas.\n\n" +
		"Más bajo = más fluido, pero el terreno aparece un pelín más tarde.\n" +
		"Recomendado: 4-8.   0 (o vacío) = desactivado (vanilla)."
	if s, ok := ui.AskNumber(chunkHelp, "Fix anti-tirones, ms (0 = desactivado):", strconv.Itoa(cfg.ChunkBudgetMs)); ok {
		cfg.ChunkBudgetMs = clamp(parseIntOr(s, cfg.ChunkBudgetMs), 0, 50)
	}

	// 2) Memoria (-Xmx).
	ramGB := gameopts.TotalRAMGB()
	suggest := gameopts.SuggestHeapMB()
	defHeap := cfg.MaxHeapMB
	if defHeap == 0 {
		defHeap = suggest
	}
	ramLine := "No pude detectar tu RAM."
	if ramGB > 0 {
		ramLine = fmt.Sprintf("Tu PC tiene %d GB de RAM.", ramGB)
	}
	sugLine := ""
	if suggest > 0 {
		sugLine = fmt.Sprintf("   Sugerido: %d MB.", suggest)
	}
	memHelp := "Memoria para el juego (-Xmx, en MB)\n\n" +
		ramLine + " PZ usa 3072 MB (3 GB) por defecto.\n" +
		"Darle más puede reducir tirones por recolección de basura; pasarte " +
		"puede dejar sin RAM al resto del sistema." + sugLine + "\n" +
		"0 (o vacío) = no tocar el valor del juego."
	if s, ok := ui.AskNumber(memHelp, "Memoria -Xmx, MB (0 = no tocar):", strconv.Itoa(defHeap)); ok {
		cfg.MaxHeapMB = clampHeap(parseIntOr(s, cfg.MaxHeapMB))
	}

	// 3) Ancho de carga de mapa (cap de grilla de chunks, parche C26). Baja la RAM
	// que el server carga por este jugador (el cliente le pide exactamente este
	// ancho). Sin chunks negros, a costa de un poco menos de distancia de visión.
	gridHelp := "Ancho de carga de mapa (ahorro de RAM del servidor)\n\n" +
		"Baja cuánto mapa carga tu cliente. Como el servidor carga por vos " +
		"exactamente lo que pedís, bajarlo le ahorra memoria al servidor (y algo " +
		"a vos).\n\n" +
		"Se ve un poco menos de distancia, pero completo (sin partes negras).\n" +
		"Recomendado: 16.   0 (o vacío) = sin tope (vanilla). Rango 12-20."
	if s, ok := ui.AskNumber(gridHelp, "Ancho de carga de mapa (0 = sin tope):", strconv.Itoa(cfg.MaxChunkGridWidth)); ok {
		cfg.MaxChunkGridWidth = clampGrid(parseIntOr(s, cfg.MaxChunkGridWidth))
	}

	// 4) Fix de reenganche de remolque (parche C18). Viene activado; esto es un
	// interruptor para volver a vanilla si hiciera falta diagnosticar.
	towHelp := "Fix de remolque (anti-choque al reenganchar)\n\n" +
		"Evita que un auto que remolca un trailer u otro auto se re-enganche de " +
		"golpe tras una desincronización y provoque un choque entre ambos.\n\n" +
		"Recomendado: Activado. Desactívalo solo si querés el comportamiento " +
		"vanilla para una prueba."
	towOn := ui.AskYesNo(towHelp, "Activado (recomendado)", "Desactivado (vanilla)", !cfg.SafeTowReconnectOff)
	cfg.SafeTowReconnectOff = !towOn

	cfg.PerfConfigured = true
	_ = config.Save(exeDir, *cfg)

	if err := gameopts.Apply(gameDir, cfg.ChunkBudgetMs, cfg.MaxHeapMB, cfg.MaxChunkGridWidth, cfg.SafeTowReconnectOff); err != nil {
		ui.ShowError("No se pudieron aplicar los ajustes: " + err.Error())
		return
	}
	ui.Info(perfSummary(*cfg))
}

// perfSummary describe los ajustes aplicados para el aviso final.
func perfSummary(cfg config.Config) string {
	chunk := "desactivado (vanilla)"
	if cfg.ChunkBudgetMs > 0 {
		chunk = fmt.Sprintf("%d ms", cfg.ChunkBudgetMs)
	}
	heap := "sin cambios (default del juego)"
	if cfg.MaxHeapMB > 0 {
		heap = fmt.Sprintf("%d MB", cfg.MaxHeapMB)
	}
	tow := "activado (recomendado)"
	if cfg.SafeTowReconnectOff {
		tow = "desactivado (vanilla)"
	}
	grid := "sin tope (vanilla)"
	if cfg.MaxChunkGridWidth >= 12 {
		grid = fmt.Sprintf("%d", cfg.MaxChunkGridWidth)
	}
	return "Ajustes aplicados (al arrancar el juego):\n" +
		"• Fix anti-tirones: " + chunk + "\n" +
		"• Memoria (-Xmx): " + heap + "\n" +
		"• Ancho de carga de mapa: " + grid + "\n" +
		"• Fix de remolque: " + tow
}

// parseIntOr convierte s a entero; vacío = 0 (desactivar); inválido = def.
func parseIntOr(s string, def int) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// clamp acota v al rango [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampHeap acota el -Xmx: 0 = no gestionar; si no, entre 1536 y 32768 MB.
func clampHeap(v int) int {
	if v <= 0 {
		return 0
	}
	return clamp(v, 1536, 32768)
}

// clampGrid normaliza el ancho de grilla de chunks: 0 = sin tope (vanilla);
// cualquier valor positivo se acota a [12,20] (el jar lo fuerza impar, piso 13).
func clampGrid(v int) int {
	if v <= 0 {
		return 0
	}
	return clamp(v, 12, 20)
}

// offlineManifest da el mínimo para poder lanzar el juego sin conexión.
func offlineManifest() *manifest.Manifest {
	exe := "ProjectZomboid64.exe"
	if runtime.GOOS != "windows" {
		exe = "./ProjectZomboid64"
	}
	return &manifest.Manifest{
		SteamAppID: pzAppID,
		Launch:     map[string]manifest.LaunchSpec{runtime.GOOS: {Exe: exe}},
	}
}
