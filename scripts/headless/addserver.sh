# Add library for a server: the address is checked before the sign-in fields show. A reachable (Open)Subsonic server
# leads to the sign-in step, an address nothing listens on shows the error with Continue anyway and Try again, and
# editing the address after a passed check goes back to the address step. Needs SERVER=1.
failed=0
fact() { # fact NAME ACTUAL WANTED
    if [ "$2" = "$3" ]; then
        echo "ok   $1: $2" >>"$W/out/facts.txt"
        return
    fi
    failed=1
    echo "FAIL $1: got '$2', wanted '$3'" | tee -a "$W/out/facts.txt"
}

port=$(cat "$W/server/port")
ui settings libraries
sleep 2
fact "Add library found" "$([ "$(ask press "Add library" 0)" -ge 1 ] && echo 1)" 1
sleep 1
# the kind group's buttons aren't reachable by label: click the second one
click 1222 644
sleep 1
shot addserver-step1
fact "step 1 shows the address" "$(ask shown "Server address")" 1
fact "step 1 has no user name" "$(ask shown "User name")" 0
fact "step 1 has Continue" "$(ask shown "Continue")" 1

wtype "http://127.0.0.1:$port"
sleep 0.5
wtype -k Return
sleep 2
shot addserver-step2
fact "step 2 shows the user name" "$(ask shown "User name")" 1
fact "step 2 shows Test connection" "$(ask shown "Test connection")" 1

# editing the address goes back to step 1
click 1280 662
sleep 0.7
wtype -k End
wtype 1
sleep 1.5
shot addserver-back
fact "an address edit goes back to step 1" "$(ask shown "User name")" 0

# an address nothing listens on
wtype -M ctrl -k a -m ctrl
wtype "http://127.0.0.1:1"
sleep 0.5
wtype -k Return
sleep 3
shot addserver-error
fact "error shows Continue anyway" "$(ask shown "Continue anyway")" 1
fact "error shows Try again" "$(ask shown "Try again")" 1
fact "error has no user name" "$(ask shown "User name")" 0
fact "Continue anyway found" "$([ "$(ask press "Continue anyway" 0)" -ge 1 ] && echo 1)" 1
sleep 1
shot addserver-anyway
fact "Continue anyway shows the user name" "$(ask shown "User name")" 1

[ "$failed" = 0 ]
