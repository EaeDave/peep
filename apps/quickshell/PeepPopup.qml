import QtQuick
import qs.Commons
import qs.Commons as Commons
import qs.Ui

// LNX-2: name + code fields, Entrar / Criar sala, styled by the shell theme.
KeyboardPanel {
  id: popup

  property string name: ""
  property string error: ""
  property bool busy: false
  signal joinRequested(string code, string name)
  signal createRequested(string name)
  signal dismissRequested()

  readonly property string alphabet: "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

  function mask(raw) {
    var s = ""
    var up = String(raw).toUpperCase()
    for (var i = 0; i < up.length && s.length < 6; i++)
      if (alphabet.indexOf(up[i]) >= 0) s += up[i]
    return s.length > 3 ? s.slice(0, 3) + "-" + s.slice(3) : s
  }

  function submitJoin() {
    if (!busy && codeField.text.length === 7) joinRequested(codeField.text, nameField.text.trim())
  }

  onOpenChanged: if (open) { nameField.text = name; codeField.text = ""; codeField.forceActiveFocus() }

  focusTarget: name === "" ? nameField : codeField
  contentWidth: fittedContentWidth(Style.space(300))
  contentHeight: fittedContentHeight(column.implicitHeight)

  Column {
    id: column
    anchors.left: parent.left
    anchors.right: parent.right
    spacing: Style.space(10)

    Text {
      text: "Peep"
      color: Commons.Color.foreground
      font.family: Style.font.family
      font.pixelSize: Style.font.body * 1.3
      font.bold: true
    }

    TextField {
      id: nameField
      width: parent.width
      placeholderText: "Seu nome"
      maximumLength: 24
      font.family: Style.font.family
      Keys.onReturnPressed: codeField.forceActiveFocus()
      Keys.onEscapePressed: popup.dismissRequested()
    }

    TextField {
      id: codeField
      width: parent.width
      placeholderText: "XXX-XXX"
      font.family: "monospace"
      font.pixelSize: Style.font.body * 1.4
      horizontalAlignment: TextInput.AlignHCenter
      onTextEdited: {
        var m = popup.mask(text)
        if (m !== text) text = m
      }
      Keys.onReturnPressed: popup.submitJoin()
      Keys.onEnterPressed: popup.submitJoin()
      Keys.onEscapePressed: popup.dismissRequested()
    }

    Text {
      width: parent.width
      visible: popup.error !== ""
      text: popup.error
      wrapMode: Text.Wrap
      color: Commons.Color.urgent
      font.family: Style.font.family
      font.pixelSize: Style.font.body
    }

    Row {
      spacing: Style.space(8)
      Button {
        text: popup.busy ? "…" : "Entrar"
        bordered: true
        enabled: !popup.busy && codeField.text.length === 7
        onClicked: popup.submitJoin()
      }
      Button {
        text: "Criar sala"
        bordered: true
        enabled: !popup.busy
        onClicked: popup.createRequested(nameField.text.trim())
      }
    }
  }
}
