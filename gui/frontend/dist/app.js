// Puente con el backend Go (Wails). Wails inyecta:
//   - window.go.main.App.*   -> métodos ligados (devuelven Promises)
//   - window.runtime.*       -> eventos, diálogos, etc.
// No usamos los wrappers generados (wailsjs/) para que el frontend sea estático
// y no necesite paso de build con npm.

const App = () => window.go.main.App;

function $(id) { return document.getElementById(id); }

function show(view) {
  for (const v of ["view-updating", "view-error", "view-menu"]) {
    $(v).classList.toggle("hidden", v !== view);
  }
  // Al cambiar de vista, cierra los paneles.
  $("panel-notes").classList.add("hidden");
  $("panel-settings").classList.add("hidden");
  $("panel-viewpoint").classList.add("hidden");
}

function panel(id) {
  $("view-menu").classList.add("hidden");
  $("panel-notes").classList.toggle("hidden", id !== "panel-notes");
  $("panel-settings").classList.toggle("hidden", id !== "panel-settings");
  $("panel-viewpoint").classList.toggle("hidden", id !== "panel-viewpoint");
}

function backToMenu() {
  $("panel-notes").classList.add("hidden");
  $("panel-settings").classList.add("hidden");
  $("panel-viewpoint").classList.add("hidden");
  $("view-menu").classList.remove("hidden");
}

let toastTimer = null;
function toast(msg, kind, ms) {
  const t = $("toast");
  t.textContent = msg;
  t.className = "toast" + (kind ? " " + kind : "");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.add("hidden"), ms || 4200);
}

// --- Render del estado ---

function renderState(st) {
  $("version").textContent = st.version || "";
  if (st.error) {
    $("error-msg").textContent = st.error;
    show("view-error");
    return;
  }
  if (!st.ready) return; // seguimos actualizando

  $("offline-banner").classList.toggle("hidden", !st.offline);
  $("subtitle").textContent = st.offline ? "Sin conexión" : "Listo para jugar";

  const ln = $("last-note");
  if (st.lastNote) {
    ln.innerHTML = "Última novedad: <b></b>";
    ln.querySelector("b").textContent = st.lastNote;
    ln.classList.remove("hidden");
  } else {
    ln.classList.add("hidden");
  }

  const notes = st.patchNotes || [];
  $("btn-notes").classList.toggle("hidden", notes.length === 0);
  renderNotes(notes);

  show("view-menu");
  refreshViewpointButton();
}

function renderNotes(notes) {
  const list = $("notes-list");
  list.innerHTML = "";
  for (const n of notes) {
    const card = document.createElement("div");
    card.className = "note";
    const top = document.createElement("div");
    top.className = "note-top";
    if (n.id) {
      const id = document.createElement("span");
      id.className = "note-id";
      id.textContent = n.id;
      top.appendChild(id);
    }
    const title = document.createElement("span");
    title.className = "note-title";
    title.textContent = n.title || "";
    top.appendChild(title);
    if (n.date) {
      const date = document.createElement("span");
      date.className = "note-date";
      date.textContent = n.date;
      top.appendChild(date);
    }
    card.appendChild(top);
    if (n.desc) {
      const desc = document.createElement("div");
      desc.className = "note-desc";
      desc.textContent = n.desc;
      card.appendChild(desc);
    }
    list.appendChild(card);
  }
}

// --- Ajustes ---

async function openSettings() {
  try {
    const s = await App().GetSettings();
    $("f-chunk").value = s.chunkBudgetMs || 0;
    $("f-heap").value = s.defaultHeapMB || 0;
    $("f-grid").value = s.maxChunkGridWidth || 0;
    $("f-tow").checked = !!s.safeTowOff;
    const ramLine = s.ramGB > 0 ? `Tu PC tiene ${s.ramGB} GB de RAM.` : "No pude detectar tu RAM.";
    const sug = s.suggestHeapMB > 0 ? ` Sugerido: ${s.suggestHeapMB} MB.` : "";
    $("ram-help").textContent = `${ramLine} PZ usa 3072 MB por defecto.${sug} 0 = no tocar.`;
    if (!s.hasGameJSON) {
      toast("Ojo: no encontré ProjectZomboid64.json; los ajustes no se podrán aplicar.", "err");
    }
    panel("panel-settings");
  } catch (e) {
    toast(String(e), "err");
  }
}

