#!/bin/bash

for f in *.MAP; do
    [ -f "$f" ] && map2gpx "$f" "${f%.MAP}.gpx"
done
