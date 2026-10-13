# pzlauncher + Viewpoint

Es el launcher del servidor **Patagonia PZ**
([original](https://github.com/patagoniapz/pz-launcher)) con un agregado: una
opción para jugar con **[Project Viewpoint](https://steamcommunity.com/sharedfiles/filedetails/?id=3809306528)**,
el mod de cámara en primera/tercera persona. Todo lo demás funciona igual que el
launcher original.

## Cómo usarlo

1. En Steam Workshop suscribite a
   **[ZombieBuddy](https://steamcommunity.com/sharedfiles/filedetails/?id=3619862853)**
   y a **[Viewpoint](https://steamcommunity.com/sharedfiles/filedetails/?id=3809306528)**,
   y esperá a que Steam los descargue. No hace falta instalar nada más.
2. Bajá `pzlauncher-windows-amd64-gui.exe` desde
   [Releases](../../releases/latest) y abrilo.
3. Entrá a **Mod Viewpoint**, tildá **Activar mod Viewpoint**, **Guardar** y
   **Jugar**.
4. La primera vez, ZombieBuddy te pide aprobar el mod en una ventana: decí que
   **sí**. Si arriba a la izquierda del menú dice **"ZombieBuddy … loaded"**, está
   funcionando. Dentro del juego, la tecla **Supr** abre los ajustes de Viewpoint.

Para dejar de usarlo, destildá la opción y dale **Guardar**: el juego vuelve a
arrancar como siempre. Para sacar ZombieBuddy del todo (aunque lo hayas instalado
con su instalador o a mano), usá **Desinstalar ZombieBuddy** en el mismo panel. El launcher nunca toca tus partidas ni tus opciones de
Steam.

## Cómo comprobar que el archivo es el original

Un `.exe` puede ser modificado por cualquiera que lo vuelva a subir a otro lado.
Estas dos pruebas te aseguran que el tuyo es exactamente el que compiló GitHub a
partir del código de este repositorio.

**1. Compará la huella (lo más fácil).** Cada archivo tiene una "huella" (hash
SHA-256): un código de 64 letras y números que cambia por completo si se toca un
solo byte del archivo.

- Abrí la carpeta donde descargaste el `.exe`, hacé clic derecho en un espacio
  vacío y elegí **Abrir en Terminal** (o "Abrir ventana de PowerShell aquí").
- Escribí esto y apretá Enter:

  ```powershell
  Get-FileHash .\pzlauncher-windows-amd64-gui.exe
  ```

- Compará lo que sale en **Hash** con el que figura en la página del
  [release](../../releases/latest) (y en su archivo `SHA256SUMS.txt`). Tienen que
  ser **idénticos**; que estén en mayúsculas o minúsculas da igual. Si difiere en
  algo, **no lo abras**.

**2. Verificá que lo compiló GitHub (opcional).** Cada release lleva una
atestación firmada por GitHub que dice de qué repositorio y de qué commit salió
el archivo. Se ve en [Attestations](../../attestations), o con
[GitHub CLI](https://cli.github.com/):

```powershell
gh attestation verify .\pzlauncher-windows-amd64-gui.exe --repo 0xSuku/pz-launcher-viewpoint
```

Si dice `Verification succeeded`, el archivo salió de este repositorio, compilado
por GitHub y no por una persona.

Windows puede mostrar "Windows protegió su PC" porque el `.exe` no está firmado
con un certificado comercial. Si la huella coincide, es seguro: **Más
información → Ejecutar de todas formas**.

## Para publicar una versión nueva

```powershell
powershell -ExecutionPolicy Bypass -File .\release.ps1
```

Crea el tag siguiente (sube el último número: `v0.2.12`, `v0.2.13`, …) y GitHub Actions compila,
calcula las huellas, firma la atestación y publica el release solo. Para traer
cambios del launcher original: `git pull upstream main` antes de publicar.

---

> 🇬🇧 English below · 🇪🇸 Español más abajo (documentación del launcher original)

---

## 🇬🇧 English

Cross-platform **launcher / updater for Project Zomboid**. A single executable
you can run from **anywhere** — it **auto-detects your Steam install** of the
game (registry + `libraryfolders.vdf`, so it also finds installs on other
drives). On launch it:

1. Reads a `manifest.json` from GitHub Releases.
2. Self-updates if a newer launcher version is published.
3. Compares each managed file (e.g. `projectzomboid.jar`) by **SHA-256** and
   downloads only what changed.
4. Starts the game.

Progress is shown in a small **native window** with an **estimated time left**
(falls back to console if no GUI is available). On Windows it carries the
Project Zomboid icon.

It does **not** auto-launch: after updating it asks **Play / Performance
settings / Clean logs / Quit**. The game is started **through Steam**
(`steam://rungameid/108600`), which boots Steam if it is closed. "Clean logs"
wipes `%UserProfile%\Zomboid\Logs\` and the `*console.txt` files (never your
saves or mods).

**Performance settings** let players tune a few optional things without editing
files by hand; the launcher writes them into the game's `ProjectZomboid64.json`
(re-applied every launch, so a Steam update or "Verify integrity" can't lose
them):

- **Movement stutter fix** (`-Dpz.chunkBudgetMs`): spreads map-chunk loading over
  several frames to smooth the micro-stutters when you move or enter new areas.
  Lower = smoother but terrain pops in slightly later. Recommended 4-8; `0` (or
  blank) = disabled (vanilla).
- **Memory** (`-Xmx`): the launcher detects your total RAM and suggests a heap
  size you can accept or change. `0` (or blank) leaves the game's default.
- **Tow fix** (`-Dpz.safeTowReconnect`): stops a car towing a trailer/another car
  from snapping back together after a desync and crashing. **On by default**
  (recommended); turn it off only to get vanilla behavior for a test. When left
  on, no flag is written (the jar's built-in default is on).

**Viewpoint mod** (Windows): an optional menu entry to enable
[Project Viewpoint](https://steamcommunity.com/sharedfiles/filedetails/?id=3809306528)
(first/third person camera). Viewpoint is a Java mod that needs the
[ZombieBuddy](https://github.com/zed-0xff/ZombieBuddy) loader
([Workshop](https://steamcommunity.com/sharedfiles/filedetails/?id=3619862853)).
Subscribe to both; there is no need to run ZombieBuddy's installer. When enabled,
on Play the launcher copies `zbNative.dll` + `ZombieBuddy.jar` from your Workshop
download into the game folder (updating them when Steam gets a new version,
unless you turn that off) adds `-agentlib:zbNative` to `ProjectZomboid64.json`, and enables ZombieBuddy +
Viewpoint in the main-menu mod list (`Zomboid\mods\default.txt`). That is enough
for multiplayer even if the server doesn't list them: ZombieBuddy applies Java
mods in memory at startup and keeps them until the game closes, and Viewpoint's
camera is all Java. Turning it off undoes only what the launcher added. Your Steam
Launch Options are never touched. ZombieBuddy asks you once to approve the
Viewpoint jar (and again whenever Viewpoint updates).

Downloads are resilient to flaky connections: a stalled transfer is detected and
automatically retried.

The **game install** folder is auto-detected via Steam; the **Zomboid data**
folder (logs/saves) is autodetected at `%UserProfile%\Zomboid` (`~/Zomboid`).
Both can be overridden with env vars (`PZL_GAME_DIR`, `PZL_ZOMBOID_DIR`) or a
`pzlauncher.json` next to the executable:
`{ "game_dir": "D:\\...\\ProjectZomboid", "zomboid_dir": "D:\\...\\Zomboid" }`.
Run `pzlauncher --detect` to print the detected game folder.

### Get it

A prebuilt binary is provided in the [Releases](../../releases) section for
Windows, Linux and macOS — just download and drop it in the game folder.

If you prefer, you can **download the source and build it yourself**:

```bash
# requires Go 1.25+
git clone https://github.com/patagoniapz/pz-launcher.git
cd pz-launcher
go build -o pzlauncher ./cmd/launcher     # add .exe on Windows
```

Optionally pin version and manifest URL:

```bash
go build -ldflags "-X main.version=1.0.0 -X main.manifestURL=https://github.com/patagoniapz/pz-launcher/releases/latest/download/manifest.json" -o pzlauncher ./cmd/launcher
```

---

## 🇪🇸 Español

**Launcher / actualizador multiplataforma para Project Zomboid**. Un único
ejecutable que puedes ejecutar desde **cualquier sitio** — **autodetecta tu
instalación de Steam** del juego (registro + `libraryfolders.vdf`, así encuentra
también instalaciones en otros discos). Al arrancar:

1. Lee un `manifest.json` desde GitHub Releases.
2. Se autoactualiza si hay una versión más nueva del launcher.
3. Compara cada fichero gestionado (p. ej. `projectzomboid.jar`) por **SHA-256**
   y descarga solo lo que haya cambiado.
4. Arranca el juego.

El progreso se muestra en una pequeña **ventana nativa** con **tiempo estimado
restante** (si no hay entorno gráfico, cae a consola). En Windows lleva el icono
de Project Zomboid.

**No** arranca el juego solo: tras actualizar pregunta **Jugar / Ajustes de
rendimiento / Limpiar logs / Salir**. El juego se lanza **vía Steam**
(`steam://rungameid/108600`), que abre Steam si está cerrado. "Limpiar logs"
borra `%UserProfile%\Zomboid\Logs\` y los `*console.txt` (nunca tus partidas ni
mods).

**Ajustes de rendimiento**: permiten al jugador configurar algunas cosas opcionales
sin editar ficheros a mano; el launcher las escribe en el `ProjectZomboid64.json` del juego
(reaplicadas en cada arranque, así un update de Steam o "Verificar integridad" no
las pierde):

- **Fix de tirones al moverse** (`-Dpz.chunkBudgetMs`): reparte la carga del mapa
  en varios frames para suavizar los tironcitos al desplazarte o entrar a zonas
  nuevas. Más bajo = más fluido pero el terreno aparece un pelín más tarde.
  Recomendado 4-8; `0` (o vacío) = desactivado (vanilla).
- **Memoria** (`-Xmx`): el launcher detecta tu RAM total y sugiere un tamaño de
  heap que puedes aceptar o cambiar. `0` (o vacío) deja el valor por defecto del
  juego.
- **Fix de remolque** (`-Dpz.safeTowReconnect`): evita que un auto que remolca un
  trailer/otro auto se re-enganche de golpe tras una desincronización y provoque
  un choque. **Activado por defecto** (recomendado); desactívalo solo para probar
  el comportamiento vanilla. Si se deja activado no se escribe ningún flag (el jar
  ya viene con el fix activado).

**Mod Viewpoint** (Windows): una opción del menú para activar
[Project Viewpoint](https://steamcommunity.com/sharedfiles/filedetails/?id=3809306528)
(cámara en primera/tercera persona). Viewpoint es un mod Java que necesita el
loader [ZombieBuddy](https://github.com/zed-0xff/ZombieBuddy)
([Workshop](https://steamcommunity.com/sharedfiles/filedetails/?id=3619862853)).
Hay que suscribirse a los dos; no hace falta correr el instalador de ZombieBuddy.
Activado, al Jugar el launcher copia `zbNative.dll` + `ZombieBuddy.jar` desde tu
descarga de Workshop a la carpeta del juego (y los actualiza cuando Steam baja una
versión nueva, salvo que lo desactives) y añade `-agentlib:zbNative` al
`ProjectZomboid64.json`, y activa ZombieBuddy + Viewpoint en la lista de mods del
menú principal (`Zomboid\mods\default.txt`). Eso alcanza para multijugador aunque
el servidor no los tenga en su lista: ZombieBuddy aplica los mods Java en memoria
al arrancar y no los deshace hasta cerrar el juego, y la cámara de Viewpoint es
toda Java. Al desactivarlo se deshace solo lo que agregó el launcher. Nunca toca tus Opciones de lanzamiento de Steam. ZombieBuddy pide una
vez aprobar el jar de Viewpoint (y de nuevo cuando Viewpoint se actualiza).

Las descargas toleran conexiones inestables: si la transferencia se queda parada,
se detecta y se reintenta automáticamente.

La carpeta de **instalación** del juego se autodetecta vía Steam; la carpeta de
**datos** de Zomboid (logs/partidas) se autodetecta en `%UserProfile%\Zomboid`
(`~/Zomboid`). Ambas se pueden cambiar con variables (`PZL_GAME_DIR`,
`PZL_ZOMBOID_DIR`) o con un `pzlauncher.json` junto al ejecutable:
`{ "game_dir": "D:\\...\\ProjectZomboid", "zomboid_dir": "D:\\...\\Zomboid" }`.
Ejecuta `pzlauncher --detect` para ver la carpeta del juego detectada.

### Cómo obtenerlo

Se ofrece un binario ya compilado en la sección de [Releases](../../releases)
para Windows, Linux y macOS — basta con descargarlo y dejarlo en la carpeta del
juego.

Si lo prefieres, puedes **descargar el código fuente y compilarlo por tus
propios medios**:

```bash
# requiere Go 1.22+
git clone https://github.com/patagoniapz/pz-launcher.git
cd pz-launcher
go build -o pzlauncher ./cmd/launcher     # añade .exe en Windows
```

Opcionalmente, fija la versión y la URL del manifiesto:

```bash
go build -ldflags "-X main.version=1.0.0 -X main.manifestURL=https://github.com/patagoniapz/pz-launcher/releases/latest/download/manifest.json" -o pzlauncher ./cmd/launcher
```
