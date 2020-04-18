package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	chromiumprebuilt "github.com/clementauger/torproxy/chromium-prebuilt"

	"github.com/clementauger/tor-prebuilt/embedded"
	"github.com/cretz/bine/tor"
)

func main() {

	var port int
	var verbose bool
	var noIncognito bool
	var profilePath string
	var dataDir string
	flag.BoolVar(&verbose, "verbose", false, "increase verbosity")
	flag.IntVar(&port, "port", 9045, "socks5 port")
	flag.BoolVar(&noIncognito, "no-incognito", false, "enable incognito mode")
	flag.StringVar(&profilePath, "profile", "", "path to the directory containing profile path")
	flag.StringVar(&dataDir, "data-dir", "", "path to the data directory")

	flag.Parse()

	socksAddr := fmt.Sprintf("127.0.0.1:%v", port)

	client, err := getTorSocks5(port, verbose)
	if err != nil {
		log.Fatalf("failed to initialize tor socks5 connection: %v", err)
	}
	defer client.Close()

	errCh := make(chan error)

	cpb := chromiumprebuilt.Provider{DataDir: dataDir}

	err = cpb.Install(socksAddr)
	if err != nil {
		log.Fatalf("failed to setup chromium-browser: %v", err)
	}
	if profilePath == "" {
		profilePath, err = cpb.ProfilePath()
		if err != nil {
			log.Fatalf("failed to determine profile path: %v", err)
		}
	}
	var args []string
	args, err = cpb.LookupChromeArgs()
	if err != nil {
		log.Fatalf("failed to read chrlauncher.ini: %v", err)
	}
	args = append(args, "--flag-switches-begin")
	proxyOpt := fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%v", port)
	args = append(args, proxyOpt)
	if !noIncognito {
		args = append(args, "--incognito")
	}
	args = append(args, "--incognito")
	args = append(args, fmt.Sprintf("--user-data-dir=%v", profilePath))
	args = append(args, "--disable-breakpad")
	args = append(args, "--disable-logging")
	args = append(args, "--allow-outdated-plugins")
	args = append(args, "--no-default-browser-check")
	args = append(args, "--flag-switches-end")
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
			return
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
