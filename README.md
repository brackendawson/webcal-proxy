![Go workflow](https://github.com/brackendawson/webcal-proxy/actions/workflows/go.yml/badge.svg)

# webcal-proxy
A simple server to proxy and filter webcal links.  

## Usage
### Server
Usage of webcal-proxy:
* -addr string
local address:port to bind to (default ":80")
* -logfile string
File to log to
* -loglevel string
log level (default "info")

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
webcal://example.com/webcal-proxy?cal=webcal://example.com/my/calendar&exc=SUMMARY=Boring%20Events
```
