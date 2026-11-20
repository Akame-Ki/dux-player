import QtQuick
import qs.DCommon.Common

// The short pill at the top edge of a sheet or view that is grabbed to move it, with the open hand on hover and
// the closed hand while dragging. Emits dragStarted once per drag; the owner decides what moves.
Item {
    id: root

    readonly property bool dragging: drag.active

    signal dragStarted

    implicitWidth: Style.minimumTouchTargetSize
    implicitHeight: Style.minimumTouchTargetSize

    Accessible.role: Accessible.Button
    Accessible.name: ""

    Rectangle {
        anchors.centerIn: parent
        width: Style.bottomSheetHandleWidth
        height: Style.bottomSheetHandleHeight
        radius: Style.fullRadius(width, height)
        color: Style.onSurfaceVariant_40
    }

    HoverHandler {
        cursorShape: root.dragging ? Qt.ClosedHandCursor : Qt.OpenHandCursor
    }

    DragHandler {
        id: drag
        target: null
        onActiveChanged: {
            if (active)
                root.dragStarted();
        }
    }
}
