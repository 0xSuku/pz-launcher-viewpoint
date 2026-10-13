// Package viewpoint prepara el mod Project Viewpoint (cámara en primera/tercera
// persona, Workshop 3809306528) para el arranque del juego.
//
// Viewpoint es un mod Java: solo corre si la JVM arranca con ZombieBuddy
// (https://github.com/zed-0xff/ZombieBuddy, Workshop 3619862853). Hacemos lo
// mismo que la "instalación manual" de ZombieBuddy en Windows, pero sin que el
// jugador toque nada:
//
//  1. Copiamos zbNative.dll y ZombieBuddy.jar desde la descarga de Workshop del
//     propio jugador (<item>/mods/ZombieBuddy/libs/) a la carpeta del juego.
//     Tienen que estar ahí: zbNative.dll carga ZombieBuddy.jar y
//     jre64\bin\instrument.dll con rutas relativas a la carpeta del juego.
//  2. internal/gameopts añade "-agentlib:zbNative" a los vmArgs del
//     ProjectZomboid64.json (la misma vía que usan los ajustes de rendimiento,
//     porque lanzamos con steam://rungameid y no podemos pasar argumentos).
//
// Al desactivarlo se quita esa línea y los dos ficheros quedan inertes (nada los
// carga). No tocamos las Opciones de lanzamiento de Steam: solo las leemos para
// no cargar el agente dos veces si el jugador ya lo puso ahí a mano.
//
// La primera vez (y cada vez que Viewpoint se actualiza) ZombieBuddy muestra su
// propio diálogo para aprobar el jar del mod. No lo salteamos.
package viewpoint

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"pzlauncher/internal/hashutil"
	"pzlauncher/internal/steam"
)

const (
	AppID                 = 108600
	ZombieBuddyWorkshopID = "3619862853"
	ViewpointWorkshopID   = "3809306528"

	dll = "zbNative.dll"
	jar = "ZombieBuddy.jar"
	// pendingJar lo deja el autoactualizador de ZombieBuddy cuando no pudo
	// reemplazar el jar bloqueado; zbNative.dll lo aplica en el próximo arranque,
	// así que uno viejo desharía la copia nueva que acabamos de poner.
	pendingJar = "ZombieBuddy.jar.new"
)

var agentFiles = []string{dll, jar}

// WorkshopURL es la página del ítem, para que el jugador se suscriba él mismo.
func WorkshopURL(itemID string) string {
	return "https://steamcommunity.com/sharedfiles/filedetails/?id=" + itemID
}

// libsIn devuelve <item>/mods/ZombieBuddy/libs si tiene los dos ficheros (el
// mismo sitio del que copia el instalador oficial de ZombieBuddy).
func libsIn(itemDir string) (string, bool) {
	libs := filepath.Join(itemDir, "mods", "ZombieBuddy", "libs")
	for _, f := range agentFiles {
		if !isFile(filepath.Join(libs, f)) {
			return "", false
		}
	}
	return libs, true
}

// ZombieBuddyLibs localiza los ficheros de ZombieBuddy en la descarga de Workshop.
func ZombieBuddyLibs() (string, bool) {
	item, ok := steam.WorkshopItemDir(AppID, ZombieBuddyWorkshopID)
	if !ok {
		return "", false
	}
	return libsIn(item)
}

// ViewpointSubscribed indica si Steam ya descargó Viewpoint.
func ViewpointSubscribed() bool {
	_, ok := steam.WorkshopItemDir(AppID, ViewpointWorkshopID)
	return ok
}

// JarVersion lee "Implementation-Version" del manifiesto del jar (ZombieBuddy
// la trae, p. ej. 2.3.4). Solo para mostrar; "" si no se pudo leer.
func JarVersion(path string) string {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return ""
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return ""
		}
		defer rc.Close()
		sc := bufio.NewScanner(rc)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "Implementation-Version:"); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

// AgentState resume cómo está ZombieBuddy en la carpeta del juego respecto de
// la copia de Workshop.
type AgentState string

const (
	NoSource          AgentState = "noSource"          // no está en el juego ni en Workshop
	NotInstalled      AgentState = "notInstalled"      // está en Workshop, falta copiarlo
	UpdateAvailable   AgentState = "updateAvailable"   // Steam bajó otra versión
	UpToDate          AgentState = "upToDate"          // igual que la de Workshop
	InstalledNoSource AgentState = "installedNoSource" // en el juego, pero sin Workshop para comparar
)

// StateOf compara los ficheros de gameDir con los de libs ("" = sin Workshop).
func StateOf(gameDir, libs string) AgentState {
	installed := true
	for _, f := range agentFiles {
		if !isFile(filepath.Join(gameDir, f)) {
			installed = false
		}
	}
	switch {
	case libs == "" && !installed:
		return NoSource
	case libs == "":
		return InstalledNoSource
	case !installed:
		return NotInstalled
	}
	for _, f := range agentFiles {
		if !sameFile(filepath.Join(libs, f), filepath.Join(gameDir, f)) {
			return UpdateAvailable
		}
	}
	return UpToDate
}

