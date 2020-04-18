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

func (p Provider) SystemWidePath() (binpath string, err error) {
	if strings.HasPrefix(runtime.GOOS, "windo") == false {
		paths := []string{
			"chromium-browser",
			"google-chrome",
		}
		for _, pa := range paths {
			path, err := exec.LookPath(pa)
			if err != nil {
				continue
			}
			return path, nil
		}
		return "", fmt.Errorf("binary not found in PATH (%v)", paths)
	}
	path, err := exec.LookPath("chrome.exe")
	if err == nil {
		return path, nil
	}
	path = `C:\Program Files (x86)\Google\Chrome\Application\Chrome.exe`
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return path, nil
	}
	return "", fmt.Errorf("chrome.exe not found in PATH")
}

func (p Provider) Install(socksAddr string) (err error) {
	if strings.HasPrefix(runtime.GOOS, "windo") == false {
		_, err := p.SystemWidePath()
		return err // user must have it within its PATH
	}
	_, err = p.SystemWidePath()
	if err == nil {
		return nil // is in PATH
	}

	// proceed install

	url, err := p.AutoDetectURL()
	if err != nil {
		return err
	}

	h, err := p.ResolveDataDir()
	if err != nil {
		return err
	}

	h = filepath.Join(h, ".torproxy")
	os.MkdirAll(h, os.ModePerm)
	dlDir := filepath.Join(h, "chromium")
	os.MkdirAll(dlDir, os.ModePerm)
	binDir := filepath.Join(h, "chromium", "bin")
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
	dlDir := filepath.Join(h, ".torproxy", "chromium")
	chrIni := filepath.Join(dlDir, "chrlauncher.ini.updated")

	var extraArgs []string
	extraArgs = append(extraArgs, "/autodownload")
	extraArgs = append(extraArgs, "/wait")
	extraArgs = append(extraArgs, "/ini="+chrIni)
	return extraArgs, nil
}

func (p Provider) ResolveDataDir() (h string, err error) {
	h = p.DataDir
	if h == "" {
		h, err = os.UserHomeDir()
	}
	return
}

func (p Provider) ProfilePath() (string, error) {
	h, err := p.ResolveDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".torproxy", "profile"), nil
}

func (p Provider) BinPath() (string, error) {
	if strings.HasPrefix(runtime.GOOS, "windo") == false {
		return p.SystemWidePath()
	}
	binpath, err := p.SystemWidePath()
	if err == nil {
		return binpath, nil // is in PATH
	}

	h, err := p.ResolveDataDir()
	if err != nil {
		return "", err
	}

	url, err := p.AutoDetectURL()
	if err != nil {
		return "", err
	}
	h = filepath.Join(h, ".torproxy")
	dst := filepath.Join(h, filepath.Base(url))
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return "", err
	}
	g, _ := filepath.Glob(filepath.Join(h, "chromium", "chrlauncher*.exe"))
	if len(g) > 0 {
		return g[0], nil
	}
	h = filepath.Join(h, "chromium", "bin", "chrome.exe")
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
