package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"slices"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"pzlauncher/internal/config"
	"pzlauncher/internal/gamelaunch"
	"pzlauncher/internal/gameopts"
	"pzlauncher/internal/manifest"
	"pzlauncher/internal/paths"
	"pzlauncher/internal/selfupdate"
	"pzlauncher/internal/updater"
	"pzlauncher/internal/viewpoint"
	"pzlauncher/internal/zomboid"
)

// selfUpdateVariant es el sufijo de la clave en manifest.launcher_binaries para
// la autoactualización de ESTA GUI. El launcher de consola usa "windows/amd64";
// la GUI usa "windows/amd64-gui" para no bajarse por error el binario de consola.
const selfUpdateVariant = "-gui"

const pzAppID = 108600 // Project Zomboid en Steam

// App es el puente entre la ventana (frontend web) y la lógica del launcher
// (paquetes internal/*). Todo el estado mutable se protege con mu porque el
// flujo de actualización corre en una goroutine aparte mientras el frontend
// puede llamar a los métodos ligados.
type App struct {
	ctx         context.Context
	version     string
	manifestURL string

	once sync.Once
	mu   sync.Mutex

	exeDir  string
	cfg     config.Config
	m       *manifest.Manifest
	gameDir string
	offline bool
	ready   bool
	errMsg  string
}

// NewApp crea la App con la versión y la URL del manifiesto inyectadas en
// compilación (ver main.go).
func NewApp(version, manifestURL string) *App {
	return &App{version: version, manifestURL: manifestURL}
}

// startup guarda el contexto de Wails (necesario para emitir eventos y abrir
// diálogos nativos). Se llama antes de que cargue el frontend.
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// --- Tipos que viajan al frontend (JSON) ---

// StatePayload es el estado que el frontend necesita para pintar el menú.
type StatePayload struct {
	Ready      bool                 `json:"ready"`
	Offline    bool                 `json:"offline"`
	Version    string               `json:"version"`
	GameDir    string               `json:"gameDir"`
	Error      string               `json:"error"`
	LastNote   string               `json:"lastNote"`
	PatchNotes []manifest.PatchNote `json:"patchNotes"`
}

// SettingsPayload alimenta el formulario de "Ajustes de rendimiento".
type SettingsPayload struct {
	ChunkBudgetMs     int  `json:"chunkBudgetMs"`
	MaxHeapMB         int  `json:"maxHeapMB"`
	MaxChunkGridWidth int  `json:"maxChunkGridWidth"`
	SafeTowOff        bool `json:"safeTowOff"`
	RAMGB         int  `json:"ramGB"`
	SuggestHeapMB int  `json:"suggestHeapMB"`
	DefaultHeapMB int  `json:"defaultHeapMB"`
	Configured    bool `json:"configured"`
	HasGameJSON   bool `json:"hasGameJSON"`
}

// --- Ciclo de vida ---

// Start lo invoca el frontend UNA vez, después de registrar sus escuchas de
// eventos, para arrancar el flujo de actualización sin perder los primeros
// eventos de progreso. sync.Once evita que se dispare dos veces.
func (a *App) Start() { a.once.Do(func() { go a.run() }) }

