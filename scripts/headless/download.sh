# The Download button on a server album, a local album and a server playlist: not kept, kept, the Remove dialog and removed.
# Needs SERVER=1.
failed=0
fact() { # fact NAME ACTUAL WANTED
    if [ "$2" = "$3" ]; then
        echo "ok   $1: $2" >>"$W/out/facts.txt"
        return
    fi
    failed=1
    echo "FAIL $1: got '$2', wanted '$3'" | tee -a "$W/out/facts.txt"
}
# rules: the number of album rules
rules() { rpc cache.status '{}' 100000 | python3 -I -c 'import json,sys; print(len([r for r in json.load(sys.stdin)["result"]["rules"] if r["kind"] == "album"]))'; }
# point LABEL: the screen point of the item with that tooltip, "" when none
point() {
    read -r px py < <(ask pagePoint "$1")
    [ -n "$px" ] || return 0
    read -r wx wy < <(hyprctl -j clients | python3 -I -c 'import json,sys
c = next(c for c in json.load(sys.stdin) if c["title"] == "Dux Player")
print(*c["at"])')
    echo "$((wx + px)) $((wy + py))"
}

lib=$(rpc libraries.list '{}' 100000 | python3 -I -c 'import json,sys; print([l["id"] for l in json.load(sys.stdin)["result"] if l["kind"] == "subsonic"][0])' 2>/dev/null)
if [ -z "$lib" ]; then
    echo "FAIL no server library (run with SERVER=1)" | tee -a "$W/out/facts.txt"
    exit 1
fi

ui open album "$lib:al-1-1"
sleep 2
fact "button shown on a server album" "$([ -n "$(point Download)" ] && echo 1)" 1
shot download-1-not-kept
click $(point Download)
sleep 2
fact "rule added" "$(rules)" 1
fact "button reads Downloaded" "$([ -n "$(point Downloaded)" ] && echo 1)" 1
shot download-2-kept
click $(point Downloaded)
sleep 1.5
shot download-3-dialog
fact "dialog keeps the rule" "$(rules)" 1
# Remove sits at the lower right of the dialog, which is centered on the window
click 1494 768
sleep 2
fact "rule removed" "$(rules)" 0
shot download-4-removed

ui open album "$(ask albumId "Test Album" | tr -d '\n')"
sleep 2
fact "no button on a local album" "$([ -z "$(point Download)" ] && [ -z "$(point Downloaded)" ] && echo 1)" 1
shot download-5-local

ui open playlist "$lib:pl-1"
sleep 2
fact "button shown on a server playlist" "$([ -n "$(point Download)" ] && echo 1)" 1
shot download-6-playlist
exit $failed