// Install copia los dos ficheros de libs a gameDir (a un temporal y luego
// rename, para no dejar nunca un dll/jar a medias que rompa el arranque).
func Install(gameDir, libs string) error {
	for _, f := range agentFiles {
		dest := filepath.Join(gameDir, f)
		tmp := dest + ".pzl-tmp"
		if err := copyFile(filepath.Join(libs, f), tmp); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("copiando %s a la carpeta del juego: %w", f, err)
		}
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("reemplazando %s (¿el juego está abierto?): %w", f, err)
		}
	}
	os.Remove(filepath.Join(gameDir, pendingJar))
	return nil
}

// ErrNotSubscribed se devuelve al intentar jugar con Viewpoint sin ZombieBuddy.
var ErrNotSubscribed = fmt.Errorf("Para usar Viewpoint suscribite a ZombieBuddy en Steam Workshop " +
	"y esperá a que Steam termine de descargarlo.")

// Ensure deja ZombieBuddy listo en gameDir antes de jugar: lo instala la
// primera vez y, si autoUpdate, lo actualiza cuando Steam bajó otra versión.
func Ensure(gameDir string, autoUpdate bool) error {
	libs, _ := ZombieBuddyLibs()
	switch StateOf(gameDir, libs) {
	case NoSource:
		return ErrNotSubscribed
	case NotInstalled:
		return Install(gameDir, libs)
	case UpdateAvailable:
		if autoUpdate {
			return Install(gameDir, libs)
		}
	}
	return nil
}

// UpdateNow copia la versión de Workshop ya mismo (botón "Actualizar ahora").
func UpdateNow(gameDir string) error {
	libs, ok := ZombieBuddyLibs()
	if !ok {
		return ErrNotSubscribed
	}
	return Install(gameDir, libs)
}

// SteamOptionsLoadAgent indica si alguna cuenta de Steam de esta PC ya carga
// ZombieBuddy desde las Opciones de lanzamiento (instalación manual u oficial).
// Solo PZ usa zbNative, así que basta con buscarlo en las líneas LaunchOptions.
func SteamOptionsLoadAgent() bool {
	for _, f := range steam.LocalConfigFiles() {
		data, err := os.ReadFile(f)
		if err == nil && launchOptionsLoadAgent(string(data)) {
			return true
		}
	}
	return false
}

func launchOptionsLoadAgent(localconfig string) bool {
	for _, line := range strings.Split(localconfig, "\n") {
		if strings.Contains(strings.ToLower(line), `"launchoptions"`) &&
			strings.Contains(strings.ToLower(line), "zbnative") {
			return true
		}
	}
	return false
}

// Status es lo que muestra el panel de Viewpoint.
type Status struct {
	Supported             bool       `json:"supported"` // solo Windows (zbNative.dll)
	Enabled               bool       `json:"enabled"`
	AutoUpdate            bool       `json:"autoUpdate"`
	GameFound             bool       `json:"gameFound"`
	ZombieBuddySubscribed bool       `json:"zombieBuddySubscribed"`
	ViewpointSubscribed   bool       `json:"viewpointSubscribed"`
	Agent                 AgentState `json:"agent"`
	InstalledVersion      string     `json:"installedVersion"`
	AvailableVersion      string     `json:"availableVersion"`
	// ZombieBuddy ya se carga siempre por fuera del launcher (Opciones de Steam
	// o un ProjectZomboid64.json que no editamos nosotros): no lo podemos apagar.
	InstalledExternally bool `json:"installedExternally"`
}

// GetStatus arma el estado. gameDir puede ser "" si no se encontró el juego.
func GetStatus(gameDir string, enabled, autoUpdate, jsonAgentNotOurs bool) Status {
	libs, subscribed := ZombieBuddyLibs()
	st := Status{
		Supported:             true,
		Enabled:               enabled,
		AutoUpdate:            autoUpdate,
		GameFound:             gameDir != "",
		ZombieBuddySubscribed: subscribed,
		ViewpointSubscribed:   ViewpointSubscribed(),
		Agent:                 NoSource,
		InstalledExternally:   jsonAgentNotOurs || SteamOptionsLoadAgent(),
	}
	if subscribed {
		st.AvailableVersion = JarVersion(filepath.Join(libs, jar))
	}
	if gameDir != "" {
		st.Agent = StateOf(gameDir, libs)
		st.InstalledVersion = JarVersion(filepath.Join(gameDir, jar))
	}
	return st
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func sameFile(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	if errA != nil || errB != nil || ia.Size() != ib.Size() {
		return false
	}
	ha, errA := hashutil.SHA256File(a)
	hb, errB := hashutil.SHA256File(b)
	return errA == nil && errB == nil && ha == hb
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
