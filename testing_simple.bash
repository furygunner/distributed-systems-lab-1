#!/usr/bin/env bash
# =============================================================================
# testing_simple.bash - a simple list of curl commands.
#
# HOW TO USE
#   1. Start the server in another terminal:   go run . 8080
#   2. Run this file:                          ./testing_simple.bash
#   3. EDIT THE LIST BELOW: add, delete or comment out (#) any line you like.
#      Each "t" line is one request. Lines are run top to bottom.
#
# HOW A LINE WORKS
#   t "label" METHOD "/path?query" [extra curl options]
#   - label   : text printed above the reply so you know which test it is
#   - METHOD  : GET, POST, DELETE, ...
#   - path    : everything after host:port
#   - extra   : e.g.  --data "hello"   puts "hello" in the request body
#
# Another port:  PORT=9000 ./testing_simple.bash
# =============================================================================

PORT="${PORT:-8080}"
BASE="http://localhost:${PORT}"

# t = run one request and print the status line, headers and body
t() {
  label="$1"; method="$2"; path="$3"
  shift 3                                  # whatever is left are extra curl options
  echo "----- $label:  $method $path"
  curl -s -i -m 5 -X "$method" "$@" "${BASE}${path}"
  echo; echo
}

# =============================================================================
# EDIT BELOW THIS LINE: one request per line. Add, remove or reorder freely.
# =============================================================================

# --- files ---
#t "get existing file"       GET    "/test.html"
#sleep 3
#t "get missing file"        GET    "/nope.html"
#sleep 3
#t "get bad extension"       GET    "/data.json"
#sleep 3
#t "delete not implemented"  DELETE "/test.html"
#sleep 5

# --- subscribe (the reply shows the new id: {"id": N}) ---
t "subscribe news"          POST   "/subscribe?topic=news"
sleep 3
t "subscribe news again"    POST   "/subscribe?topic=news"
sleep 3
t "subscribe sports"        POST   "/subscribe?topic=sports"
sleep 3
t "subscribe no topic"      POST   "/subscribe"
sleep 5

# --- publish (message goes in the body with --data) ---
t "publish hello to news"   POST   "/publish?topic=news" --data "hello"
sleep 3
t "publish to sports"       POST   "/publish?topic=sports" --data "goal"
sleep 3
t "publish no topic"        POST   "/publish" --data "x"
sleep 5
# --- events (change id=1 to an id you actually got from subscribe) ---
t "events id 1"           GET    "/events?topic=news&id=1"
sleep 5
# --- unsubscribe (change id=1 to a real id) ---
t "unsubscribe id 1"      POST   "/unsubscribe?topic=news&id=1"
sleep 3
t "unsubscribe bad id"    POST   "/unsubscribe?topic=news&id=abc"
