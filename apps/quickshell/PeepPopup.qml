import QtQuick
import qs.Commons
import qs.Commons as Commons
import qs.Ui

KeyboardPanel {
  id: popup

  property string name: ""
  property string apiUrl: ""
  property string webUrl: ""
  property string accessKey: ""
  property var defaults: ({})
  property string error: ""
  property string validationError: ""
  property string notice: ""
  property bool busy: false
  property bool saving: false
  property bool editingSettings: false
  signal joinRequested(string code, string name)
  signal createRequested(string name)
  signal settingsOpened()
  signal settingsSaveRequested(string apiUrl, string webUrl, string key)
  signal dismissRequested()

  readonly property string alphabet: "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

  function mask(raw) {
    var s = ""
    var up = String(raw).toUpperCase()
    for (var i = 0; i < up.length && s.length < 6; i++)
      if (alphabet.indexOf(up[i]) >= 0) s += up[i]
    return s.length > 3 ? s.slice(0, 3) + "-" + s.slice(3) : s
  }

  function validUrl(value) {
    // Explicit authority grammar rejects credentials, malformed ports and bare schemes.
    var match = /^(https?):\/\/(\[[0-9a-f:.]+\]|[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?)(?::([0-9]{1,5}))?(?:\/[^\s?#]*)?\/?$/i.exec(value)
    if (!match || (match[3] && (Number(match[3]) < 1 || Number(match[3]) > 65535))) return false
    var host = match[2].toLowerCase()
    if (host[0] === "[") {
      var address = host.slice(1, -1)
      if (address.indexOf(".") !== -1) {
        var tail = address.slice(address.lastIndexOf(":") + 1)
        var octets = tail.split(".")
        if (octets.length !== 4 || octets.some(function(part) { return !/^(0|[1-9][0-9]{0,2})$/.test(part) || Number(part) > 255 })) return false
        address = address.slice(0, address.lastIndexOf(":") + 1) + "0:0"
      }
      if (/:::/.test(address) || (address.indexOf("::") === -1 && (address[0] === ":" || address[address.length - 1] === ":"))) return false
      var halves = address.split("::")
      if (halves.length > 2) return false
      var groups = address.split(":").filter(function(group) { return group !== "" })
      if (groups.some(function(group) { return !/^[0-9a-f]{1,4}$/.test(group) })) return false
      if (halves.length === 2 ? groups.length >= 8 : groups.length !== 8) return false
    } else {
      if (host.length > 253 || host.split(".").some(function(label) {
        return !/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label)
      })) return false
      if (/^[0-9.]+$/.test(host) && (host.split(".").length !== 4 || host.split(".").some(function(part) { return Number(part) > 255 }))) return false
    }
    return match[1].toLowerCase() === "https" || host === "localhost" || host === "127.0.0.1" || host === "[::1]"
  }

  function openSettings() {
    if (busy) return
    settingsOpened()
    apiField.text = apiUrl || defaults.apiUrl || ""
    webField.text = webUrl || defaults.webUrl || ""
    keyField.text = accessKey
    keyField.password = true
    validationError = ""
    notice = ""
    editingSettings = true
    Qt.callLater(function() { apiField.forceActiveFocus() })
  }

  function cancelSettings() {
    if (saving) return
    apiField.text = ""
    webField.text = ""
    keyField.text = ""
    validationError = ""
    editingSettings = false
    codeField.forceActiveFocus()
  }

  function settingsSaved() {
    cancelSettings()
    notice = "Configurações salvas"
  }

  function saveSettings() {
    if (saving) return
    var api = apiField.text.trim().replace(/\/+$/, "")
    var web = webField.text.trim().replace(/\/+$/, "")
    if (!validUrl(api) || !validUrl(web)) {
      validationError = "Informe URLs válidos com HTTPS. HTTP é permitido apenas em localhost, 127.0.0.1 ou [::1]."
      return
    }
    if (!keyField.text.trim()) {
      validationError = "Informe a chave de acesso ao servidor."
      keyField.forceActiveFocus()
      return
    }
    validationError = ""
    settingsSaveRequested(api, web, keyField.text.trim())
  }

  function submitJoin() {
    if (!busy && codeField.text.length === 7) joinRequested(codeField.text, nameField.text.trim())
  }

  onOpenChanged: {
    if (open) {
      editingSettings = false
      nameField.text = name
      codeField.text = ""
      notice = ""
    } else if (!saving) {
      cancelSettings()
    }
  }

  focusTarget: editingSettings ? apiField : (name === "" ? nameField : codeField)
  contentWidth: fittedContentWidth(Style.space(300))
  contentHeight: fittedContentHeight(header.height + Style.space(16) + form.implicitHeight, Style.space(440))

  component Label: Text {
    color: Commons.Color.foreground
    font.family: Style.font.family
    font.pixelSize: Style.font.body
    textFormat: Text.PlainText
  }

  Column {
    anchors.fill: parent
    spacing: Style.space(16)
    Keys.onEscapePressed: {
      if (popup.editingSettings) popup.cancelSettings()
      else popup.dismissRequested()
    }

    Row {
      id: header
      width: parent.width
      spacing: Style.space(8)
      Label {
        width: parent.width - gear.width - parent.spacing
        anchors.verticalCenter: parent.verticalCenter
        text: popup.editingSettings ? "Servidor" : "Peep"
        font.pixelSize: Style.font.body * 1.3
        font.bold: true
      }
      Button {
        id: gear
        iconText: "\uf013"
        tooltipText: "Configurações do servidor"
        Accessible.name: "Configurações do servidor"
        focusable: true
        selected: popup.editingSettings
        enabled: !popup.busy
        onClicked: popup.editingSettings ? popup.cancelSettings() : popup.openSettings()
      }
    }

    Flickable {
      id: scroll
      width: parent.width
      height: Math.max(0, parent.height - header.height - parent.spacing)
      contentWidth: width
      contentHeight: form.implicitHeight
      clip: true
      boundsBehavior: Flickable.StopAtBounds
      flickableDirection: Flickable.VerticalFlick

      function reveal(field) {
        var point = field.mapToItem(form, 0, 0)
        if (point.y < contentY) contentY = point.y
        else if (point.y + field.height > contentY + height) contentY = Math.max(0, point.y + field.height - height)
      }

      Column {
        id: form
        width: parent.width
        spacing: Style.space(10)

        Column {
          width: parent.width
          spacing: Style.space(6)
          visible: !popup.editingSettings
          Label { text: "Nome" }
          TextField {
            id: nameField
            width: parent.width
            Accessible.name: "Nome"
            placeholderText: "Seu nome"
            maximumLength: 24
            enabled: !popup.busy
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
            Keys.onReturnPressed: codeField.forceActiveFocus()
          }
        }

        Column {
          width: parent.width
          spacing: Style.space(6)
          visible: !popup.editingSettings
          Label { text: "Código da sala" }
          TextField {
            id: codeField
            width: parent.width
            Accessible.name: "Código da sala"
            placeholderText: "XXX-XXX"
            font.family: "monospace"
            font.pixelSize: Style.font.body * 1.4
            horizontalAlignment: TextInput.AlignHCenter
            enabled: !popup.busy
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
            onTextEdited: {
              var m = popup.mask(text)
              if (m !== text) text = m
            }
            Keys.onReturnPressed: popup.submitJoin()
            Keys.onEnterPressed: popup.submitJoin()
          }
        }

        Column {
          width: parent.width
          spacing: Style.space(6)
          visible: popup.editingSettings
          Label { text: "URL da API" }
          TextField {
            id: apiField
            width: parent.width
            Accessible.name: "URL da API"
            placeholderText: "https://api.exemplo.com"
            enabled: !popup.saving
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
            Keys.onReturnPressed: webField.forceActiveFocus()
          }
          Label { text: "URL do site" }
          TextField {
            id: webField
            width: parent.width
            Accessible.name: "URL do site"
            placeholderText: "https://call.exemplo.com"
            enabled: !popup.saving
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
            Keys.onReturnPressed: keyField.forceActiveFocus()
          }
          Label { text: "Chave de acesso ao servidor" }
          TextField {
            id: keyField
            width: parent.width
            Accessible.name: "Chave de acesso ao servidor"
            password: true
            enabled: !popup.saving
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
            Keys.onReturnPressed: popup.saveSettings()
          }
          Button {
            text: keyField.password ? "Mostrar chave" : "Ocultar chave"
            focusable: true
            enabled: !popup.saving
            onClicked: keyField.password = !keyField.password
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
          }
          Label {
            width: parent.width
            text: "Fornecida por quem hospeda o Peep; não é o código da sala."
            wrapMode: Text.Wrap
            opacity: 0.72
            font.pixelSize: Style.font.bodySmall
          }
        }

        Label {
          width: parent.width
          visible: text !== ""
          text: popup.validationError || popup.error
          wrapMode: Text.Wrap
          color: Commons.Color.urgent
        }
        Label {
          width: parent.width
          visible: !popup.editingSettings && popup.notice !== ""
          text: popup.notice
          wrapMode: Text.Wrap
          color: Commons.Color.accent
        }

        Row {
          visible: !popup.editingSettings
          spacing: Style.space(8)
          Button {
            text: popup.busy ? "Aguarde…" : "Entrar"
            bordered: true
            focusable: true
            enabled: !popup.busy && codeField.text.length === 7
            onClicked: popup.submitJoin()
          }
          Button {
            text: "Criar sala"
            bordered: true
            focusable: true
            enabled: !popup.busy
            onClicked: popup.createRequested(nameField.text.trim())
          }
        }
        Row {
          visible: popup.editingSettings
          spacing: Style.space(8)
          Button {
            text: popup.saving ? "Salvando…" : "Salvar"
            bordered: true
            focusable: true
            enabled: !popup.saving
            onClicked: popup.saveSettings()
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
          }
          Button {
            text: "Cancelar"
            bordered: true
            focusable: true
            enabled: !popup.saving
            onClicked: popup.cancelSettings()
            onActiveFocusChanged: if (activeFocus) scroll.reveal(this)
          }
        }
      }
    }
  }
}
