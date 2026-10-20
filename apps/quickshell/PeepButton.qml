import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

// LNX-1/6: bar icon + popup; IPC target "peep" (omarchy-shell peep toggle).
Panel {
  id: root
  moduleName: "peep"
  ipcTarget: "peep"

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  // LNX-9: { "apiUrl", "webUrl", "inviteSecret", "name"? }
  property var cfg: ({})
  property string error: ""
  property bool busy: false
  readonly property string cfgPath: (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/peep/config.json"
  readonly property string launcher: String(Qt.resolvedUrl("peep-launch.sh")).replace("file://", "")

  property FileView cfgFile: FileView {
    path: root.cfgPath
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: {
      try { root.cfg = JSON.parse(text()) } catch (e) { root.cfg = ({}) }
    }
    onLoadFailed: root.cfg = ({})
  }

  function saveName(name) {
    if (!name || cfg.name === name) return
    var next = Object.assign({}, cfg, { name: name })
    cfgFile.setText(JSON.stringify(next, null, 2) + "\n")
  }

  // Maps API failures to the same messages as the web client (WEB-5).
  function request(method, path, body, onOk) {
    if (!cfg.apiUrl || !cfg.inviteSecret) {
      error = "Configure " + cfgPath
      return
    }
    busy = true
    error = ""
    var xhr = new XMLHttpRequest()
    xhr.onreadystatechange = function() {
      if (xhr.readyState !== XMLHttpRequest.DONE) return
      busy = false
      if (xhr.status >= 200 && xhr.status < 300) onOk(xhr.responseText ? JSON.parse(xhr.responseText) : {})
      else if (xhr.status === 0) error = "Sem conexão com o servidor"
      else if (xhr.status === 401) error = "Segredo inválido"
      else if (xhr.status === 404) error = "Sala não existe"
      else if (xhr.status === 429) error = "Muitas tentativas, aguarde um minuto"
      else error = "Erro do servidor (" + xhr.status + ")"
    }
    xhr.open(method, String(cfg.apiUrl).replace(/\/+$/, "") + path)
    xhr.setRequestHeader("Authorization", "Bearer " + cfg.inviteSecret)
    if (body) xhr.setRequestHeader("Content-Type", "application/json")
    xhr.send(body ? JSON.stringify(body) : null)
  }

  // LNX-4: Chromium --app via peep-launch.sh.
  function launch(code, name) {
    Quickshell.execDetached(["bash", launcher, code, name])
    close()
  }

  function join(code, name) {
    if (!name) { error = "Informe seu nome"; return }
    saveName(name)
    request("GET", "/rooms/" + encodeURIComponent(code), null, function() { launch(code, name) })
  }

  // LNX-3: create, copy code with wl-copy, then join.
  function create(name) {
    if (!name) { error = "Informe seu nome"; return }
    saveName(name)
    request("POST", "/rooms", { name: name }, function(res) {
      Quickshell.execDetached(["wl-copy", res.code])
      launch(res.code, name)
    })
  }

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: "\uf108"
    tooltipText: "Peep"
    onPressed: root.toggle()
  }

  PeepPopup {
    anchorItem: button
    owner: root
    bar: root.bar
    open: root.opened
    name: root.cfg.name || ""
    error: root.error
    busy: root.busy
    onJoinRequested: function(code, name) { root.join(code, name) }
    onCreateRequested: function(name) { root.create(name) }
    onDismissRequested: root.close()
  }
}
