import QtQuick
import qs.Common
import qs.Services
import qs.Widgets

// Hidden unless the item's library stores ratings, which also covers a library list that hasn't arrived yet.
DRatingBar {
    id: root

    property string kind: "track"
    property var item: null
    readonly property string blocker: PlayerService.whyNot("rate", item)
    // a track list column keeps its width while hidden, so the rows don't shift
    property bool keepsSpace: false
    readonly property bool shown: !!item && blocker === ""

    rating: item?.rating ?? 0
    visible: shown || keepsSpace
    opacity: shown ? 1 : 0
    enabled: shown
    onRated: stars => PlayerService.setRating(kind, item, stars)
}