// run ejecuta el flujo completo (igual que cmd/launcher) emitiendo progreso por
// eventos de Wails en vez de por la ventana de zenity.
func (a *App) run() {
	rep := &wailsReporter{ctx: a.ctx}

	// Limpia un posible "<exe>.old" y temporales de descarga que dejara una
	// autoactualización anterior (igual que el launcher de consola).
	selfupdate.CleanupOld()

	exeDir, err := paths.ExecutableDir()
	if err != nil {
		a.fail("No se pudo determinar la carpeta del launcher: " + err.Error())
		return
	}
	a.mu.Lock()
	a.exeDir = exeDir
	a.cfg = config.Load(exeDir)
	a.mu.Unlock()
	log.Printf("pzlauncher-gui %s | carpeta del exe: %s", a.version, exeDir)

	url := a.manifestURL
	if env := os.Getenv("PZL_MANIFEST_URL"); env != "" {
		url = env
	}

	rep.Stage("Comprobando actualizaciones…")
	m, ferr := manifest.Fetch(url)
	offline := ferr != nil
	var gameDir string
	if offline {
		// Sin conexión no debería impedir jugar: avisamos y seguimos con un
		// manifiesto mínimo para poder lanzar el juego igualmente.
		log.Printf("[aviso] no se pudo obtener el manifiesto: %v", ferr)
		m = offlineManifest()
		if dir, derr := a.resolveGameDir(exeDir, pzAppID); derr == nil {
			gameDir = dir
		}
	} else {
		// Autoactualización de la propia GUI. Usa la clave "…-gui" del manifiesto
		// (ver selfUpdateVariant) para no bajarse el binario de consola. Si relanza,
		// el nuevo proceso ya está en marcha y este debe cerrarse de inmediato.
		relaunched, uerr := selfupdate.MaybeUpdate(a.version, m, rep, selfUpdateVariant)
		if uerr != nil {
			log.Printf("[aviso] fallo autoactualizando la GUI: %v", uerr)
		} else if relaunched {
			wruntime.Quit(a.ctx)
			return
		}

		appID := m.SteamAppID
		if appID == 0 {
			appID = pzAppID
		}
		gameDir, err = a.resolveGameDir(exeDir, appID)
		if err != nil {
			a.fail(err.Error())
			return
		}
		log.Printf("carpeta del juego: %s", gameDir)
		if err := updater.SyncFiles(gameDir, m.Files, rep); err != nil {
			a.fail("Error actualizando ficheros: " + err.Error())
			return
		}
	}

	a.mu.Lock()
	a.m = m
	a.gameDir = gameDir
	a.offline = offline
	a.ready = true
	a.mu.Unlock()

	rep.Success("Listo")
	wruntime.EventsEmit(a.ctx, "ready", a.GetState())
}

// fail registra el error y lo manda al frontend (que muestra la pantalla de
// error en vez del menú).
func (a *App) fail(msg string) {
	log.Println("ERROR:", msg)
	a.mu.Lock()
	a.errMsg = msg
	a.mu.Unlock()
	wruntime.EventsEmit(a.ctx, "error", msg)
}

// GetState devuelve el estado actual; el frontend lo usa al arrancar (por si se
// suscribió tarde al evento "ready") y tras recibir "ready".
func (a *App) GetState() StatePayload {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := StatePayload{
		Ready:   a.ready,
		Offline: a.offline,
		Version: a.version,
		GameDir: a.gameDir,
		Error:   a.errMsg,
	}
	if a.m != nil {
		st.PatchNotes = a.m.PatchNotes
		if len(a.m.PatchNotes) > 0 {
			st.LastNote = a.m.PatchNotes[0].Title
		}
	}
	return st
}

// --- Acciones del menú (ligadas al frontend) ---

// Play aplica los ajustes y lanza el juego vía Steam; después cierra la ventana.
func (a *App) Play() error {
	a.mu.Lock()
	m, gameDir, cfg := a.m, a.gameDir, a.cfg
	a.mu.Unlock()
	if gameDir == "" || m == nil {
		return fmt.Errorf("No se encontró la carpeta del juego.")
	}
	a.applyGameOpts(gameDir, cfg)
	if err := a.applyViewpoint(gameDir); err != nil {
		return err
	}
	if err := gamelaunch.Launch(gameDir, m); err != nil {
		return err
	}
	wruntime.Quit(a.ctx)
	return nil
}

// Quit cierra el launcher sin arrancar el juego.
func (a *App) Quit() { wruntime.Quit(a.ctx) }

