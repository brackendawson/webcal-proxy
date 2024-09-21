[![Go workflow](https://github.com/brackendawson/webcal-proxy/actions/workflows/go.yml/badge.svg)](https://github.com/brackendawson/webcal-proxy/actions/workflows/go.yml)

# ![wp logo](assets/img/favicon.ico) webcal-proxy
A simple server to filter events from webcal feeds.

[![Try it out](doc/try-it-out.png)](https://bracken.cc/webcal-proxy)

## Usage
### Server
#### Arguments
Usage of webcal-proxy:
* -addr string
local address:port to bind to (default ":8080")
* -logfile string
File to log to
* -log-level string
log level (default "info")
* -max-conns maximum total upstream connections
* -dev disables security policies that prevent http://localhost from working

#### TLS
The server should be run behind a reverse proxy which terminates TLS because the webcal:// protocol requires valid TLS. The web interface will also not function on http without the -dev argument, even then some things will not work, such as clipboard interaction.

#### Proxy Path
If the reverse proxy uses a path then provide it in the `X-Forwarded-URI` header. Example nginx config:
```nginx
location /webcal-proxy/ {
    proxy_pass          http://127.0.0.1:8080;
    rewrite             ^/webcal-proxy/(.*)$ /$1 break;
    proxy_set_header    Host    $host;
    proxy_set_header    X-Forwarded-URI /webcal-proxy;
    proxy_redirect      off;
    proxy_buffering     off;
}
```

### Client
Enter the URL into your webcal client:
```
webcal://<this_server>/?cal=<webcal_url>[&inc=<query> ...][&exc=<query> ...][&mrg=true]
```
Where:
* **this_server** is the address and path hosting this program.
* **cal** your upstream webcal link, including the protocol scheme (webcal, http, https) (Required).
* **inc** query for events to include in the form `<FIELD>=<regexp>` where **FIELD** is an iCal event field (eg `SUMMARY`) and **regexp** is an unbound regular expression. Multiple inc arguments are allowed, (default `SUMMARY=.*`).
* **exc** query for events to exclude in the form `<FIELD>=<regexp>` where **FIELD** is an iCal event field (eg `SUMMARY`) and **regexp** is an unbound regular expression. Multiple inc arguments are allowed.
* **mrg** optional parameter to merge overlapping events into the one event.

eg:
```
webcal://webcal-proxy.example.com/webcal-proxy?cal=webcal://example.com/my/calendar&exc=SUMMARY=Boring%20Events
```
