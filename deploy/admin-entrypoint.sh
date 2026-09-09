#!/bin/sh
# Replaces the build-time sentinels with this installation's URLs.
#
# nginx runs every executable in /docker-entrypoint.d before starting, so this
# happens once per container start, before the first request is served.
set -e

: "${UCODE_API_URL:=http://127.0.0.1:8000}"
: "${UCODE_AUTH_URL:=http://127.0.0.1:9104}"
: "${UCODE_CDN_URL:=}"
: "${UCODE_ICON_CDN_URL:=}"

root=/usr/share/nginx/html

find "$root" -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' \) -exec sed -i \
    -e "s|__UCODE_API_URL__|${UCODE_API_URL}|g" \
    -e "s|__UCODE_AUTH_URL__|${UCODE_AUTH_URL}|g" \
    -e "s|__UCODE_CDN_URL__|${UCODE_CDN_URL}|g" \
    -e "s|__UCODE_ICON_CDN_URL__|${UCODE_ICON_CDN_URL}|g" \
    {} +

echo "ucode: admin panel pointed at ${UCODE_API_URL}"
