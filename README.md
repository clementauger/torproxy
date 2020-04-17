# torproxy

provides a tor connected chromium session.

it creates an http proxy connected to a tor socks5, and starts chromium over that http proxy.

the http proxy is able to read a list of line seperated domain name to reject.

If you run `!windows` OS, make sure that `chromium-browser` is on your path.

If you run `windows` OS, it will download and install a portable version of the `chromium-browser`.

```sh
$ go run . -h
Usage of /tmp/go-build969835250/b001/exe/torproxy:
  -disable string
    	path to a list of disabled dns
  -hport int
    	http proxy port (default 9046)
  -no-incognito
    	disable incognito mode
  -port int
    	socks5 port (default 9045)
  -split
    	split networks so that clear internet does not go through tor
  -verbose
    	increase verbosity
exit status 2
```