async function saveSettings() {
  const chunk = parseInt($("f-chunk").value || "0", 10);
  const heap = parseInt($("f-heap").value || "0", 10);
  const grid = parseInt($("f-grid").value || "0", 10);
  const towOff = $("f-tow").checked;
  try {
    const summary = await App().SaveSettings(chunk, heap, grid, towOff);
    toast(summary, "ok");
    backToMenu();
  } catch (e) {
    toast(String(e), "err");
  }
}

// --- Viewpoint (ver internal/viewpoint) ---

// line arma una línea de estado con nodos DOM (las versiones salen de ficheros
// en disco, así que nada de innerHTML). parts: textos o [texto, clase|"b"].
function line(parts) {
  const div = document.createElement("div");
  for (const p of parts) {
    if (!Array.isArray(p)) { div.appendChild(document.createTextNode(p)); continue; }
    const el = document.createElement(p[1] === "b" ? "b" : "span");
    el.textContent = p[0];
    if (p[1] !== "b") el.className = p[1];
    div.appendChild(el);
  }
  return div;
}

const ver = (v) => (v ? "v" + v : "?");

function renderViewpoint(s) {
  $("vp-enabled").checked = !!s.enabled;
  $("vp-auto").checked = !!s.autoUpdate;
  const box = $("vp-status");
  box.innerHTML = "";
  let canUpdate = false;

  if (s.installedExternally) {
    box.appendChild(line([["ZombieBuddy ya se carga siempre por fuera del launcher (sus opciones de Steam o su instalador oficial). Funciona igual, pero desmarcar esto no lo apaga.", "warn"]]));
  } else {
    switch (s.agent) {
      case "noSource":
        box.appendChild(line([["Falta ZombieBuddy: suscribite en Workshop y esperá a que Steam lo baje.", "bad"]]));
        break;
      case "notInstalled":
        box.appendChild(line(["ZombieBuddy ", [ver(s.availableVersion), "b"], " se instala al jugar."]));
        canUpdate = true;
        break;
      case "updateAvailable":
        box.appendChild(line([["Hay versión nueva de ZombieBuddy: ", "warn"], [ver(s.installedVersion), "b"], " → ", [ver(s.availableVersion), "b"],
          s.autoUpdate ? " (se instala al jugar)" : ""]));
        canUpdate = true;
        break;
      case "upToDate":
        box.appendChild(line([["✓ ", "ok"], "ZombieBuddy ", [ver(s.installedVersion), "b"], " al día."]));
        break;
      case "installedNoSource":
        box.appendChild(line(["ZombieBuddy ", [ver(s.installedVersion), "b"], [" instalado, pero no está en tu Workshop para buscar versiones nuevas.", "warn"]]));
        break;
    }
  }
  box.appendChild(s.viewpointSubscribed
    ? line([["✓ ", "ok"], "Viewpoint descargado de Workshop."])
    : line([["Viewpoint no está descargado: suscribite en Workshop.", "bad"]]));
  if (s.zombieBuddySubscribed && s.viewpointSubscribed) {
    box.appendChild(s.modsActive
      ? line([["✓ ", "ok"], "ZombieBuddy y Viewpoint activos en el menú de mods."])
      : line(["ZombieBuddy y Viewpoint se activan en el menú de mods al jugar."]));
  }

  if (s.steamOptionsAgent) {
    box.appendChild(line([["ZombieBuddy está en tus Opciones de lanzamiento de Steam: eso solo se quita desde Steam (Propiedades → Opciones de lanzamiento).", "warn"]]));
  }
  $("vp-danger").classList.toggle("hidden", !s.zombieBuddyPresent && !s.steamOptionsAgent);

  $("btn-vp-update").textContent = s.agent === "notInstalled" ? "Instalar ahora" : "Actualizar ahora";
  $("btn-vp-update").classList.toggle("hidden", !canUpdate || !s.gameFound || s.installedExternally);
  $("btn-vp-sub-zb").classList.toggle("hidden", !!s.zombieBuddySubscribed);
  $("btn-vp-sub-vp").classList.toggle("hidden", !!s.viewpointSubscribed);
}

