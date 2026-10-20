import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

Panel {
  id: root
  moduleName: "peep"
  ipcTarget: "peep"

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  property var cfg: ({})
  property var loadedDefaults: ({})
  readonly property var defaults: loadedDefaults
  property string error: ""
  property bool busy: false
  property bool saving: false
  property bool writeReady: false
  property var pendingPatch: null
  property var settingsBaseline: null
  property var pendingConfig: null
  property var afterSave: null
  readonly property string cfgPath: (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/peep/config.json"
  readonly property string launcher: String(Qt.resolvedUrl("peep-launch.sh")).replace("file://", "")

  property FileView defaultsFile: FileView {
    path: Qt.resolvedUrl("server-defaults.json")
    printErrors: false
    onLoaded: {
      try {
        var value = JSON.parse(text())
        root.loadedDefaults = { apiUrl: String(value.apiUrl || ""), webUrl: String(value.webUrl || "") }
      } catch (e) { root.loadedDefaults = ({}) }
    }
  }

  // --- Updates: GitHub release check at startup and every 5 hours. ---
  property string currentVersion: ""
  property string latestVersion: ""
  property bool checkingUpdate: false
  property bool updating: false
  property string updateStatus: ""
  readonly property bool updateAvailable: newerVersion(latestVersion, currentVersion)

  property FileView manifestFile: FileView {
    path: Qt.resolvedUrl("manifest.json")
    printErrors: false
    onLoaded: {
      try { root.currentVersion = String(JSON.parse(text()).version || "") } catch (e) { root.currentVersion = "" }
    }
  }

  property Timer updateTimer: Timer {
    interval: 5 * 60 * 60 * 1000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.checkUpdate(false)
  }

  // Downloads the installer of that exact release and swaps the plugin in place.
  property Process updateProcess: Process {
    stderr: StdioCollector { id: updateErr }
    onExited: function(exitCode, exitStatus) {
      root.updating = false
      if (exitCode === 0 && exitStatus === 0) {
        root.updateStatus = "Reiniciando…"
        Quickshell.execDetached(["omarchy-restart-shell"])
      } else {
        var lines = updateErr.text.trim().split("\n")
        root.updateStatus = lines[lines.length - 1] || "Falha ao atualizar"
      }
    }
  }

  function newerVersion(a, b) {
    if (!/^\d+\.\d+\.\d+$/.test(a) || !/^\d+\.\d+\.\d+$/.test(b)) return false
    var x = a.split(".").map(Number), y = b.split(".").map(Number)
    for (var i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] > y[i]
    return false
  }

  function checkUpdate(manual) {
    if (checkingUpdate || updating) return
    checkingUpdate = true
    if (manual) updateStatus = ""
    var xhr = new XMLHttpRequest()
    xhr.onreadystatechange = function() {
      if (xhr.readyState !== XMLHttpRequest.DONE) return
      checkingUpdate = false
      try {
        if (xhr.status !== 200) throw new Error()
        var tag = String(JSON.parse(xhr.responseText).tag_name || "")
        if (!/^v\d+\.\d+\.\d+$/.test(tag)) throw new Error()
        latestVersion = tag.slice(1)
        if (manual) updateStatus = updateAvailable ? "" : "atualizado"
      } catch (e) {
        if (manual) updateStatus = "não foi possível verificar"
      }
    }
    xhr.open("GET", "https://api.github.com/repos/EaeDave/peep/releases/latest")
    xhr.setRequestHeader("Accept", "application/vnd.github+json")
    xhr.send()
  }

  function installUpdate() {
    if (updating || !updateAvailable) return
    updating = true
    updateStatus = ""
    var tag = "v" + latestVersion
    updateProcess.command = ["bash", "-c",
      'set -o pipefail; curl --fail --silent --show-error --location --proto "=https" --proto-redir "=https" "https://github.com/EaeDave/peep/releases/download/$1/install.sh" | bash -s -- --update --version "$1"',
      "peep-update", tag]
    updateProcess.running = true
  }

  property FileView cfgFile: FileView {
    path: root.cfgPath
    watchChanges: true
    atomicWrites: true
    printErrors: false
    onFileChanged: if (!root.saving) reload()
    onLoaded: root.configLoaded(text())
    onLoadFailed: {
      root.cfg = ({})
      root.failSave("Não foi possível ler a configuração. Abra a engrenagem para configurar o servidor.")
    }
    onSaved: {
      root.finishSave()
      Qt.callLater(function() { cfgFile.reload() })
    }
    onSaveFailed: {
      root.failSave("Não foi possível salvar. Verifique as permissões de " + root.cfgPath)
      Qt.callLater(function() { cfgFile.reload() })
    }
  }

  // Create only a missing config, privately. Never follow a config symlink.
  property Process prepareConfig: Process {
    command: ["bash", "-c", 'umask 077; dir=${1%/*}; [ ! -L "$dir" ] && mkdir -p -- "$dir" && [ ! -L "$1" ] || exit 1; if [ ! -e "$1" ]; then (set -C; printf "{}\\n" > "$1") || exit 1; fi; [ -f "$1" ] && [ -r "$1" ] && [ -w "$1" ]', "peep-config", root.cfgPath]
    onExited: function(exitCode, exitStatus) {
      if (exitCode !== 0 || exitStatus !== 0) {
        root.failSave("Não foi possível acessar a configuração. Verifique o arquivo e suas permissões.")
        return
      }
      root.writeReady = true
      cfgFile.reload()
    }
  }

  function failSave(message) {
    error = message
    pendingPatch = null
    pendingConfig = null
    afterSave = null
    saving = false
    writeReady = false
  }

  function finishSave() {
    cfg = pendingConfig
    var done = afterSave
    pendingPatch = null
    pendingConfig = null
    settingsBaseline = null
    afterSave = null
    saving = false
    writeReady = false
    error = ""
    if (done) done()
  }

  function configLoaded(raw) {
    var current
    try {
      current = JSON.parse(raw)
      if (!current || typeof current !== "object" || Array.isArray(current)) throw new Error()
    } catch (e) {
      cfg = ({})
      failSave("Configuração inválida. Corrija o JSON em " + cfgPath + " antes de salvar.")
      return
    }
    cfg = current
    if (!saving || !writeReady || !pendingPatch) return
    writeReady = false
    if (settingsBaseline) {
      var keys = ["apiUrl", "webUrl", "inviteSecret"]
      for (var i = 0; i < keys.length; i++) {
        if (current[keys[i]] !== settingsBaseline[keys[i]]) {
          failSave("O servidor foi alterado fora do Peep. Cancele e abra as configurações novamente.")
          return
        }
      }
    }
    // Merge only edited fields into the freshly loaded file (including unknown keys).
    pendingConfig = Object.assign({}, current, pendingPatch)
    pendingPatch = null
    var serialized = JSON.stringify(pendingConfig, null, 2) + "\n"
    // FileView does not emit saved() when the content is already identical.
    if (serialized === raw) { finishSave(); return }
    // Finish the load callback before starting the atomic write.
    Qt.callLater(function() { cfgFile.setText(serialized) })
  }

  function persist(patch, baseline, done) {
    if (saving) return
    error = ""
    pendingPatch = patch
    settingsBaseline = baseline
    afterSave = done
    saving = true
    writeReady = false
    prepareConfig.running = true
  }

  function saveSettings(apiUrl, webUrl, key) {
    persist({ apiUrl: apiUrl, webUrl: webUrl, inviteSecret: key }, settingsBaseline, function() {
      popup.settingsSaved()
    })
  }

  function openSettings() {
    settingsBaseline = Object.assign({}, cfg)
    error = ""
  }

  function configured() {
    if (!popup.validUrl(String(cfg.apiUrl || "")) || !popup.validUrl(String(cfg.webUrl || "")) || !String(cfg.inviteSecret || "").trim()) {
      error = "Configure o servidor na engrenagem antes de entrar ou criar uma sala."
      return false
    }
    return true
  }

  function request(method, path, body, onOk) {
    if (!configured()) return
    busy = true
    error = ""
    var xhr = new XMLHttpRequest()
    xhr.onreadystatechange = function() {
      if (xhr.readyState !== XMLHttpRequest.DONE) return
      busy = false
      if (xhr.status >= 200 && xhr.status < 300) onOk(xhr.responseText ? JSON.parse(xhr.responseText) : {})
      else if (xhr.status === 0) error = "Sem conexão com o servidor"
      else if (xhr.status === 401) error = "Chave de acesso ao servidor inválida. Revise na engrenagem."
      else if (xhr.status === 404) error = "Sala não existe"
      else if (xhr.status === 429) error = "Muitas tentativas, aguarde um minuto"
      else error = "Erro do servidor (" + xhr.status + ")"
    }
    xhr.open(method, String(cfg.apiUrl).replace(/\/+$/, "") + path)
    xhr.setRequestHeader("Authorization", "Bearer " + cfg.inviteSecret)
    if (body) xhr.setRequestHeader("Content-Type", "application/json")
    xhr.send(body ? JSON.stringify(body) : null)
  }

  function launch(code, name) {
    Quickshell.execDetached(["bash", launcher, code, name])
    close()
  }

  function withName(name, done) {
    if (!name) { error = "Informe seu nome"; return }
    if (!configured()) return
    if (cfg.name === name) done()
    else persist({ name: name }, null, done)
  }

  function join(code, name) {
    withName(name, function() {
      request("GET", "/rooms/" + encodeURIComponent(code), null, function() { launch(code, name) })
    })
  }

  function create(name) {
    withName(name, function() {
      request("POST", "/rooms", { name: name }, function(res) {
        Quickshell.execDetached(["wl-copy", res.code])
        launch(res.code, name)
      })
    })
  }

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: "\uf108"
    tooltipText: root.updateAvailable ? "Peep — versão " + root.latestVersion + " disponível" : "Peep"
    onPressed: root.toggle()
  }

  PeepPopup {
    id: popup
    anchorItem: button
    owner: root
    bar: root.bar
    open: root.opened
    name: root.cfg.name || ""
    apiUrl: String(root.cfg.apiUrl || "")
    webUrl: String(root.cfg.webUrl || "")
    accessKey: String(root.cfg.inviteSecret || "")
    error: root.error
    busy: root.busy || root.saving
    saving: root.saving
    defaults: root.defaults
    currentVersion: root.currentVersion
    latestVersion: root.updateAvailable ? root.latestVersion : ""
    checkingUpdate: root.checkingUpdate
    updating: root.updating
    updateStatus: root.updateStatus
    onCheckUpdateRequested: root.checkUpdate(true)
    onInstallUpdateRequested: root.installUpdate()
    onSettingsOpened: root.openSettings()
    onSettingsSaveRequested: function(api, web, key) { root.saveSettings(api, web, key) }
    onJoinRequested: function(code, name) { root.join(code, name) }
    onCreateRequested: function(name) { root.create(name) }
    onDismissRequested: root.close()
  }
}