// CleanLogs borra los logs del cliente y devuelve un resumen para mostrar.
func (a *App) CleanLogs() (string, error) {
	dir, ok := a.resolveZomboidDir()
	if !ok {
		return "", fmt.Errorf("No se encontró la carpeta de Zomboid; no se limpiaron logs.")
	}
	removed, err := zomboid.CleanLogs(dir)
	if err != nil {
		return "", fmt.Errorf("Error limpiando logs: %w", err)
	}
	log.Printf("Logs limpiados: %d elemento(s) en %s", removed, dir)
	if removed == 0 {
		return "No había logs que limpiar.", nil
	}
	return fmt.Sprintf("Logs limpiados: %d elemento(s).", removed), nil
}

// OpenLogs abre la carpeta de datos de Zomboid en el explorador.
func (a *App) OpenLogs() error {
	dir, ok := a.resolveZomboidDir()
	if !ok {
		return fmt.Errorf("No se encontró la carpeta de Zomboid.")
	}
	if err := zomboid.OpenDir(dir); err != nil {
		return fmt.Errorf("No se pudo abrir la carpeta:\n%s\n\n%w", dir, err)
	}
	return nil
}

// GetSettings devuelve los ajustes actuales más la RAM detectada y la sugerencia
// de heap, para prerrellenar el formulario.
func (a *App) GetSettings() SettingsPayload {
	a.mu.Lock()
	cfg, gameDir := a.cfg, a.gameDir
	a.mu.Unlock()
	sp := SettingsPayload{
		ChunkBudgetMs:     cfg.ChunkBudgetMs,
		MaxHeapMB:         cfg.MaxHeapMB,
		MaxChunkGridWidth: cfg.MaxChunkGridWidth,
		SafeTowOff:        cfg.SafeTowReconnectOff,
		RAMGB:             gameopts.TotalRAMGB(),
		SuggestHeapMB:     gameopts.SuggestHeapMB(),
		Configured:        cfg.PerfConfigured,
	}
	sp.DefaultHeapMB = cfg.MaxHeapMB
	if sp.DefaultHeapMB == 0 {
		sp.DefaultHeapMB = sp.SuggestHeapMB
	}
	if gameDir != "" {
		if _, err := os.Stat(gameopts.Path(gameDir)); err == nil {
			sp.HasGameJSON = true
		}
	}
	return sp
}

// SaveSettings valida, guarda en pzlauncher.json y aplica los ajustes al
// ProjectZomboid64.json del juego. Devuelve un resumen legible.
func (a *App) SaveSettings(chunkMs, heapMB, gridWidth int, towOff bool) (string, error) {
	a.mu.Lock()
	gameDir, exeDir, cfg := a.gameDir, a.exeDir, a.cfg
	a.mu.Unlock()
	if gameDir == "" {
		return "", fmt.Errorf("No se encontró la carpeta del juego; no puedo aplicar ajustes.")
	}
	if _, err := os.Stat(gameopts.Path(gameDir)); err != nil {
		return "", fmt.Errorf("No encontré ProjectZomboid64.json en:\n%s", gameDir)
	}

	cfg.ChunkBudgetMs = clamp(chunkMs, 0, 50)
	cfg.MaxHeapMB = clampHeap(heapMB)
	cfg.MaxChunkGridWidth = clampGrid(gridWidth)
	cfg.SafeTowReconnectOff = towOff
	cfg.PerfConfigured = true
	if err := config.Save(exeDir, cfg); err != nil {
		return "", fmt.Errorf("No se pudo guardar la config: %w", err)
	}
	if err := gameopts.Apply(gameDir, cfg.ChunkBudgetMs, cfg.MaxHeapMB, cfg.MaxChunkGridWidth, cfg.SafeTowReconnectOff); err != nil {
		return "", fmt.Errorf("No se pudieron aplicar los ajustes: %w", err)
	}
	a.mu.Lock()
	a.cfg = cfg
	a.mu.Unlock()
	return perfSummary(cfg), nil
}

