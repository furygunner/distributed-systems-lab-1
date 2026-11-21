#!/usr/bin/env bash
# Walks through the pub/sub endpoints with pauses and plain-English narration.
# Usage: start the server (go run . 8080), then ./testing_events.bash
# Faster run: PAUSE=0 ./testing_events.bash
PORT="${PORT:-8080}"
B="http://localhost:${PORT}"
PAUSE="${PAUSE:-2}"   # seconds to pause between steps

# --- helpers ---------------------------------------------------------------
say()   { echo "   $*"; }
pause() { sleep "$PAUSE"; }
title() { echo; echo "=================================================="; echo "TEST $1"; echo "=================================================="; }
status() { head -1 | tr -d '\r'; }   # keep only the "HTTP/1.1 200 OK" line

# subscribe to topic $1, print the new subscriber id
sub() { curl -s -X POST "$B/subscribe?topic=$1" | sed -E 's/[^0-9]*([0-9]+).*/\1/'; }
# publish message $2 to topic $1
pub() { curl -s -o /dev/null -X POST "$B/publish?topic=$1" --data "$2"; }

# --- 1 ---------------------------------------------------------------------
title "1: a waiting client receives a message as soon as it is published"
say "A 'subscriber' is a client that wants messages about a topic. Here: topic 'news'."
id=$(sub news); say "Subscribed to 'news'. The server gave us subscriber id = $id"
pause
say "Now asking the server for events. With nothing published yet, this request just waits."
( curl -s -m 10 -i "$B/events?topic=news&id=$id" | sed 's/\r$//' | sed 's/^/      server replied: /' ) &
pause
say "Publishing the message 'hello' to 'news' ..."
pub news hello
wait
say "EXPECTED: status 200 and a body like '1 hello' (1 = message number, then the text)."
pause

# --- 2 ---------------------------------------------------------------------
title "2: several messages published before asking arrive together, in order"
id=$(sub news); say "New subscriber id = $id on 'news'"
pause
say "Publishing three messages: a, b, c"
pub news a; pause; pub news b; pause; pub news c; pause
say "Asking for events once. Server should return all three at once:"
curl -s -m 10 "$B/events?topic=news&id=$id" | sed 's/^/      server replied: /'
say "EXPECTED: message numbers go up (e.g. '2 a', '3 b', '4 c')."
say "NOTE: the server currently puts them on one line with no separator."
pause

# --- 3 ---------------------------------------------------------------------
title "3: a subscriber only gets messages for its own topic"
id=$(sub sports); say "Subscribed to 'sports' (id = $id)"
pause
say "Publishing 'ignored' to a DIFFERENT topic ('news'). 'sports' must not see it."
pub news ignored
pause
say "Asking 'sports' for events. Waiting up to 30s for the server to give up ..."
curl -s -i -m 35 "$B/events?topic=sports&id=$id" | status
say "EXPECTED: 204 No Content (nothing arrived for 'sports', so the server timed out)."
pause

# --- 4 ---------------------------------------------------------------------
title "4: messages published BEFORE subscribing are not delivered"
say "Publishing 'old' to topic 'late' while nobody is subscribed."
pub late old
pause
id=$(sub late); say "Now subscribed to 'late' (id = $id)"
say "Asking for events. Waiting up to 30s ..."
curl -s -i -m 35 "$B/events?topic=late&id=$id" | status
say "EXPECTED: 204 No Content (the old message was missed, as intended)."
pause

# --- 5 ---------------------------------------------------------------------
title "5: bad requests should be rejected with the right error"
say "a) subscriber id that doesn't exist (9999)"
curl -s -i "$B/events?topic=news&id=9999" | status
say "   EXPECTED: 404 Not Found"; pause
say "b) id is not a number ('abc')"
curl -s -i "$B/events?topic=news&id=abc" | status
say "   EXPECTED: 400 Bad Request"; pause
say "c) no topic given"
curl -s -i "$B/events?id=1" | status
say "   EXPECTED: 400 Bad Request"; pause
say "d) DELETE method (not supported)"
curl -s -i -X DELETE "$B/events?topic=news&id=1" | status
say "   EXPECTED: 501 Not Implemented"; pause

# --- 6 ---------------------------------------------------------------------
title "6: after unsubscribing, the id no longer works"
id=$(sub news); say "Subscribed to 'news' (id = $id)"
pause
say "Unsubscribing id $id ..."
curl -s -X POST "$B/unsubscribe?topic=news&id=$id" | sed 's/^/      server replied: /'
pause
say "Asking for events with the old id:"
curl -s -i "$B/events?topic=news&id=$id" | status
say "EXPECTED: 404 Not Found"
echo; echo "Done."