async function refreshViewpointButton() {
  try {
    const s = await App().GetViewpoint();
    $("btn-viewpoint").classList.toggle("hidden", !s.supported);
    $("vp-pill").classList.toggle("hidden", !s.enabled);
  } catch (e) { /* opcional: si falla, simplemente no se muestra */ }
}

async function openViewpoint() {
  try {
    renderViewpoint(await App().GetViewpoint());
    panel("panel-viewpoint");
  } catch (e) {
    toast(String(e), "err");
  }
}

async function saveViewpoint() {
  try {
    const s = await App().SaveViewpoint($("vp-enabled").checked, $("vp-auto").checked);
    renderViewpoint(s);
    toast(s.enabled ? "Viewpoint activado: se aplica al Jugar." : "Viewpoint desactivado.", "ok");
    refreshViewpointButton();
    backToMenu();
  } catch (e) {
    toast(String(e), "err");
  }
}

async function uninstallZombieBuddy() {
  try {
    const r = await App().UninstallZombieBuddy();
    renderViewpoint(r.status);
    refreshViewpointButton();
    if (r.message) toast(r.message, "ok", 12000); // vacío = el jugador canceló
  } catch (e) {
    toast(String(e), "err", 8000);
  }
}

async function updateZombieBuddy() {
  try {
    renderViewpoint(await App().UpdateZombieBuddy());
    toast("ZombieBuddy actualizado.", "ok");
  } catch (e) {
    toast(String(e), "err");
  }
}

// --- Acciones del menú ---

async function handleAction(act) {
  switch (act) {
    case "play":
      try { await App().Play(); } catch (e) { toast(String(e), "err"); }
      break;
    case "quit":
      App().Quit();
      break;
    case "settings":
      openSettings();
      break;
    case "notes":
      panel("panel-notes");
      break;
    case "clean":
      try { toast(await App().CleanLogs(), "ok"); } catch (e) { toast(String(e), "err"); }
      break;
    case "openlogs":
      try { await App().OpenLogs(); } catch (e) { toast(String(e), "err"); }
      break;
    case "save-settings":
      saveSettings();
      break;
    case "viewpoint":
      openViewpoint();
      break;
    case "vp-save":
      saveViewpoint();
      break;
    case "vp-update":
      updateZombieBuddy();
      break;
    case "vp-uninstall":
      uninstallZombieBuddy();
      break;
    case "vp-sub-zb":
      App().OpenWorkshop("zombiebuddy");
      break;
    case "vp-sub-vp":
      App().OpenWorkshop("viewpoint");
      break;
    case "back":
      backToMenu();
      break;
  }
}

// --- Arranque ---

function init() {
  document.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-act]");
    if (btn) handleAction(btn.dataset.act);
  });

  const rt = window.runtime;
  rt.EventsOn("stage", (text) => { $("stage").textContent = text; });
  rt.EventsOn("progress", (p) => {
    const pct = (p && p.pct) || 0;
    $("bar-fill").style.width = pct + "%";
    $("pct").textContent = pct > 0 ? pct + "%" : "";
  });
  rt.EventsOn("success", (text) => {
    $("bar-fill").style.width = "100%";
    $("stage").textContent = text || "Listo";
  });
  rt.EventsOn("ready", (st) => renderState(st));
  rt.EventsOn("error", (msg) => {
    $("error-msg").textContent = msg;
    show("view-error");
  });

  // Arranca el flujo y recupera el estado por si ya estábamos listos.
  App().Start();
  App().GetState().then(renderState).catch(() => {});
}

// Espera a que Wails inyecte runtime y bindings antes de arrancar.
function waitForWails() {
  if (window.runtime && window.go && window.go.main && window.go.main.App) {
    init();
  } else {
    setTimeout(waitForWails, 30);
  }
}

document.addEventListener("DOMContentLoaded", waitForWails);