// --- Viewpoint (ver internal/viewpoint) ---

// GetViewpoint devuelve el estado para el panel de Viewpoint.
func (a *App) GetViewpoint() viewpoint.Status {
	a.mu.Lock()
	cfg, gameDir := a.cfg, a.gameDir
	a.mu.Unlock()
	if runtime.GOOS != "windows" {
		return viewpoint.Status{} // zbNative.dll es solo Windows
	}
	st := viewpoint.GetStatus(gameDir, cfg.Viewpoint, !cfg.ViewpointManualUpdates,
		gameDir != "" && gameopts.HasAgent(gameDir) && !cfg.ViewpointAgentOwned)
	zdir := zomboid.Resolve(cfg.ZomboidDir)
	st.ModsActive = viewpoint.ModsActiveIn(zdir)
	st.ZombieBuddyPresent = zombieBuddyModActive(zdir) ||
		(gameDir != "" && (viewpoint.AgentFilesPresent(gameDir) || gameopts.HasAgent(gameDir)))
	return st
}

func zombieBuddyModActive(zomboidDir string) bool {
	data, err := os.ReadFile(viewpoint.DefaultModsFile(zomboidDir))
	return err == nil && slices.Contains(viewpoint.ActiveMods(string(data)), "ZombieBuddy")
}

// UninstallResult es la respuesta de UninstallZombieBuddy: el estado nuevo y un
// resumen para mostrar (incluye qué falta hacer a mano, si algo).
type UninstallResult struct {
	Status  viewpoint.Status `json:"status"`
	Message string           `json:"message"`
}

// UninstallZombieBuddy quita ZombieBuddy de esta PC, venga de donde venga (este
// launcher, su instalador oficial o a mano): apaga Viewpoint, saca el agente del
// json del juego, desactiva ZombieBuddy y Viewpoint en el menú de mods y borra
// sus ficheros de la carpeta del juego. Las Opciones de lanzamiento de Steam no
// se editan (Steam pisa ese fichero mientras está abierto): si ZombieBuddy está
// ahí, se le dice al jugador qué borrar. La suscripción de Workshop y las
// aprobaciones de ZombieBuddy (%UserProfile%\.zombie_buddy) no se tocan.
func (a *App) UninstallZombieBuddy() (UninstallResult, error) {
	answer, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:  wruntime.QuestionDialog,
		Title: "Desinstalar ZombieBuddy",
		Message: "Se desactiva Viewpoint y se borran zbNative.dll y ZombieBuddy.jar " +
			"de la carpeta del juego. Cerrá el juego antes.\n\n¿Desinstalar ZombieBuddy?",
	})
	if err != nil || answer != "Yes" {
		return UninstallResult{Status: a.GetViewpoint()}, nil // cancelado
	}

	a.mu.Lock()
	gameDir, exeDir := a.gameDir, a.exeDir
	a.cfg.Viewpoint = false
	a.cfg.ViewpointAgentOwned = false
	a.cfg.ViewpointAddedMods = nil
	cfg := a.cfg
	a.mu.Unlock()
	if err := config.Save(exeDir, cfg); err != nil {
		log.Printf("[aviso] no se pudo guardar la config: %v", err)
	}

	var done []string
	if gameDir != "" && gameopts.HasAgent(gameDir) {
		if err := gameopts.SetAgent(gameDir, false); err != nil {
			return UninstallResult{a.GetViewpoint(), ""}, fmt.Errorf("No se pudo sacar ZombieBuddy de ProjectZomboid64.json: %w", err)
		}
		done = append(done, "Quitado de ProjectZomboid64.json.")
	}
	zdir := zomboid.Resolve(cfg.ZomboidDir)
	if zombieBuddyModActive(zdir) || viewpoint.ModsActiveIn(zdir) {
		if err := viewpoint.DisableMods(zdir, viewpoint.ModIDs); err != nil {
			return UninstallResult{a.GetViewpoint(), ""}, fmt.Errorf("No se pudieron desactivar los mods: %w", err)
		}
		done = append(done, "ZombieBuddy y Viewpoint desactivados en el menú de mods.")
	}
	if gameDir != "" && viewpoint.AgentFilesPresent(gameDir) {
		if err := viewpoint.RemoveAgentFiles(gameDir); err != nil {
			return UninstallResult{a.GetViewpoint(), ""}, err
		}
		done = append(done, "Borrados zbNative.dll y ZombieBuddy.jar de la carpeta del juego.")
	}

	msg := "ZombieBuddy desinstalado."
	if len(done) == 0 {
		msg = "No había nada de ZombieBuddy que quitar en el juego."
	}
	for _, d := range done {
		msg += "\n• " + d
	}
	if viewpoint.SteamOptionsLoadAgent() {
		msg += "\n\nFalta un paso a mano: en Steam, clic derecho en Project Zomboid → " +
			"Propiedades → Opciones de lanzamiento, y borrá \"-agentlib:zbNative\" " +
			"(y el \"--\" que lo acompaña, si no lo usás para otra cosa)."
	}
	return UninstallResult{a.GetViewpoint(), msg}, nil
}

