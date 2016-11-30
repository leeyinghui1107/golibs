package main

//go:generate go-bindata-assetfs asset/...

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	//"github.com/elazarl/go-bindata-assetfs"
	"github.com/xiqingping/golibs/thrustrpc"

	thrwin "github.com/miketheprogrammer/go-thrust/lib/bindings/window"
	thrcmd "github.com/miketheprogrammer/go-thrust/lib/commands"
	"github.com/miketheprogrammer/go-thrust/thrust"
)

func main() {
	//	http.Handle("/", http.FileServer(&assetfs.AssetFS{
	//		Asset:     Asset,
	//		AssetDir:  AssetDir,
	//		AssetInfo: AssetInfo,
	//		Prefix:    "asset/html"}))

	http.Handle("/", http.FileServer(http.Dir("asset/html")))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal("Listen:", err)
	}

	fmt.Println(ln.Addr().String())

	thrust.SetProvisioner(ThrustProvisioner{})
	thrust.InitLogger()
	thrust.Start()

	win := thrust.NewWindow(thrust.WindowOptions{
		HasFrame: true,
		RootUrl:  fmt.Sprintf("http://%s/", ln.Addr().String()),
	})
	win.Show()
	win.OpenDevtools()
	win.SetTitle("GVRTool")
	win.Focus()

	rpc, err := thrustrpc.NewRpc(win)
	if err != nil {
		panic(err)
	}

	rpc.Register("add", func(arg interface{}) (interface{}, error) {
		adds, ok := arg.([]int)
		if !ok {
			return nil, errors.New("arguments format error")
		}
		sum := 0
		for add := range adds {
			sum += add
		}
		return sum, nil
	})

	_, err = win.HandleEvent("closed", func(er thrcmd.EventResult, this *thrwin.Window) {
		thrust.Exit()
	})

	if err != nil {
		fmt.Println(err)
		thrust.Exit()
	}
	http.Serve(ln, nil)
}
