package chromiumprebuilt

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/artdarek/go-unzip"
	"github.com/sethgrid/pester"
)

type Provider struct {
	DataDir string
}

func (p Provider) AutoDetectURL() (url string, err error) {
	if strings.HasPrefix(runtime.GOOS, "windo") {
		url = "https://chromium.woolyss.com/f/chrlauncher-win64-stable-ungoogled.zip"
		if strings.HasPrefix(runtime.GOARCH, "arm") {
			err = fmt.Errorf("unsuported %v %v", runtime.GOOS, runtime.GOARCH)
		}
	}
	return
}

func (p Provider) Install(socksAddr string) (err error) {
	if strings.HasPrefix(runtime.GOOS, "windo") == false {
		return nil // user must install via package manager
	}
	url, err := p.AutoDetectURL()
	if err != nil {
		return err
	}

	h, err := p.ResolveDataDir()
	if err != nil {
		return err
	}

	h = filepath.Join(h, ".torproxy/")
	os.MkdirAll(h, os.ModePerm)
	dlDir := filepath.Join(h, "chromium")
	os.MkdirAll(dlDir, os.ModePerm)
	binDir := filepath.Join(h, "chromium/bin")
	os.MkdirAll(binDir, os.ModePerm)

	chrExe := filepath.Join(binDir, "chrome.exe")
	if _, err := os.Stat(chrExe); os.IsNotExist(err) == false {
		return nil
	}

	var chrPath string
	g, err := filepath.Glob(filepath.Join(dlDir, "chrlauncher*.exe"))
	if err != nil {
		return err
	}
	if len(g) > 0 {
		chrPath = g[0]
	}

	if chrPath == "" {
		dst := filepath.Join(h, filepath.Base(url))
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, os.ModePerm)
			if err != nil {
				return err
			}
			resp, err := pester.Get(url)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			_, err = io.Copy(f, resp.Body)
			if err != nil {
				return err
			}
			resp.Body.Close()
			f.Close()
		}

		uz := unzip.New(dst, dlDir)
		err = uz.Extract()
		if err != nil {
			return err
		}
	}

	if chrPath == "" {
		g, err := filepath.Glob(filepath.Join(dlDir, "chrlauncher*.exe"))
		if err != nil {
			return err
		}
		if len(g) > 0 {
			chrPath = g[0]
		}
	}
	if chrPath == "" {
		return fmt.Errorf("can not find chrlauncher file path")
	}

	chrIni := filepath.Join(dlDir, "chrlauncher.ini")
	if _, err := os.Stat(chrIni); os.IsNotExist(err) {
		return err
	}
	chrB, err := ioutil.ReadFile(chrIni)
	if err != nil {
		return err
	}
	chrB = bytes.Replace(chrB, []byte(`#Proxy=127.0.0.1:80`), []byte(fmt.Sprintf("Proxy=%v", socksAddr)), -1)
	chrB = bytes.Replace(chrB, []byte(`Proxy=127.0.0.1:80`), []byte(fmt.Sprintf("Proxy=%v", socksAddr)), -1)
	chrB = bytes.Replace(chrB, []byte(`ChromiumWaitForDownloadEnd=false`), []byte(`ChromiumWaitForDownloadEnd=true`), -1)
	chrB = bytes.Replace(chrB, []byte(`ChromiumUpdateOnly=false`), []byte(`ChromiumUpdateOnly=true`), -1)
	chrB = bytes.Replace(chrB, []byte(`ChromiumAutoDownload=false`), []byte(`ChromiumAutoDownload=true`), -1)
	chrB = bytes.Replace(chrB, []byte(`ChromiumBringToFront=true`), []byte(`ChromiumBringToFront=false`), -1)
	err = ioutil.WriteFile(chrIni+".updated", chrB, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (p Provider) LookupChromeArgs() ([]string, error) {
	if strings.HasPrefix(runtime.GOOS, "windo") == false {
		return []string{}, nil
	}
	h, err := p.ResolveDataDir()
	if err != nil {
		return nil, err
	}
	dlDir := filepath.Join(h, ".torproxy/chromium")
	chrIni := filepath.Join(dlDir, "chrlauncher.ini.updated")
	if _, err := os.Stat(chrIni); os.IsNotExist(err) {
		return nil, err
	}
	chrB, err := ioutil.ReadFile(chrIni)
	if err != nil {
		return nil, err
	}

	var extraArgs []string
	extraArgs = append(extraArgs, "/autodownload")
	extraArgs = append(extraArgs, "/wait")
	extraArgs = append(extraArgs, "/ini="+chrIni)
	for _, line := range strings.Split(string(chrB), "\n") {
		if strings.HasPrefix(line, "ChromiumCommandLine=") {
			u := strings.TrimPrefix(line, "ChromiumCommandLine=")
			extraArgs = append(extraArgs, strings.Split(u, " ")...)
			break
		}
	}
	return extraArgs, nil
}

func (p Provider) ResolveDataDir() (h string, err error) {
	h = p.DataDir
	if h == "" {
		h, err = os.UserHomeDir()
	}
	return
}

func (p Provider) BinPath() (string, error) {
	if strings.HasPrefix(runtime.GOOS, "windo") == false {
		return "chromium-browser", nil
	}
	h, err := p.ResolveDataDir()
	if err != nil {
		return "", err
	}

	url, err := p.AutoDetectURL()
	if err != nil {
		return "", err
	}
	h = filepath.Join(h, ".torproxy/")
	dst := filepath.Join(h, filepath.Base(url))
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return "", err
	}
	g, _ := filepath.Glob(filepath.Join(h, "chromium/chrlauncher*.exe"))
	if len(g) > 0 {
		return g[0], nil
	}
	h = filepath.Join(h, "chromium/bin/chrome.exe")
	if _, err := os.Stat(h); os.IsNotExist(err) {
		return "", err
	}
	return h, nil
}

func (p Provider) Cmd(args ...string) (*exec.Cmd, error) {
	b, err := p.BinPath()
	if err != nil {
		return nil, err
	}
	return exec.Command(b, args...), nil
}