// SaveViewpoint guarda los dos checks. Se aplican al Jugar (igual que los
// ajustes de rendimiento), salvo apagarlo, que quita el agente ya mismo.
func (a *App) SaveViewpoint(enabled, autoUpdate bool) (viewpoint.Status, error) {
	a.mu.Lock()
	a.cfg.Viewpoint = enabled
	a.cfg.ViewpointManualUpdates = !autoUpdate
	cfg, exeDir, gameDir := a.cfg, a.exeDir, a.gameDir
	a.mu.Unlock()
	if err := config.Save(exeDir, cfg); err != nil {
		return a.GetViewpoint(), fmt.Errorf("No se pudo guardar la config: %w", err)
	}
	if !enabled && gameDir != "" {
		if err := a.applyViewpoint(gameDir); err != nil {
			return a.GetViewpoint(), err
		}
	}
	return a.GetViewpoint(), nil
}

// UpdateZombieBuddy copia ya mismo la versión de Workshop a la carpeta del juego.
func (a *App) UpdateZombieBuddy() (viewpoint.Status, error) {
	a.mu.Lock()
	gameDir := a.gameDir
	a.mu.Unlock()
	if gameDir == "" {
		return a.GetViewpoint(), fmt.Errorf("No se encontró la carpeta del juego.")
	}
	if err := viewpoint.UpdateNow(gameDir); err != nil {
		return a.GetViewpoint(), err
	}
	return a.GetViewpoint(), nil
}

// OpenWorkshop abre la página de Workshop de ZombieBuddy o de Viewpoint para
// que el jugador se suscriba (el launcher nunca se suscribe por él).
func (a *App) OpenWorkshop(which string) {
	id := viewpoint.ViewpointWorkshopID
	if which == "zombiebuddy" {
		id = viewpoint.ZombieBuddyWorkshopID
	}
	wruntime.BrowserOpenURL(a.ctx, viewpoint.WorkshopURL(id))
}

