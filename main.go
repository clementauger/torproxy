package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	chromiumprebuilt "test/proxy/chromium-prebuilt"

	"github.com/clementauger/tor-prebuilt/embedded"
	"github.com/cretz/bine/tor"
	"github.com/go-httpproxy/httpproxy"
	"golang.org/x/net/proxy"
)

func main() {

	var port int
	var httpPort int
	var verbose bool
	var disableIncognito bool
	var splitNetworks bool
	var forbidden string
	flag.BoolVar(&verbose, "verbose", false, "increase verbosity")
	flag.IntVar(&port, "port", 9045, "socks5 port")
	flag.IntVar(&httpPort, "hport", 9046, "http proxy port")
	flag.BoolVar(&disableIncognito, "no-incognito", false, "disable incognito mode")
	flag.BoolVar(&splitNetworks, "split", false, "split networks so that clear internet does not go through tor")
	flag.StringVar(&forbidden, "disable", "", "path to a list of disabled dns")

	flag.Parse()

	var forbiddenList []string
	if forbidden != "" {
		b, err := ioutil.ReadFile(forbidden)
		if err != nil {
			log.Fatalf("failed to read disabled dns file %v: %v", forbidden, err)
		}
		forbiddenList = strings.Split(string(b), "\n")
	}

	socksAddr := fmt.Sprintf("127.0.0.1:%v", port)
	httpAddr := fmt.Sprintf("127.0.0.1:%v", httpPort)

	c, err := net.Dial("tcp", socksAddr)
	if err != nil {
		client, err := getTorSocks5(port, verbose)
		if err != nil {
			log.Fatalf("failed to initialize tor socks5 connection: %v", err)
		}
		defer client.Close()
	} else {
		c.Close()
	}

	errCh := make(chan error)
	if httpPort > 0 {
		c, err = net.Dial("tcp", httpAddr)
		if err != nil {
			go func() {
				err := startHTTPProxy(socksAddr, httpPort, splitNetworks, forbiddenList)
				if err != nil {
					errCh <- fmt.Errorf("failed to start http proxy: %v", err)
				}
			}()
		} else {
			c.Close()
		}
	}

	cpb := chromiumprebuilt.Provider{}

	err = cpb.Install(httpAddr)
	if err != nil {
		log.Fatalf("failed to setup chromium-browser: %v", err)
	}
	var args []string
	args, err = cpb.LookupChromeArgs()
	if err != nil {
		log.Fatalf("failed to read chrlauncher.ini: %v", err)
	}
	proxyOpt := fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%v", port)
	if httpPort > 0 {
		proxyOpt = fmt.Sprintf("--proxy-server=http://127.0.0.1:%v", httpPort)
	}
	args = append(args, proxyOpt)
	if !disableIncognito {
		args = append(args, "--incognito")
	}
	go func() {
		cmd, err := cpb.Cmd(args...)
		if err != nil {
			errCh <- fmt.Errorf("failed to start chromium-browser: %v", err)
			return
		}
		if verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		err = cmd.Run()
		if err != nil {
			errCh <- fmt.Errorf("failed to run chromium-browser: %v", err)
		}
	}()

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
	for {
		select {
		case err = <-errCh:
			if err != nil {
				log.Println(err)
			}
		case <-sigChan:
			os.Exit(0)
		}
	}
}

func getTorSocks5(localPort int, verbose bool) (*tor.Tor, error) {
	torConfig := fmt.Sprintf(embedded.TorRCDefaults+`
SocksPort 127.0.0.1:%v
SocksPolicy accept 127.0.0.0/8
SocksPolicy reject *
`, localPort)

	ioutil.WriteFile("torrc", []byte(torConfig), os.ModePerm)
	d, _ := ioutil.TempDir("", "data-dir")
	h, _ := filepath.Abs("torrc")
	dw := ioutil.Discard
	if verbose {
		dw = os.Stderr
	}
	conf := &tor.StartConf{
		NoHush:          true,
		DebugWriter:     dw,
		NoAutoSocksPort: true,
		ProcessCreator:  embedded.NewCreator(),
		TorrcFile:       h,
		DataDir:         d,
		EnableNetwork:   true,
	}
	t, err := tor.Start(nil, conf)
	if err != nil {
		return nil, err
	}
	return t, err
}

func OnError(ctx *httpproxy.Context, where string, err *httpproxy.Error, opErr error) {
	// Log errors.
	log.Printf("ERR: %s: %s [%s]", where, err, opErr)
}

func OnAuth(ctx *httpproxy.Context, authType string, user string, pass string) bool {
	return true
	// // Auth test user.
	// if user == "test" && pass == "1234" {
	// 	return true
	// }
	// return false
}

func OnConnect(ctx *httpproxy.Context, host string) (ConnectAction httpproxy.ConnectAction, newHost string) {
	log.Printf("OnConnect: %s", ctx.Req.URL)
	// Apply "Man in the Middle" to all ssl connections. Never change host.
	return httpproxy.ConnectProxy, host
}

func OnRequest(ctx *httpproxy.Context, req *http.Request) (resp *http.Response) {
	log.Printf("OnRequest: %s", ctx.Req.URL)
	// Log proxying requests.
	// log.Printf("INFO: Proxy: %s %s", req.Method, req.URL.String())
	return
}

func OnResponse(ctx *httpproxy.Context, req *http.Request, resp *http.Response) {
	log.Printf("OnResponse: %s", ctx.Req.URL)
	// Add header "Via: go-httpproxy".
	resp.Header.Add("Via", "go-httpproxy")
}

func startHTTPProxy(socks5Addr string, httpPort int, splitNetworks bool, forbidden []string) error {
	torSocks, err := proxy.SOCKS5("tcp", socks5Addr, nil, proxy.Direct)
	if err != nil {
		return err
	}
	var d proxy.Dialer
	d = &splitDialer{tor: torSocks}
	if !splitNetworks {
		d = torSocks
	}

	prx, _ := httpproxy.NewProxy()
	prx.Rt = &http.Transport{Dial: d.Dial}

	OnAccept := func(ctx *httpproxy.Context, w http.ResponseWriter, r *http.Request) bool {
		j := r.URL.Host
		for _, f := range forbidden {
			if len(f) > 0 && strings.Contains(j, f) {
				log.Printf("OnAccept: blocked %s", r.URL)
				return true
			}
		}
		log.Printf("OnAccept: %s", r.URL)
		return false
	}

	prx.OnError = OnError
	prx.OnAccept = OnAccept
	prx.OnAuth = OnAuth
	prx.OnConnect = OnConnect
	prx.OnRequest = OnRequest
	prx.OnResponse = OnResponse

	addr := fmt.Sprintf("127.0.0.1:%v", httpPort)
	return http.ListenAndServe(addr, prx)
}

type splitDialer struct {
	tor   proxy.Dialer
	clear proxy.Dialer
}

func (o *splitDialer) Dial(network, addr string) (c net.Conn, err error) {
	if strings.Contains(addr, ".onion") {
		return o.tor.Dial(network, addr)
	}
	if o.clear != nil {
		return o.clear.Dial(network, addr)
	}
	return net.Dial(network, addr)
}

type socksDialer struct {
	socks5Addr string
}

func (o *socksDialer) Dial(network, addr string) (c net.Conn, err error) {
	dialSocksProxy, err := proxy.SOCKS5("tcp", o.socks5Addr, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}
	return dialSocksProxy.Dial(network, addr)
}