// applyViewpoint deja el juego listo según el check:
//   - activado -> instala/actualiza ZombieBuddy, pone el agente en el json del
//     juego y activa ZombieBuddy + Viewpoint en la lista de mods del menú
//     principal (ver viewpoint.ModIDs para por qué eso alcanza en multijugador).
//   - apagado -> deshace solo lo que hizo el launcher.
//
// Si ZombieBuddy ya se carga por fuera (Opciones de Steam o su instalador
// oficial) no tocamos el agente, pero los mods sí hacen falta igual.
func (a *App) applyViewpoint(gameDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	a.mu.Lock()
	cfg, exeDir := a.cfg, a.exeDir
	a.mu.Unlock()
	if _, err := os.Stat(gameopts.Path(gameDir)); err != nil {
		if cfg.Viewpoint {
			return fmt.Errorf("No encontré ProjectZomboid64.json en:\n%s", gameDir)
		}
		return nil
	}

	hasAgent := gameopts.HasAgent(gameDir)
	owned := cfg.ViewpointAgentOwned
	external := (hasAgent && !owned) || viewpoint.SteamOptionsLoadAgent()
	switch {
	case cfg.Viewpoint && !external:
		if err := viewpoint.Ensure(gameDir, !cfg.ViewpointManualUpdates); err != nil {
			return err
		}
		if !hasAgent {
			if err := gameopts.SetAgent(gameDir, true); err != nil {
				return fmt.Errorf("No se pudo activar ZombieBuddy: %w", err)
			}
		}
		owned = true
	case !cfg.Viewpoint && hasAgent && owned:
		if err := gameopts.SetAgent(gameDir, false); err != nil {
			return fmt.Errorf("No se pudo desactivar ZombieBuddy: %w", err)
		}
		owned = false
	case !hasAgent:
		owned = false // Steam reescribió el json (update / verificar integridad)
	}

	addedMods, modsErr := a.applyViewpointMods(cfg)

	if owned != cfg.ViewpointAgentOwned || !sameStrings(addedMods, cfg.ViewpointAddedMods) {
		a.mu.Lock()
		a.cfg.ViewpointAgentOwned = owned
		a.cfg.ViewpointAddedMods = addedMods
		cfg = a.cfg
		a.mu.Unlock()
		if err := config.Save(exeDir, cfg); err != nil {
			log.Printf("[aviso] no se pudo guardar la config: %v", err)
		}
	}
	return modsErr
}

// applyViewpointMods activa (o, al desactivar, quita) ZombieBuddy y Viewpoint
// en Zomboid\mods\default.txt. Devuelve la lista actualizada de mods que puso
// el launcher, para guardarla en la config.
func (a *App) applyViewpointMods(cfg config.Config) ([]string, error) {
	dir := zomboid.Resolve(cfg.ZomboidDir)
	if cfg.Viewpoint {
		added, err := viewpoint.EnableMods(dir)
		if err != nil {
			return cfg.ViewpointAddedMods, fmt.Errorf("No se pudieron activar los mods en %s: %w", viewpoint.DefaultModsFile(dir), err)
		}
		return mergeStrings(cfg.ViewpointAddedMods, added), nil
	}
	if err := viewpoint.DisableMods(dir, cfg.ViewpointAddedMods); err != nil {
		return cfg.ViewpointAddedMods, fmt.Errorf("No se pudieron desactivar los mods en %s: %w", viewpoint.DefaultModsFile(dir), err)
	}
	return nil, nil
}

func mergeStrings(a, b []string) []string {
	out := append([]string(nil), a...)
	for _, s := range b {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func sameStrings(a, b []string) bool { return slices.Equal(a, b) }

// --- Helpers (portados de cmd/launcher) ---

func (a *App) applyGameOpts(gameDir string, cfg config.Config) {
	if gameDir == "" || !cfg.PerfConfigured {
		return
	}
	if err := gameopts.Apply(gameDir, cfg.ChunkBudgetMs, cfg.MaxHeapMB, cfg.MaxChunkGridWidth, cfg.SafeTowReconnectOff); err != nil {
		log.Printf("[aviso] no se pudieron aplicar los ajustes de rendimiento: %v", err)
	}
}

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

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampGrid normaliza el ancho de grilla de chunks: 0 = sin tope (vanilla);
// cualquier valor positivo se acota a [12,20] (el jar lo fuerza impar con piso 13).
func clampGrid(v int) int {
	if v <= 0 {
		return 0
	}
	return clamp(v, 12, 20)
}

func clampHeap(v int) int {
	if v <= 0 {
		return 0
	}
	return clamp(v, 1536, 32768)
}

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
